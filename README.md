# k8s-dashboard

A lightweight, multi-cluster Kubernetes dashboard. Single Go binary serving a REST API and a static web UI. Connects to clusters via kubeconfig (uploadable at runtime) or in-cluster service account.

## Features

- Multi-cluster: connect to many clusters at once, switch between them in the UI, upload/import/remove kubeconfigs without restarting
- Read views for the common resource types: nodes, namespaces, pods, deployments, daemonsets, statefulsets, replicasets, jobs, cronjobs, services, ingresses, network policies, configmaps, secrets, PVs/PVCs, storage classes, RBAC, service accounts, HPAs, events
- Write actions: delete pods; delete/update/scale/restart deployments and statefulsets
- Pod log streaming over SSE
- Node and pod metrics (requires `metrics-server`)

## Project layout

```
cmd/server/        entry point
pkg/api/           Gin router and HTTP handlers
pkg/k8s/           clientset, informers, multi-cluster manager
static/            web UI (single index.html)
k8s/               namespace, RBAC, service, deployment manifests
Dockerfile         distroless image build
deploy.sh          build/push/deploy helper
```

## Run locally

```bash
go run ./cmd/server
```

Listens on `:8080`. By default reads `$KUBECONFIG` or `~/.kube/config`. The `/api/clusters` endpoint shows what was loaded.

Environment variables:

| Var | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `KUBECONFIG` | `~/.kube/config` | Kubeconfig to load on startup |
| `KUBECONFIG_DIR` | unset | Directory where uploaded kubeconfigs are persisted |

## Build

```bash
# Binary
CGO_ENABLED=0 go build -ldflags="-w -s" -o k8s-dashboard ./cmd/server

# Docker image
docker build -t quay.io/abhilash_bs1/k8s-dashboard:latest .
```

## Deploy to Kubernetes

Manifests under [k8s/](k8s/) create a dedicated namespace, ServiceAccount, ClusterRole/Binding (read-only across most resources, plus delete/update/scale on workloads), Deployment, and a NodePort Service on `30081`.

```bash
./deploy.sh all          # build, push, deploy
./deploy.sh deploy       # apply manifests only
./deploy.sh forward 8080 # port-forward to localhost
./deploy.sh logs
./deploy.sh clean
```

Image and tag come from `IMAGE_NAME` / `IMAGE_TAG` in [deploy.sh](deploy.sh).

When running in-cluster, the pod uses its ServiceAccount automatically. Additional clusters can be added by uploading their kubeconfigs through the UI or `POST /api/kubeconfig/upload`.

## API

- `GET  /api` — endpoint index
- `GET  /healthz`, `/readyz` — probes
- `GET  /api/clusters` — list connected clusters
- `POST /api/kubeconfig/upload` — upload a kubeconfig
- `POST /api/kubeconfig/preview`, `/api/kubeconfig/upload/preview` — list contexts before importing
- `GET  /api/kubeconfigs`, `DELETE /api/kubeconfigs/:name` — manage stored kubeconfigs
- `DELETE /api/clusters/:cluster` — disconnect a cluster
- `*    /api/clusters/:cluster/...` — per-cluster resource endpoints (pods, deployments, services, …)
- `*    /api/...` — legacy single-cluster endpoints, routed to the first connected cluster

Full route list in [pkg/api/router.go](pkg/api/router.go).
