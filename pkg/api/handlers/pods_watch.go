package handlers

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"k8s-dashboard/pkg/k8s"
)

// podEvent is the wire format pushed over SSE.
// For "delete" events Pod is nil and Key carries "namespace/name".
type podEvent struct {
	Type string   `json:"type"`
	Pod  *PodInfo `json:"pod,omitempty"`
	Key  string   `json:"key,omitempty"`
}

// podBroker fans out informer events to N concurrent SSE subscribers.
// Per-subscriber buffer is bounded; slow clients drop events rather than
// stall the broker (they'll resync on the next snapshot/reconnect).
type podBroker struct {
	mu   sync.Mutex
	subs map[chan podEvent]struct{}
}

func newPodBroker() *podBroker {
	return &podBroker{subs: make(map[chan podEvent]struct{})}
}

func (b *podBroker) subscribe() chan podEvent {
	ch := make(chan podEvent, 256)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *podBroker) unsubscribe(ch chan podEvent) {
	b.mu.Lock()
	if _, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(ch)
	}
	b.mu.Unlock()
}

func (b *podBroker) publish(ev podEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
			// Buffer full — skip; subscriber will resync on reconnect.
		}
	}
}

// brokers, keyed by cluster name. Initialized lazily on first subscribe.
var (
	podBrokers   = map[string]*podBroker{}
	podBrokersMu sync.Mutex
)

func brokerForCluster(client *k8s.ClusterClient) *podBroker {
	podBrokersMu.Lock()
	defer podBrokersMu.Unlock()
	if b, ok := podBrokers[client.Name]; ok {
		return b
	}
	b := newPodBroker()
	podBrokers[client.Name] = b

	// Attach exactly one ResourceEventHandler per cluster informer.
	client.InformerFactory.Core().V1().Pods().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if pod, ok := obj.(*corev1.Pod); ok {
				info := toPodInfo(pod)
				b.publish(podEvent{Type: "add", Pod: &info})
			}
		},
		UpdateFunc: func(_, obj interface{}) {
			if pod, ok := obj.(*corev1.Pod); ok {
				info := toPodInfo(pod)
				b.publish(podEvent{Type: "update", Pod: &info})
			}
		},
		DeleteFunc: func(obj interface{}) {
			var pod *corev1.Pod
			switch t := obj.(type) {
			case *corev1.Pod:
				pod = t
			case cache.DeletedFinalStateUnknown:
				pod, _ = t.Obj.(*corev1.Pod)
			}
			if pod != nil {
				b.publish(podEvent{Type: "delete", Key: pod.Namespace + "/" + pod.Name})
			}
		},
	})
	return b
}

// WatchClusterPods streams pod events as SSE. The first event is
// "snapshot" carrying the full current pod list; subsequent events are
// "add" / "update" / "delete".
func WatchClusterPods(c *gin.Context) {
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

	broker := brokerForCluster(client)
	sub := broker.subscribe()
	defer broker.unsubscribe(sub)

	// Initial snapshot from the informer cache.
	pods, err := client.InformerFactory.Core().V1().Pods().Lister().List(labels.Everything())
	if err == nil {
		rows := make([]PodInfo, 0, len(pods))
		for _, p := range pods {
			rows = append(rows, toPodInfo(p))
		}
		data, _ := json.Marshal(rows)
		c.SSEvent("snapshot", string(data))
		c.Writer.Flush()
	}

	// Heartbeat every 15s so reverse proxies don't time out idle streams.
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
