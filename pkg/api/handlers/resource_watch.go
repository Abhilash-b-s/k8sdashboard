package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"k8s-dashboard/pkg/k8s"
)

// resourceEvent is the wire format pushed over SSE.
// For "delete" events Row is nil and Key carries "namespace/name".
type resourceEvent struct {
	Type string `json:"type"`
	Row  any    `json:"row,omitempty"`
	Key  string `json:"key,omitempty"`
}

// resourceBroker fans out informer events to N concurrent SSE subscribers.
// Per-subscriber buffer is bounded; slow clients drop events rather than
// stall the broker (they resync via the snapshot on reconnect, and via
// the frontend's periodic /resource fetch reconciliation).
type resourceBroker struct {
	mu   sync.Mutex
	subs map[chan resourceEvent]struct{}
}

func newResourceBroker() *resourceBroker {
	return &resourceBroker{subs: make(map[chan resourceEvent]struct{})}
}

func (b *resourceBroker) subscribe() chan resourceEvent {
	ch := make(chan resourceEvent, 256)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *resourceBroker) unsubscribe(ch chan resourceEvent) {
	b.mu.Lock()
	if _, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(ch)
	}
	b.mu.Unlock()
}

func (b *resourceBroker) publish(ev resourceEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
			// Buffer full — slow client; skip.
		}
	}
}

// kindAdapter abstracts everything that varies per resource kind so the
// SSE handler stays generic.
type kindAdapter struct {
	// listSnapshot returns the full current set as row objects (anything
	// JSON-serializable). Used for the first SSE message on connect.
	listSnapshot func(client *k8s.ClusterClient) ([]any, error)
	// attachHandler registers an informer event handler that converts the
	// raw object to a row and publishes events on the broker.
	attachHandler func(client *k8s.ClusterClient, b *resourceBroker)
}

// keyOf is the canonical "namespace/name" key used in delete events.
func keyOf(ns, name string) string { return ns + "/" + name }

var kindAdapters = map[string]kindAdapter{
	"pods": {
		listSnapshot: func(client *k8s.ClusterClient) ([]any, error) {
			pods, err := client.InformerFactory.Core().V1().Pods().Lister().List(labels.Everything())
			if err != nil {
				return nil, err
			}
			out := make([]any, len(pods))
			for i, p := range pods {
				out[i] = toPodInfo(p)
			}
			return out, nil
		},
		attachHandler: func(client *k8s.ClusterClient, b *resourceBroker) {
			client.InformerFactory.Core().V1().Pods().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj any) {
					if p, ok := obj.(*corev1.Pod); ok {
						b.publish(resourceEvent{Type: "add", Row: toPodInfo(p)})
					}
				},
				UpdateFunc: func(_, obj any) {
					if p, ok := obj.(*corev1.Pod); ok {
						b.publish(resourceEvent{Type: "update", Row: toPodInfo(p)})
					}
				},
				DeleteFunc: func(obj any) {
					p := tombstonePod(obj)
					if p != nil {
						b.publish(resourceEvent{Type: "delete", Key: keyOf(p.Namespace, p.Name)})
					}
				},
			})
		},
	},

	"deployments": {
		listSnapshot: func(client *k8s.ClusterClient) ([]any, error) {
			items, err := client.InformerFactory.Apps().V1().Deployments().Lister().List(labels.Everything())
			if err != nil {
				return nil, err
			}
			out := make([]any, len(items))
			for i, d := range items {
				out[i] = toDeploymentRow(d)
			}
			return out, nil
		},
		attachHandler: func(client *k8s.ClusterClient, b *resourceBroker) {
			client.InformerFactory.Apps().V1().Deployments().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj any) {
					if d, ok := obj.(*appsv1.Deployment); ok {
						b.publish(resourceEvent{Type: "add", Row: toDeploymentRow(d)})
					}
				},
				UpdateFunc: func(_, obj any) {
					if d, ok := obj.(*appsv1.Deployment); ok {
						b.publish(resourceEvent{Type: "update", Row: toDeploymentRow(d)})
					}
				},
				DeleteFunc: func(obj any) {
					d := tombstoneDeployment(obj)
					if d != nil {
						b.publish(resourceEvent{Type: "delete", Key: keyOf(d.Namespace, d.Name)})
					}
				},
			})
		},
	},

	"statefulsets": {
		listSnapshot: func(client *k8s.ClusterClient) ([]any, error) {
			items, err := client.InformerFactory.Apps().V1().StatefulSets().Lister().List(labels.Everything())
			if err != nil {
				return nil, err
			}
			out := make([]any, len(items))
			for i, s := range items {
				out[i] = toStatefulSetRow(s)
			}
			return out, nil
		},
		attachHandler: func(client *k8s.ClusterClient, b *resourceBroker) {
			client.InformerFactory.Apps().V1().StatefulSets().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj any) {
					if s, ok := obj.(*appsv1.StatefulSet); ok {
						b.publish(resourceEvent{Type: "add", Row: toStatefulSetRow(s)})
					}
				},
				UpdateFunc: func(_, obj any) {
					if s, ok := obj.(*appsv1.StatefulSet); ok {
						b.publish(resourceEvent{Type: "update", Row: toStatefulSetRow(s)})
					}
				},
				DeleteFunc: func(obj any) {
					s := tombstoneStatefulSet(obj)
					if s != nil {
						b.publish(resourceEvent{Type: "delete", Key: keyOf(s.Namespace, s.Name)})
					}
				},
			})
		},
	},

	"jobs": {
		listSnapshot: func(client *k8s.ClusterClient) ([]any, error) {
			items, err := client.InformerFactory.Batch().V1().Jobs().Lister().List(labels.Everything())
			if err != nil {
				return nil, err
			}
			out := make([]any, len(items))
			for i, j := range items {
				out[i] = toJobRow(j)
			}
			return out, nil
		},
		attachHandler: func(client *k8s.ClusterClient, b *resourceBroker) {
			client.InformerFactory.Batch().V1().Jobs().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj any) {
					if j, ok := obj.(*batchv1.Job); ok {
						b.publish(resourceEvent{Type: "add", Row: toJobRow(j)})
					}
				},
				UpdateFunc: func(_, obj any) {
					if j, ok := obj.(*batchv1.Job); ok {
						b.publish(resourceEvent{Type: "update", Row: toJobRow(j)})
					}
				},
				DeleteFunc: func(obj any) {
					j := tombstoneJob(obj)
					if j != nil {
						b.publish(resourceEvent{Type: "delete", Key: keyOf(j.Namespace, j.Name)})
					}
				},
			})
		},
	},
}

