# GCP Disk Compatibility Webhook

A Kubernetes mutating admission webhook that automatically adds `nodeSelector` labels to pods based on the storage classes of their persistent volume claims (PVCs). This ensures that pods are scheduled on nodes that are compatible with their required disk types.

## Overview

This webhook intercepts pod creation requests and:
1. Inspects all PersistentVolumeClaims (PVCs) referenced by the pod
2. Retrieves the StorageClass for each PVC
3. Determines appropriate nodeSelectors based on the storage class
4. Mutates the pod to add the required nodeSelectors

## Features

- **Automatic Certificate Management**: Uses [cert-controller](https://github.com/open-policy-agent/cert-controller) to automatically generate and rotate TLS certificates
- **Helm Deployment**: Simple installation and upgrades using Helm charts
- **Flexible Configuration**: Configure via CLI flags or environment variables using Viper
- **Comprehensive Testing**: Includes table-driven unit tests for all core functionality
- **GCP Disk Type Support**: Supports all GCP disk types (pd-standard, pd-ssd, pd-balanced, pd-extreme, hyperdisk-balanced, hyperdisk-extreme, hyperdisk-throughput)

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
- Uses Cobra for CLI commands and Viper for configuration
- Supports both CLI flags and environment variables (with `WEBHOOK_` prefix)
- Listens on port 8443 (configurable)
- Handles `/mutate` endpoint for admission reviews
- Provides `/health` endpoint for health checks
- Uses TLS certificates automatically managed by cert-controller

## Prerequisites

- Kubernetes cluster (1.16+)
- kubectl configured to access your cluster
- Helm 3.x or later

## Installation

### Deploy Using Helm

The webhook includes automatic certificate management using [cert-controller](https://github.com/open-policy-agent/cert-controller), so no manual certificate generation is required.

Deploy the webhook using Helm:

```bash
helm upgrade -i -n gcp-disk-compatibility-webhook gcp-disk-compatibility-webhook --create-namespace charts/gcp-disk-compatibility-webhook
```

### Verify Deployment

Check that the webhook is running:
```bash
kubectl get pods -n gcp-disk-compatibility-webhook
kubectl logs -n gcp-disk-compatibility-webhook -l app.kubernetes.io/name=gcp-disk-compatibility-webhook
```

Check that certificates have been automatically generated:
```bash
kubectl get secret -n gcp-disk-compatibility-webhook disk-compatibility-webhook-certs
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

### Helm Configuration

You can customize the deployment by overriding values in the Helm chart:

```bash
helm upgrade -i -n gcp-disk-compatibility-webhook gcp-disk-compatibility-webhook \
  --create-namespace \
  --set image.tag=v1.0.0 \
  --set resources.limits.memory=256Mi \
  charts/gcp-disk-compatibility-webhook
```

See `charts/gcp-disk-compatibility-webhook/values.yaml` for all available configuration options.

### Namespace Filtering

By default, the webhook applies to all namespaces except those labeled with `disk-compatibility-webhook: ignore`. To exclude a namespace:

```bash
kubectl label namespace my-namespace disk-compatibility-webhook=ignore
```

## RBAC Permissions

The webhook requires the following permissions:
- `get`, `list` on `persistentvolumeclaims`
- `get`, `list` on `storageclasses`

These are automatically configured by the Helm chart.

## Testing

### Unit Tests
```bash
go test ./...
```

Or test specific packages:
```bash
go test ./pkg/selector/...
go test ./pkg/webhook/...
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

Verify the caBundle is set correctly. The cert-controller automatically manages this.

### Certificate Issues

Certificates are automatically managed by cert-controller. If you encounter certificate issues:

1. Check the cert-controller logs:
```bash
kubectl logs -n gcp-disk-compatibility-webhook -l app.kubernetes.io/name=gcp-disk-compatibility-webhook -c manager
```

2. Verify the secret exists:
```bash
kubectl get secret -n gcp-disk-compatibility-webhook disk-compatibility-webhook-certs
```

3. Restart the webhook to regenerate certificates:
```bash
kubectl delete pod -n gcp-disk-compatibility-webhook -l app.kubernetes.io/name=gcp-disk-compatibility-webhook
```

### Check Webhook Logs

```bash
kubectl logs -n gcp-disk-compatibility-webhook -l app.kubernetes.io/name=gcp-disk-compatibility-webhook -f
```

### Test Webhook Directly

Port-forward to the webhook service:
```bash
kubectl port-forward -n gcp-disk-compatibility-webhook svc/disk-compatibility-webhook 8443:443
```

Send a test admission review request (requires valid client certificates).

## Cleanup

Remove the webhook using Helm:
```bash
helm uninstall -n gcp-disk-compatibility-webhook gcp-disk-compatibility-webhook
```

To also remove the namespace:
```bash
kubectl delete namespace gcp-disk-compatibility-webhook
```

## Development

### Project Structure

```
.
├── main.go                                  # Entry point, HTTPS server with Cobra/Viper
├── pkg/
│   ├── selector/
│   │   ├── selector.go                     # StorageClass to nodeSelector mapping
│   │   └── selector_test.go               # Unit tests for selector
│   └── webhook/
│       ├── handler.go                      # Admission webhook logic
│       └── handler_test.go                 # Unit tests for webhook handler
├── charts/
│   └── gcp-disk-compatibility-webhook/    # Helm chart
│       ├── Chart.yaml
│       ├── values.yaml
│       └── templates/
├── Dockerfile                               # Container image definition
└── README.md                                # This file
```

### Configuration via Environment Variables

All CLI flags can be configured via environment variables with the `WEBHOOK_` prefix:

```bash
# Example: Set port via environment variable
WEBHOOK_PORT=9090

# Example: Disable certificate rotator
WEBHOOK_DISABLE_CERT_ROTATOR=true
```

### Adding New Features

1. Extend `GetNodeSelectorsForStorageClass` in `pkg/selector/selector.go`
2. Add unit tests in the corresponding `*_test.go` file
3. Build and push a new image:
   ```bash
   docker build -t your-registry/gcp-disk-compatibility-webhook:v1.0.0 .
   docker push your-registry/gcp-disk-compatibility-webhook:v1.0.0
   ```
4. Upgrade the Helm release:
   ```bash
   helm upgrade -i -n gcp-disk-compatibility-webhook gcp-disk-compatibility-webhook \
     --set image.tag=v1.0.0 \
     charts/gcp-disk-compatibility-webhook
   ```

## License

MIT

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.
