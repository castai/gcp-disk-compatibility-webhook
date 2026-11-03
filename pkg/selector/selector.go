// Package selector derives node selector labels from Kubernetes storage
// classes, mapping GCP disk types to the CAST AI scheduling labels used to
// pin pods to compatible nodes.
package selector

import (
	storagev1 "k8s.io/api/storage/v1"
)

const (
	nodeLabelDomain = "volume.scheduling.cast.ai"
	valueTrue       = "true"
)

// GetNodeSelectorsForStorageClass returns a map of nodeSelectors based on the provided storage class.
// This function can be extended to implement custom logic for determining node affinity
// based on storage class properties (provisioner, parameters, etc.).
func GetNodeSelectorsForStorageClass(sc *storagev1.StorageClass) map[string]string {
	ret := map[string]string{}
	if sc.Parameters == nil {
		return ret
	}

	typeParam := sc.Parameters["type"]
	switch typeParam {
	case "hyperdisk-balanced", "hyperdisk-extreme", "hyperdisk-throughput",
		"pd-balanced", "pd-extreme", "pd-ssd", "pd-standard":
		ret[nodeLabelDomain+"/"+typeParam] = valueTrue
	default:
	}
	return ret
}
