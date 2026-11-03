# GCP Disk Compatibility Webhook

Kubernetes mutating admission webhook that automatically adds node selectors to pods based on their GCP disk types. When a pod uses a PVC with a specific storage class, the webhook inspects the disk type (pd-balanced, pd-ssd, hyperdisk-balanced, etc.) and adds corresponding node selector labels like `volume.scheduling.cast.ai/pd-balanced=true`. This ensures pods are scheduled only on nodes that support their required disk types.

## How It Works

The webhook intercepts pod creation requests, examines all PVCs attached to the pod, retrieves their storage classes, and adds node selectors based on the GCP disk type parameter. Supports all GCP disk types: pd-standard, pd-ssd, pd-balanced, pd-extreme, hyperdisk-balanced, hyperdisk-extreme, and hyperdisk-throughput.

Certificates are automatically managed using cert-controller with leader-elected rotation across replicas.

## Build from source

```bash
go build .
```

Or use the Makefile helpers:

```bash
make help        # list available targets
make build       # build the binary into bin/webhook
make test        # unit tests with the race detector
make lint        # golangci-lint (downloads the pinned version from .golangci-lint-version)
make lint-fix    # golangci-lint with automatic fixes
make vet fmt tidy
```

The binary serves `/mutate` and `/health` endpoints and is configured with flags (also settable via `WEBHOOK_`-prefixed environment variables):

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8443` | Webhook server port |
| `--tls-cert-file` | `/etc/webhook/certs/tls.crt` | Path to TLS certificate file |
| `--tls-key-file` | `/etc/webhook/certs/tls.key` | Path to TLS key file |
| `--namespace` | `disk-compatibility-webhook` | Namespace where the webhook is deployed |
| `--service-name` | `disk-compatibility-webhook` | Name of the webhook service |
| `--secret-name` | `disk-compatibility-webhook-certs` | Name of the secret storing certificates |
| `--webhook-name` | `disk-compatibility-webhook` | Name of the MutatingWebhookConfiguration |
| `--disable-cert-rotator` | `false` | Disable automatic certificate rotation |

## Deploy with Helm

A prebuilt chart is also published in CAST AI's public Helm chart repository ([castai/helm-charts-public](https://github.com/castai/helm-charts-public), chart `gcp-disk-compatibility-webhook`), or you can deploy the in-repo chart:

```bash
helm upgrade -i gcp-disk-compatibility-webhook charts/gcp-disk-compatibility-webhook \
  --create-namespace -n gcp-disk-compatibility-webhook
```

Verify deployment:
```bash
kubectl get pods -n gcp-disk-compatibility-webhook
```

## Configuration

Override default values:
```bash
helm upgrade -i gcp-disk-compatibility-webhook charts/gcp-disk-compatibility-webhook \
  -n gcp-disk-compatibility-webhook \
  --set replicaCount=3 \
  --set resources.limits.memory=256Mi
```

Exclude namespaces:
```bash
kubectl label namespace my-namespace disk-compatibility-webhook=ignore
```

## Testing

Run unit tests:

```bash
make test
```

CI runs build, tests (race detector), and golangci-lint on every pull request (`.github/workflows/build.yaml`).

## Troubleshooting

View logs:
```bash
kubectl logs -n gcp-disk-compatibility-webhook -l app.kubernetes.io/name=gcp-disk-compatibility-webhook -f
```

Check webhook configuration:
```bash
kubectl get mutatingwebhookconfiguration gcp-disk-compatibility-webhook -o yaml
```

## Uninstall

```bash
helm uninstall gcp-disk-compatibility-webhook -n gcp-disk-compatibility-webhook
kubectl delete namespace gcp-disk-compatibility-webhook
```

## License

Distributed under the [MIT License](LICENSE).
