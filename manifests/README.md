# Kubernetes Manifests - Port K8s Exporter (WEBHOOK Mode)

These manifests deploy the Port K8s Exporter in **WEBHOOK mode** - no Port credentials required.

## Quick Start

### 1. Build and push the Docker image

```bash
# From the project root
docker build -t your-registry/port-k8s-exporter:v1.0 .
docker push your-registry/port-k8s-exporter:v1.0
```

### 2. Configure the webhook URL

Edit `secret.yaml` and set your webhook URL from [ingest.getport.io](https://ingest.getport.io):

```yaml
stringData:
  WEBHOOK_URL: "https://ingest.getport.io/YOUR_ACTUAL_WEBHOOK_ID"
```

### 3. Update the image in deployment.yaml

```yaml
image: your-registry/port-k8s-exporter:v1.0
```

### 4. Deploy

```bash
# Option A: Using kustomize (recommended)
kubectl apply -k manifests/

# Option B: Individual files
kubectl apply -f manifests/namespace.yaml
kubectl apply -f manifests/serviceaccount.yaml
kubectl apply -f manifests/clusterrole.yaml
kubectl apply -f manifests/clusterrolebinding.yaml
kubectl apply -f manifests/configmap.yaml
kubectl apply -f manifests/secret.yaml
kubectl apply -f manifests/deployment.yaml
```

### 5. Verify deployment

```bash
# Check pod is running
kubectl get pods -n port-k8s-exporter

# Check logs
kubectl logs -n port-k8s-exporter -l app.kubernetes.io/name=port-k8s-exporter -f
```

## Files

| File | Description |
|------|-------------|
| `namespace.yaml` | Dedicated namespace for isolation |
| `serviceaccount.yaml` | Identity for the exporter pod |
| `clusterrole.yaml` | Permissions to read all cluster resources |
| `clusterrolebinding.yaml` | Binds ClusterRole to ServiceAccount |
| `configmap.yaml` | Configuration (which resources to export) |
| `secret.yaml` | Webhook URL and optional HMAC secret |
| `deployment.yaml` | The actual exporter pod |
| `kustomization.yaml` | Kustomize configuration |

## Why this survives cluster restarts

```
Deployment (replicas: 1)
    │
    ├── restartPolicy: Always (default)
    │   └── If container crashes → K8s restarts it
    │
    ├── Pod managed by ReplicaSet
    │   └── If pod is deleted → ReplicaSet creates new one
    │
    └── Deployment controller
        └── If node goes down → Pod rescheduled on another node
```

**Kubernetes guarantees your exporter keeps running 24/7.**

## Customization

### Change cluster name detection

By default, the exporter auto-detects the cluster name. To override:

```yaml
# In deployment.yaml, add to args:
args:
  - --cluster-name=my-production-cluster
```

### Add HMAC signature security

1. Set a secret in `secret.yaml`:
   ```yaml
   WEBHOOK_SECRET: "your-hmac-secret"
   ```

2. Add args in `deployment.yaml`:
   ```yaml
   args:
     - --webhook-secret=$(WEBHOOK_SECRET)
     - --webhook-signature-header=X-Port-Signature
     - --webhook-signature-algorithm=sha256
   ```

3. Configure the same secret in Port's webhook settings.

## Uninstall

```bash
kubectl delete -k manifests/
```
