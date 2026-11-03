# gcp-disk-compatibility-webhook chart

Deploys the GCP disk compatibility webhook: a mutating admission webhook that pins pods to GCP nodes compatible with the disk types of the volumes they mount (adds `volume.scheduling.cast.ai/<disk-type>` node selectors on pod CREATE). Certificates are managed automatically by cert-controller (leader-elected rotation).

## Permissions

The chart creates a ClusterRole (bound to the release's ServiceAccount) with the following grants. Review them before installing — they are cluster-scoped.

| Resource | Verbs | Why |
|---|---|---|
| `persistentvolumeclaims` (cluster-wide) | `get` | Look up the PVCs mounted by an admitted pod |
| `storageclasses.storage.k8s.io` (cluster-wide) | `get` | Resolve each PVC's storage class to its disk type |
| `mutatingwebhookconfigurations` named `gcp-disk-compatibility-webhook` | `get`, `update`, `patch` | Inject and rotate the webhook's CA bundle |
| `mutatingwebhookconfigurations` (cluster-wide) | `list`, `watch` | **Read-only, cluster-wide:** cert-controller's informer requires it to keep the CA bundle in sync. Note this makes the names, webhook service endpoints, and CA bundle values of *all* mutating webhook configurations in the cluster readable to the ServiceAccount. |
| `secrets` (release namespace) | `get`, `list`, `watch`, `create`, `update`, `patch` | Store and rotate the webhook TLS certificate |
| `leases.coordination.k8s.io` (release namespace) | `get`, `list`, `watch`, `create`, `update`, `patch` | Leader election for certificate rotation |

## Values

| Key | Default | Description |
|---|---|---|
| `replicaCount` | `2` | Webhook replicas (leader-elected cert rotation) |
| `image.repository` / `image.tag` | `ghcr.io/castai/gcp-disk-compatibility-webhook` / `.Chart.AppVersion` | Webhook image |
| `webhookFailurePolicy` | `Ignore` | Admission behavior if the webhook is unavailable |
| `podSecurityContext` / `securityContext` | nonroot 65532, all capabilities dropped, read-only root filesystem, RuntimeDefault seccomp | Hardened by default |
