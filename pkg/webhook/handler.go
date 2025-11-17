package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cast-ai/gcp-disk-compatibility-webhook/pkg/selector"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type Handler struct {
	clientset kubernetes.Interface
}

func NewHandler() (*Handler, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	return &Handler{
		clientset: clientset,
	}, nil
}

// Handle processes the admission review request
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		klog.Error("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *admissionv1.AdmissionResponse
	ar := admissionv1.AdmissionReview{}
	if err := json.Unmarshal(body, &ar); err != nil {
		klog.Errorf("Can't decode body: %v", err)
		admissionResponse = &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = h.mutatePod(&ar)
	}

	admissionReview := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
	}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		klog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		return
	}

	if _, err := w.Write(resp); err != nil {
		klog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

// mutatePod mutates the pod to add nodeSelectors based on storage class
func (h *Handler) mutatePod(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := ar.Request
	if req == nil {
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: "admission request is nil",
			},
		}
	}

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		klog.Errorf("Could not unmarshal raw object: %v", err)
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	klog.Infof("AdmissionReview for Kind=%v, Namespace=%v Name=%v UID=%v",
		req.Kind, req.Namespace, req.Name, req.UID)

	// Get storage classes used by this pod's PVCs
	nodeSelectors, err := h.getNodeSelectorsForPod(context.Background(), &pod, req.Namespace)
	if err != nil {
		klog.Errorf("Failed to get node selectors: %v", err)
		return &admissionv1.AdmissionResponse{
			Allowed: true, // Allow the pod even if we fail to get selectors
		}
	}

	// If no nodeSelectors are needed, allow the pod without mutation
	if len(nodeSelectors) == 0 {
		klog.Infof("No nodeSelectors needed for pod %s/%s", req.Namespace, pod.Name)
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	// Create JSON patch to add nodeSelectors
	patches := []map[string]interface{}{}

	// Initialize nodeSelector if it doesn't exist
	if pod.Spec.NodeSelector == nil {
		patches = append(patches, map[string]interface{}{
			"op":    "add",
			"path":  "/spec/nodeSelector",
			"value": nodeSelectors,
		})
	} else {
		// Merge with existing nodeSelectors
		for key, value := range nodeSelectors {
			patches = append(patches, map[string]interface{}{
				"op":    "add",
				"path":  fmt.Sprintf("/spec/nodeSelector/%s", escapeJSONPointer(key)),
				"value": value,
			})
		}
	}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		klog.Errorf("Failed to marshal patches: %v", err)
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	klog.Infof("Applying patches to pod %s/%s: %s", req.Namespace, pod.Name, string(patchBytes))

	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

// getNodeSelectorsForPod retrieves the storage classes used by the pod's PVCs
// and returns the appropriate nodeSelectors
func (h *Handler) getNodeSelectorsForPod(ctx context.Context, pod *corev1.Pod, namespace string) (map[string]string, error) {
	allNodeSelectors := make(map[string]string)

	// Iterate through all volumes in the pod
	for _, volume := range pod.Spec.Volumes {
		// Check if the volume uses a PVC
		if volume.PersistentVolumeClaim == nil {
			continue
		}

		pvcName := volume.PersistentVolumeClaim.ClaimName

		// Get the PVC
		pvc, err := h.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			klog.Warningf("Failed to get PVC %s/%s: %v", namespace, pvcName, err)
			continue
		}

		// Get the storage class name
		if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName == "" {
			klog.Infof("PVC %s/%s has no storage class", namespace, pvcName)
			continue
		}

		storageClassName := *pvc.Spec.StorageClassName

		// Get the storage class
		sc, err := h.clientset.StorageV1().StorageClasses().Get(ctx, storageClassName, metav1.GetOptions{})
		if err != nil {
			klog.Warningf("Failed to get storage class %s: %v", storageClassName, err)
			continue
		}

		// Get nodeSelectors for this storage class
		nodeSelectors := selector.GetNodeSelectorsForStorageClass(sc)

		// Merge nodeSelectors
		for key, value := range nodeSelectors {
			allNodeSelectors[key] = value
		}

		klog.Infof("Storage class %s requires nodeSelectors: %v", storageClassName, nodeSelectors)
	}

	return allNodeSelectors, nil
}

// escapeJSONPointer escapes a string for use in a JSON pointer path
func escapeJSONPointer(s string) string {
	s = string([]byte(s)) // ensure we're working with a fresh string
	// JSON Pointer escaping: ~ becomes ~0, / becomes ~1
	result := ""
	for _, c := range s {
		switch c {
		case '~':
			result += "~0"
		case '/':
			result += "~1"
		default:
			result += string(c)
		}
	}
	return result
}
