# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Multi-cluster Kubernetes dashboard. Single Go binary (Gin) serving a REST + SSE API and a static web UI built by Vite. Runs locally against `~/.kube/config`, in-cluster via service account, or both at once with kubeconfigs uploaded at runtime. See `README.md` for the user-facing feature list and route summary.

## Build & run

```bash
# Backend (port 8080) — uses $KUBECONFIG or ~/.kube/config
go run ./cmd/server

# Frontend dev server (port 5173, proxies /api → :8080)
npm install
npm run dev

# Production build: Vite emits to ./static/, Go server serves it from disk
npm run build

# Docker (multi-stage: builds frontend, then Go binary, into distroless)
docker build -t k8s-dashboard:dev .

# Full deploy cycle (build → push → kubectl apply → rollout)
./deploy.sh all
```

There is no test suite, no linter, and no Go formatter wired into CI. `go build ./...` and `go vet ./...` are the only correctness checks available.

The Vite dev server proxies `/api`, `/healthz`, `/readyz` to `localhost:8080`, so run both `go run ./cmd/server` and `npm run dev` together when iterating on the UI.

## Architecture

### Entry points and dead code

- **Real entry point**: `cmd/server/main.go` — wires up the `MultiClusterManager` and starts the Gin router from `pkg/api/router.go`.
- **`main.go` at the repo root is legacy dead code** (1604-line single-file implementation that predates the `cmd/`+`pkg/` split). It is still tracked in git but is not built by `go run ./cmd/server` or by `Dockerfile`. Don't edit it; if a behavior needs changing, do it in `pkg/`.
- **`pkg/k8s/client.go` and `pkg/k8s/informer.go`** define a global single-cluster `Clientset`/`InformerFactory`. They are also legacy — `cmd/server/main.go` does not call them. The live system uses only the `MultiClusterManager`.

### Multi-cluster client model

`pkg/k8s/multi_cluster.go` is the heart of the backend.

- `Manager` (`*MultiClusterManager`) is a global map of cluster name → `*ClusterClient`. Each `ClusterClient` owns its own `Clientset`, `MetricsClient`, `InformerFactory`, `RestConfig`, and `StopCh`.
- Connection sources, in order, on startup: (1) in-cluster service account if available, registered as `"default"`; (2) every context in `$KUBECONFIG`/`~/.kube/config`; (3) any kubeconfigs previously uploaded into `$KUBECONFIG_DIR` (default `./kubeconfigs`).
- Uploaded kubeconfigs persist on disk under `KUBECONFIG_DIR` and are reloaded on next start. In the in-cluster Deployment this is `emptyDir`, so uploads do **not** survive pod restarts there.
- Informers are started and `WaitForCacheSync`'d during cluster registration. Handlers list via the lister cache (`client.InformerFactory.Core().V1().Pods().Lister().List(...)`) and only hit the API for mutations or detail fetches.

### Route layout — dual trees

`pkg/api/router.go` defines two parallel route sets:

1. **Multi-cluster** under `/api/clusters/:cluster/...`, gated by `handlers.ClusterMiddleware()`, which resolves the cluster and stashes the `*ClusterClient` on the Gin context. Handlers retrieve it via `handlers.GetClusterClient(c)`.
2. **Legacy single-cluster** under `/api/...` (no `:cluster`), routed to the first cluster in the manager. Kept for backward compatibility.

When you add a new endpoint, add it to **both** trees. The legacy handlers live in `core_resources.go`, `workloads.go`, `additional_resources.go`, `network_extended.go`, `nodes.go`, `overview.go`, `storage.go`, `rbac.go`. Their multi-cluster twins live in `cluster_handlers.go`, `cluster_handlers_extended.go`, `workloads_extended.go`, `detail_handlers.go`. A multi-cluster handler is typically a thin wrapper that pulls the cluster client from the context, then calls the same logic.

### SSE: two distinct pipelines

- **Pod log streaming**: `handlers.StreamLogs` / `StreamClusterLogs` (`logs.go`) opens `Pods().GetLogs(...).Stream()` and copies to the response. Standard SSE.
- **Live resource watch**: `handlers.WatchClusterResource` (`resource_watch.go`) under `/api/clusters/:cluster/watch/:kind`. A `resourceBroker` per `(cluster, kind)` registers informer event handlers (`AddFunc`/`UpdateFunc`/`DeleteFunc`) and fans them out to subscribers over SSE. Per-subscriber channels are bounded (256); slow clients drop events and rely on a snapshot-on-reconnect plus periodic frontend reconciliation. When adding a new watchable kind, register it here, not as a separate endpoint.

### YAML editing and CRDs

- `yaml_edit.go` implements a generic kubectl-edit-style `GET/PUT /yaml/:kind/:namespace/:name` that switches over kind to choose the right typed client.
- Longhorn volumes (`longhorn.go`) are accessed via the dynamic client, attempting `v1beta2` then falling back to `v1beta1`. Use the same pattern for any other CRD.

### Frontend

- `web/index.html` (~3400 lines) contains nearly all of the UI: inline `<script>` blocks, plain DOM manipulation, no framework. Most feature work lives here.
- `web/src/main.js` is a small Vite-bundled shim that exposes `window.lucide` (icon library) and a lazy `window.loadCodeMirror()` for the YAML editor. When you need code-split chunks (anything you don't want in the inline scripts), add the dynamic import in `main.js` rather than inlining in `index.html`.
- `web/src/styles.css` runs through `@tailwindcss/vite`. Tailwind v4 auto-discovers classes from `web/**/*.{html,js}`.
- `vite.config.js` outputs to `../static/` (the directory served by Gin via `static.LocalFile("./static", true)`). `static/index.html` and `static/assets/` are git-ignored as build artifacts but `static/index.html` is currently tracked from before the `.gitignore` rule existed.

### Container layout

The distroless image runs as `nonroot` from `/app` with `./static/` next to the binary. The Gin server resolves `./static` relative to its working directory, so any container or local run must be invoked from a directory that has `static/` as a sibling (or the binary's parent directory must contain it). Don't move the static directory without updating `pkg/api/router.go`.

### RBAC

`k8s/rbac.yaml` grants the in-cluster service account read across most resources plus `delete/update/patch` on workloads. If you add a write action on a new resource, the ClusterRole there needs the matching verbs or the in-cluster deployment will start returning 403s even though local dev (which uses your kubeconfig) works fine.

## Conventions

- Handler files are split by resource family. New resources go into the file matching their family rather than a new file.
- List handlers return `{"items": [...]}`; detail handlers return the object directly. Keep that contract — the frontend depends on it.
- Always read from `client.InformerFactory.*.Lister()` for list/get when an informer exists. Hit `client.Clientset.*` only for mutations, log streams, and resources without an informer (CRDs, metrics).
- Mutations on workloads (delete/scale/restart/edit) are wired in `pod_actions.go`, `workloads.go` (legacy), and `cluster_handlers.go`/`workloads_extended.go` (multi-cluster). Restart is implemented as a strategic-merge-patch that bumps `kubectl.kubernetes.io/restartedAt`, mirroring `kubectl rollout restart`.
- The Gin router runs in `gin.ReleaseMode` and has a permissive CORS middleware (`*`). Don't rely on browser-origin checks for auth — there is none.
