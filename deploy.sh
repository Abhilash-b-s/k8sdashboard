#!/bin/bash

# Kubernetes Dashboard - Build and Deploy Script
# Usage: ./deploy.sh [build|deploy|all|clean]

set -e

# Configuration
IMAGE_NAME="quay.io/abhilash_bs1/k8s-dashboard"
IMAGE_TAG="${IMAGE_TAG:-latest}"
FULL_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"
K8S_DIR="./k8s"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored message
print_msg() {
    local color=$1
    local msg=$2
    echo -e "${color}${msg}${NC}"
}

# Print step header
print_step() {
    echo ""
    print_msg "$BLUE" "======================================"
    print_msg "$BLUE" "$1"
    print_msg "$BLUE" "======================================"
}

# Check if command exists
check_command() {
    if ! command -v "$1" &> /dev/null; then
        print_msg "$RED" "Error: $1 is not installed or not in PATH"
        exit 1
    fi
}

# Build the Go binary locally (optional)
build_local() {
    print_step "Building Go binary locally"
    check_command "go"

    print_msg "$YELLOW" "Running go mod download..."
    go mod download

    print_msg "$YELLOW" "Building binary..."
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags="-w -s" \
        -o k8s-dashboard ./cmd/server

    print_msg "$GREEN" "Binary built successfully: ./k8s-dashboard"
}

# Build Docker image
build_docker() {
    print_step "Building Docker Image"
    check_command "docker"

    print_msg "$YELLOW" "Building image: ${FULL_IMAGE}"
    docker build -t "${FULL_IMAGE}" .

    print_msg "$GREEN" "Docker image built successfully: ${FULL_IMAGE}"
}

# Push Docker image to registry
push_image() {
    print_step "Pushing Docker Image"
    check_command "docker"

    print_msg "$YELLOW" "Pushing image: ${FULL_IMAGE}"
    docker push "${FULL_IMAGE}"

    print_msg "$GREEN" "Docker image pushed successfully"
}

# Deploy to Kubernetes
deploy_k8s() {
    print_step "Deploying to Kubernetes"
    check_command "kubectl"

    # Check if kubectl can connect to cluster
    if ! kubectl cluster-info &> /dev/null; then
        print_msg "$RED" "Error: Cannot connect to Kubernetes cluster"
        print_msg "$YELLOW" "Make sure kubectl is configured correctly"
        exit 1
    fi

    print_msg "$YELLOW" "Applying Kubernetes manifests..."

    # Apply in order: namespace -> rbac -> service -> deployment
    if [ -f "${K8S_DIR}/namespace.yaml" ]; then
        print_msg "$YELLOW" "Creating namespace..."
        kubectl apply -f "${K8S_DIR}/namespace.yaml"
    fi

    if [ -f "${K8S_DIR}/rbac.yaml" ]; then
        print_msg "$YELLOW" "Applying RBAC..."
        kubectl apply -f "${K8S_DIR}/rbac.yaml"
    fi

    if [ -f "${K8S_DIR}/service.yaml" ]; then
        print_msg "$YELLOW" "Creating service..."
        kubectl apply -f "${K8S_DIR}/service.yaml"
    fi

    if [ -f "${K8S_DIR}/deployment.yaml" ]; then
        print_msg "$YELLOW" "Creating deployment..."
        kubectl apply -f "${K8S_DIR}/deployment.yaml"
    fi

    print_msg "$GREEN" "Kubernetes resources applied successfully"

    # Force rollout restart to ensure new pods are created with latest image
    print_msg "$YELLOW" "Triggering rollout restart to replace old pods..."

    # Add timestamp annotation to force image pull even with IfNotPresent policy
    kubectl patch deployment k8s-dashboard -n k8s-dashboard \
        -p "{\"spec\":{\"template\":{\"metadata\":{\"annotations\":{\"kubectl.kubernetes.io/restartedAt\":\"$(date -Iseconds)\"}}}}}"

    # Wait for deployment to be ready
    print_msg "$YELLOW" "Waiting for rollout to complete..."
    kubectl rollout status deployment/k8s-dashboard -n k8s-dashboard --timeout=120s

    print_msg "$GREEN" "Deployment is ready!"

    # Show pod status
    echo ""
    print_msg "$BLUE" "Pod Status:"
    kubectl get pods -n k8s-dashboard -l app=k8s-dashboard

    # Show service info
    echo ""
    print_msg "$BLUE" "Service Info:"
    kubectl get svc -n k8s-dashboard
}