// brokers cached by clusterName + ":" + kind. Initialized lazily on first
// subscribe; the informer handler is attached exactly once per (cluster, kind).
var (
	resourceBrokers   = map[string]*resourceBroker{}
	resourceBrokersMu sync.Mutex
)

func brokerForResource(client *k8s.ClusterClient, kind string, ad kindAdapter) *resourceBroker {
	key := client.Name + ":" + kind
	resourceBrokersMu.Lock()
	defer resourceBrokersMu.Unlock()
	if b, ok := resourceBrokers[key]; ok {
		return b
	}
	b := newResourceBroker()
	resourceBrokers[key] = b
	ad.attachHandler(client, b)
	return b
}

// WatchClusterResource streams resource events as SSE, parameterized by :kind.
// First message is "snapshot" carrying the full current list; subsequent
// events are "add" / "update" / "delete".
func WatchClusterResource(c *gin.Context) {
	kind := c.Param("kind")
	adapter, ok := kindAdapters[kind]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported kind", "kind": kind, "supported": supportedKinds()})
		return
	}

	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	broker := brokerForResource(client, kind, adapter)
	sub := broker.subscribe()
	defer broker.unsubscribe(sub)

	if rows, err := adapter.listSnapshot(client); err == nil {
		data, _ := json.Marshal(rows)
		c.SSEvent("snapshot", string(data))
		c.Writer.Flush()
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.SSEvent("ping", "")
			c.Writer.Flush()
		case ev, ok := <-sub:
			if !ok {
				return
			}
			data, _ := json.Marshal(ev)
			c.SSEvent(ev.Type, string(data))
			c.Writer.Flush()
		}
	}
}

func supportedKinds() []string {
	out := make([]string, 0, len(kindAdapters))
	for k := range kindAdapters {
		out = append(out, k)
	}
	return out
}

// -------- per-kind row builders --------

func toDeploymentRow(d *appsv1.Deployment) DeploymentInfo {
	desired := int32(0)
	if d.Spec.Replicas != nil {
		desired = *d.Spec.Replicas
	}
	return DeploymentInfo{
		Name:      d.Name,
		Namespace: d.Namespace,
		Ready:     fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, desired),
		UpToDate:  d.Status.UpdatedReplicas,
		Available: d.Status.AvailableReplicas,
		Age:       FormatAge(d.CreationTimestamp.Time),
	}
}

func toStatefulSetRow(s *appsv1.StatefulSet) StatefulSetInfo {
	desired := int32(0)
	if s.Spec.Replicas != nil {
		desired = *s.Spec.Replicas
	}
	return StatefulSetInfo{
		Name:      s.Name,
		Namespace: s.Namespace,
		Ready:     fmt.Sprintf("%d/%d", s.Status.ReadyReplicas, desired),
		Age:       FormatAge(s.CreationTimestamp.Time),
	}
}

// jobRow keeps the same shape as GetClusterJobs returns.
type jobRow struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Completions string `json:"completions"`
	Succeeded   int32  `json:"succeeded"`
	Failed      int32  `json:"failed"`
	Age         string `json:"age"`
}

func toJobRow(j *batchv1.Job) jobRow {
	completions := int32(1)
	if j.Spec.Completions != nil {
		completions = *j.Spec.Completions
	}
	return jobRow{
		Name:        j.Name,
		Namespace:   j.Namespace,
		Completions: fmt.Sprintf("%d/%d", j.Status.Succeeded, completions),
		Succeeded:   j.Status.Succeeded,
		Failed:      j.Status.Failed,
		Age:         FormatAge(j.CreationTimestamp.Time),
	}
}

// -------- tombstone helpers (handle DeletedFinalStateUnknown) --------

func tombstonePod(obj any) *corev1.Pod {
	if p, ok := obj.(*corev1.Pod); ok {
		return p
	}
	if t, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		if p, ok := t.Obj.(*corev1.Pod); ok {
			return p
		}
	}
	return nil
}

func tombstoneDeployment(obj any) *appsv1.Deployment {
	if d, ok := obj.(*appsv1.Deployment); ok {
		return d
	}
	if t, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		if d, ok := t.Obj.(*appsv1.Deployment); ok {
			return d
		}
	}
	return nil
}

func tombstoneStatefulSet(obj any) *appsv1.StatefulSet {
	if s, ok := obj.(*appsv1.StatefulSet); ok {
		return s
	}
	if t, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		if s, ok := t.Obj.(*appsv1.StatefulSet); ok {
			return s
		}
	}
	return nil
}

func tombstoneJob(obj any) *batchv1.Job {
	if j, ok := obj.(*batchv1.Job); ok {
		return j
	}
	if t, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		if j, ok := t.Obj.(*batchv1.Job); ok {
			return j
		}
	}
	return nil
}
