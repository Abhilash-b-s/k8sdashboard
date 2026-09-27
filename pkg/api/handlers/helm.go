package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"
)

// helmTimeout mirrors the helm CLI's default --timeout for hooks and waits.
const helmTimeout = 5 * time.Minute

// restGetter adapts a cluster's rest.Config to the RESTClientGetter helm
// expects, since clusters here may come from in-cluster config or uploaded
// kubeconfigs rather than a kubeconfig file on disk.
type restGetter struct {
	cfg       *rest.Config
	namespace string
}

func (g restGetter) ToRESTConfig() (*rest.Config, error) { return rest.CopyConfig(g.cfg), nil }

func (g restGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	dc, err := discovery.NewDiscoveryClientForConfig(g.cfg)
	if err != nil {
		return nil, err
	}
	return memory.NewMemCacheClient(dc), nil
}

func (g restGetter) ToRESTMapper() (meta.RESTMapper, error) {
	dc, err := g.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	return restmapper.NewDeferredDiscoveryRESTMapper(dc), nil
}

func (g restGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return clientcmd.NewDefaultClientConfig(clientcmdapi.Config{},
		&clientcmd.ConfigOverrides{Context: clientcmdapi.Context{Namespace: g.namespace}})
}

// helmConfig builds a helm action configuration for the request's cluster,
// scoped to namespace ("" = all namespaces), using the default Secret storage.
func helmConfig(c *gin.Context, namespace string) (*action.Configuration, bool) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return nil, false
	}
	cfg := new(action.Configuration)
	getter := restGetter{cfg: client.RestConfig, namespace: namespace}
	if err := cfg.Init(getter, namespace, "secret", func(format string, v ...interface{}) {
		log.Printf("[helm] "+format, v...)
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "helm init failed", "details": err.Error()})
		return nil, false
	}
	return cfg, true
}

func helmError(c *gin.Context, msg string, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, driver.ErrReleaseNotFound) {
		status = http.StatusNotFound
	}
	c.JSON(status, gin.H{"error": msg, "details": err.Error()})
}

func releaseSummary(r *release.Release) gin.H {
	h := gin.H{"name": r.Name, "namespace": r.Namespace, "revision": r.Version}
	if r.Info != nil {
		h["status"] = r.Info.Status.String()
		h["updated"] = r.Info.LastDeployed.Time
		h["description"] = r.Info.Description
	}
	if r.Chart != nil && r.Chart.Metadata != nil {
		h["chart"] = r.Chart.Metadata.Name + "-" + r.Chart.Metadata.Version
		h["appVersion"] = r.Chart.Metadata.AppVersion
	}
	return h
}

// GetClusterHelmReleases lists the latest revision of every release
// (like `helm list -a`, or `-A` when no namespace is given).
func GetClusterHelmReleases(c *gin.Context) {
	ns := c.Query("namespace")
	cfg, ok := helmConfig(c, ns)
	if !ok {
		return
	}
	list := action.NewList(cfg)
	list.AllNamespaces = ns == ""
	list.StateMask = action.ListAll
	releases, err := list.Run()
	if err != nil {
		helmError(c, "list releases failed", err)
		return
	}
	items := make([]gin.H, 0, len(releases))
	for _, r := range releases {
		items = append(items, releaseSummary(r))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// GetClusterHelmReleaseDetail returns the latest revision with its values,
// manifest, notes and revision history.
func GetClusterHelmReleaseDetail(c *gin.Context) {
	name := c.Param("name")
	cfg, ok := helmConfig(c, c.Param("namespace"))
	if !ok {
		return
	}
	rel, err := action.NewGet(cfg).Run(name)
	if err != nil {
		helmError(c, "get release failed", err)
		return
	}
	hist := action.NewHistory(cfg)
	hist.Max = 256
	revisions, err := hist.Run(name)
	if err != nil {
		helmError(c, "get history failed", err)
		return
	}

	userValues, _ := yaml.Marshal(rel.Config)
	var allValues []byte
	if rel.Chart != nil {
		if computed, err := chartutil.CoalesceValues(rel.Chart, rel.Config); err == nil {
			allValues, _ = yaml.Marshal(computed)
		}
	}

	history := make([]gin.H, 0, len(revisions))
	for i := len(revisions) - 1; i >= 0; i-- { // newest first
		history = append(history, releaseSummary(revisions[i]))
	}

	out := releaseSummary(rel)
	out["userValues"] = string(userValues)
	out["allValues"] = string(allValues)
	out["manifest"] = rel.Manifest
	out["history"] = history
	if rel.Info != nil {
		out["notes"] = rel.Info.Notes
		out["firstDeployed"] = rel.Info.FirstDeployed.Time
	}
	c.JSON(http.StatusOK, out)
}

// RollbackClusterHelmRelease rolls a release back to {"revision": N}
// (0 = previous revision), like `helm rollback <name> [N]`.
func RollbackClusterHelmRelease(c *gin.Context) {
	var body struct {
		Revision int `json:"revision"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Revision < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body must be {\"revision\": N} with N >= 0"})
		return
	}
	cfg, ok := helmConfig(c, c.Param("namespace"))
	if !ok {
		return
	}
	rb := action.NewRollback(cfg)
	rb.Version = body.Revision
	rb.Timeout = helmTimeout
	if err := rb.Run(c.Param("name")); err != nil {
		helmError(c, "rollback failed", err)
		return
	}
	target := "previous revision"
	if body.Revision > 0 {
		target = "revision " + strconv.Itoa(body.Revision)
	}
	c.JSON(http.StatusOK, gin.H{"message": "rolled back " + c.Param("name") + " to " + target})
}

// UninstallClusterHelmRelease removes a release and its resources, like `helm uninstall`.
func UninstallClusterHelmRelease(c *gin.Context) {
	cfg, ok := helmConfig(c, c.Param("namespace"))
	if !ok {
		return
	}
	un := action.NewUninstall(cfg)
	un.Timeout = helmTimeout
	res, err := un.Run(c.Param("name"))
	if err != nil {
		helmError(c, "uninstall failed", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "uninstalled " + c.Param("name"), "info": res.Info})
}