# Restart deployment (to pick up new image)
restart_deployment() {
    print_step "Restarting Deployment"
    check_command "kubectl"

    print_msg "$YELLOW" "Restarting deployment to pull latest image..."

    # Add timestamp annotation to force new pod creation and image pull
    kubectl patch deployment k8s-dashboard -n k8s-dashboard \
        -p "{\"spec\":{\"template\":{\"metadata\":{\"annotations\":{\"kubectl.kubernetes.io/restartedAt\":\"$(date -Iseconds)\"}}}}}"

    print_msg "$YELLOW" "Waiting for rollout to complete..."
    kubectl rollout status deployment/k8s-dashboard -n k8s-dashboard --timeout=120s

    # Show new pod status
    echo ""
    print_msg "$BLUE" "New Pod Status:"
    kubectl get pods -n k8s-dashboard -l app=k8s-dashboard

    print_msg "$GREEN" "Deployment restarted successfully"
}

# Clean up / delete deployment
clean() {
    print_step "Cleaning up Kubernetes resources"
    check_command "kubectl"

    print_msg "$YELLOW" "Deleting Kubernetes resources..."
    kubectl delete -f "${K8S_DIR}/" --ignore-not-found=true

    print_msg "$GREEN" "Cleanup complete"
}

# Show deployment status
status() {
    print_step "Deployment Status"
    check_command "kubectl"

    echo ""
    print_msg "$BLUE" "Namespace:"
    kubectl get namespace k8s-dashboard 2>/dev/null || print_msg "$YELLOW" "Namespace not found"

    echo ""
    print_msg "$BLUE" "Pods:"
    kubectl get pods -n k8s-dashboard -l app=k8s-dashboard 2>/dev/null || print_msg "$YELLOW" "No pods found"

    echo ""
    print_msg "$BLUE" "Service:"
    kubectl get svc -n k8s-dashboard 2>/dev/null || print_msg "$YELLOW" "No service found"

    echo ""
    print_msg "$BLUE" "Deployment:"
    kubectl get deployment -n k8s-dashboard 2>/dev/null || print_msg "$YELLOW" "No deployment found"
}

# Show logs
logs() {
    print_step "Pod Logs"
    check_command "kubectl"

    kubectl logs -n k8s-dashboard -l app=k8s-dashboard --tail=100 -f
}

# Port forward for local access
port_forward() {
    print_step "Port Forwarding"
    check_command "kubectl"

    local port="${1:-8080}"
    print_msg "$GREEN" "Access dashboard at: http://localhost:${port}"
    print_msg "$YELLOW" "Press Ctrl+C to stop port forwarding"
    kubectl port-forward -n k8s-dashboard svc/k8s-dashboard "${port}:80"
}

# Show usage
usage() {
    echo ""
    echo "Kubernetes Dashboard - Build and Deploy Script"
    echo ""
    echo "Usage: $0 <command> [options]"
    echo ""
    echo "Commands:"
    echo "  build         Build Docker image"
    echo "  push          Push Docker image to registry"
    echo "  deploy        Deploy to Kubernetes cluster"
    echo "  all           Build, push, and deploy"
    echo "  restart       Restart deployment (pull latest image)"
    echo "  clean         Delete all Kubernetes resources"
    echo "  status        Show deployment status"
    echo "  logs          Show pod logs (follow)"
    echo "  forward [port] Port forward to localhost (default: 8080)"
    echo ""
    echo "Environment Variables:"
    echo "  IMAGE_TAG     Docker image tag (default: latest)"
    echo ""
    echo "Examples:"
    echo "  $0 build                    # Build Docker image"
    echo "  $0 all                      # Build, push, and deploy"
    echo "  IMAGE_TAG=v1.0.0 $0 all     # Build with specific tag"
    echo "  $0 forward 9090             # Port forward to localhost:9090"
    echo ""
}

# Main
case "${1:-}" in
    build)
        build_docker
        ;;
    push)
        push_image
        ;;
    deploy)
        deploy_k8s
        ;;
    all)
        build_docker
        push_image
        deploy_k8s
        ;;
    restart)
        restart_deployment
        ;;
    clean)
        clean
        ;;
    status)
        status
        ;;
    logs)
        logs
        ;;
    forward)
        port_forward "${2:-8080}"
        ;;
    *)
        usage
        exit 1
        ;;
esac
