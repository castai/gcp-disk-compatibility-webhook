# GCP Disk Compatibility Webhook

A Kubernetes mutating admission webhook that automatically adds `nodeSelector` labels to pods based on the storage classes of their persistent volume claims (PVCs). This ensures that pods are scheduled on nodes that are compatible with their required disk types.

## Overview

This webhook intercepts pod creation requests and:
1. Inspects all PersistentVolumeClaims (PVCs) referenced by the pod
2. Retrieves the StorageClass for each PVC
3. Determines appropriate nodeSelectors based on the storage class
4. Mutates the pod to add the required nodeSelectors

## Architecture

The webhook consists of three main components:

### 1. Selector Function (`pkg/selector/selector.go`)
```go
func GetNodeSelectorsForStorageClass(sc *storagev1.StorageClass) map[string]string
```
This function takes a StorageClass object and returns a map of nodeSelector labels. Currently returns an empty map but can be extended with custom logic.

### 2. Webhook Handler (`pkg/webhook/handler.go`)
Handles admission review requests:
- Deserializes incoming AdmissionReview requests
- Extracts pod specifications
- Queries Kubernetes API for PVC and StorageClass information
- Applies JSON patches to add nodeSelectors
- Returns AdmissionReview response

### 3. HTTPS Server (`main.go`)
Sets up an HTTPS server that:
- Listens on port 8443
- Handles `/mutate` endpoint for admission reviews
- Provides `/health` endpoint for health checks
- Uses TLS certificates from Kubernetes secrets

## Prerequisites

- Go 1.21 or later
- Kubernetes cluster (1.16+)
- kubectl configured to access your cluster
- Docker (for building container images)
- OpenSSL (for generating certificates)

## Installation

### Step 1: Build the Docker Image

```bash
make docker-build
```

Or manually:
```bash
docker build -t disk-compatibility-webhook:latest .
```

If using a remote registry, tag and push:
```bash
docker tag disk-compatibility-webhook:latest your-registry/disk-compatibility-webhook:latest
docker push your-registry/disk-compatibility-webhook:latest
```

### Step 2: Deploy to Kubernetes

Generate certificates and deploy:
```bash
make deploy
```

Or manually:
```bash
# Generate certificates
./deploy/generate-certs.sh

# Apply manifests
kubectl apply -f deploy/namespace.yaml
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/service.yaml
kubectl apply -f deploy/deployment.yaml
kubectl apply -f deploy/webhook.yaml
```

### Step 3: Verify Deployment

Check that the webhook is running:
```bash
kubectl get pods -n disk-compatibility-webhook
kubectl logs -n disk-compatibility-webhook -l app=disk-compatibility-webhook
```

## Configuration

### Customizing the Selector Function

The core logic for determining nodeSelectors is in `pkg/selector/selector.go`. You can extend it to implement custom rules:

```go
func GetNodeSelectorsForStorageClass(sc *storagev1.StorageClass) map[string]string {
    selectors := make(map[string]string)

    // Example: Add nodeSelector for GCP regional PD disks
    if sc.Provisioner == "pd.csi.storage.gke.io" {
        if sc.Parameters["type"] == "pd-balanced" {
            selectors["cloud.google.com/gke-nodepool"] = "pd-compatible"
        }
    }

    return selectors
}
```

### Namespace Filtering

By default, the webhook applies to all namespaces except those labeled with `disk-compatibility-webhook: ignore`. To exclude a namespace:

```bash
kubectl label namespace my-namespace disk-compatibility-webhook=ignore
```

### Webhook Failure Policy

The webhook is configured with `failurePolicy: Ignore`, meaning pod creation will proceed even if the webhook fails. To make it strict, change this to `Fail` in `deploy/webhook.yaml`.

## RBAC Permissions

The webhook requires the following permissions:
- `get`, `list` on `persistentvolumeclaims`
- `get`, `list` on `storageclasses`

These are defined in `deploy/rbac.yaml`.

## Testing

### Unit Tests
```bash
make test
```

### Manual Testing

1. Create a test StorageClass:
```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: test-storage-class
provisioner: kubernetes.io/no-provisioner
volumeBindingMode: WaitForFirstConsumer
```

2. Create a PVC using the StorageClass:
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc
spec:
  storageClassName: test-storage-class
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
```

3. Create a pod that uses the PVC:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: test
    image: nginx
    volumeMounts:
    - name: storage
      mountPath: /data
  volumes:
  - name: storage
    persistentVolumeClaim:
      claimName: test-pvc
```

4. Check if the webhook added nodeSelectors:
```bash
kubectl get pod test-pod -o yaml | grep -A 5 nodeSelector
```

## Troubleshooting

### Webhook Not Being Called

Check the MutatingWebhookConfiguration:
```bash
kubectl get mutatingwebhookconfiguration disk-compatibility-webhook -o yaml
```

Verify the caBundle is set correctly.

### Certificate Issues

Regenerate certificates:
```bash
make generate-certs
kubectl rollout restart deployment disk-compatibility-webhook -n disk-compatibility-webhook
```

### Check Webhook Logs

```bash
kubectl logs -n disk-compatibility-webhook -l app=disk-compatibility-webhook -f
```

### Test Webhook Directly

Port-forward to the webhook service:
```bash
kubectl port-forward -n disk-compatibility-webhook svc/disk-compatibility-webhook 8443:443
```

Send a test admission review request (requires valid client certificates).

## Cleanup

Remove all webhook resources:
```bash
make clean
```

Or manually:
```bash
kubectl delete -f deploy/webhook.yaml
kubectl delete -f deploy/deployment.yaml
kubectl delete -f deploy/service.yaml
kubectl delete -f deploy/rbac.yaml
kubectl delete namespace disk-compatibility-webhook
```

## Development

### Project Structure

```
.
├── main.go                          # Entry point, HTTPS server
├── pkg/
│   ├── selector/
│   │   └── selector.go             # StorageClass to nodeSelector mapping
│   └── webhook/
│       └── handler.go              # Admission webhook logic
├── deploy/
│   ├── namespace.yaml              # Namespace definition
│   ├── rbac.yaml                   # ServiceAccount, ClusterRole, ClusterRoleBinding
│   ├── service.yaml                # Kubernetes Service
│   ├── deployment.yaml             # Webhook deployment
│   ├── webhook.yaml                # MutatingWebhookConfiguration
│   └── generate-certs.sh           # Certificate generation script
├── Dockerfile                       # Container image definition
├── Makefile                         # Build and deployment shortcuts
└── README.md                        # This file
```

### Adding New Features

1. Extend `GetNodeSelectorsForStorageClass` in `pkg/selector/selector.go`
2. Add unit tests
3. Rebuild and redeploy:
   ```bash
   make docker-build
   kubectl rollout restart deployment disk-compatibility-webhook -n disk-compatibility-webhook
   ```

## License

MIT

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.
