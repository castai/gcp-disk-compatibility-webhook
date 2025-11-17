package webhook

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetNodeSelectorsForPod(t *testing.T) {
	tests := []struct {
		name              string
		pod               *corev1.Pod
		namespace         string
		existingPVCs      []*corev1.PersistentVolumeClaim
		existingSCs       []*storagev1.StorageClass
		expectedSelectors map[string]string
		expectError       bool
	}{
		{
			name:      "pod with no volumes",
			namespace: "default",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{},
				},
			},
			expectedSelectors: map[string]string{},
			expectError:       false,
		},
		{
			name:      "pod with non-PVC volume",
			namespace: "default",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "my-config",
									},
								},
							},
						},
					},
				},
			},
			expectedSelectors: map[string]string{},
			expectError:       false,
		},
		{
			name:      "pod with PVC using pd-ssd",
			namespace: "default",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "my-pvc",
								},
							},
						},
					},
				},
			},
			existingPVCs: []*corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-pvc",
						Namespace: "default",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: stringPtr("pd-ssd-sc"),
					},
				},
			},
			existingSCs: []*storagev1.StorageClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pd-ssd-sc",
					},
					Parameters: map[string]string{
						"type": "pd-ssd",
					},
				},
			},
			expectedSelectors: map[string]string{
				"volume.scheduling.cast.ai/pd-ssd": "true",
			},
			expectError: false,
		},
		{
			name:      "pod with PVC using hyperdisk-balanced",
			namespace: "default",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "my-pvc",
								},
							},
						},
					},
				},
			},
			existingPVCs: []*corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-pvc",
						Namespace: "default",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: stringPtr("hyperdisk-sc"),
					},
				},
			},
			existingSCs: []*storagev1.StorageClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "hyperdisk-sc",
					},
					Parameters: map[string]string{
						"type": "hyperdisk-balanced",
					},
				},
			},
			expectedSelectors: map[string]string{
				"volume.scheduling.cast.ai/hyperdisk-balanced": "true",
			},
			expectError: false,
		},
		{
			name:      "pod with multiple PVCs using different disk types",
			namespace: "default",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "data1",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc1",
								},
							},
						},
						{
							Name: "data2",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc2",
								},
							},
						},
					},
				},
			},
			existingPVCs: []*corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc1",
						Namespace: "default",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: stringPtr("pd-ssd-sc"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc2",
						Namespace: "default",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: stringPtr("pd-standard-sc"),
					},
				},
			},
			existingSCs: []*storagev1.StorageClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pd-ssd-sc",
					},
					Parameters: map[string]string{
						"type": "pd-ssd",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pd-standard-sc",
					},
					Parameters: map[string]string{
						"type": "pd-standard",
					},
				},
			},
			expectedSelectors: map[string]string{
				"volume.scheduling.cast.ai/pd-ssd":      "true",
				"volume.scheduling.cast.ai/pd-standard": "true",
			},
			expectError: false,
		},
		{
			name:      "pod with PVC that doesn't exist",
			namespace: "default",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "non-existent-pvc",
								},
							},
						},
					},
				},
			},
			existingPVCs:      []*corev1.PersistentVolumeClaim{},
			existingSCs:       []*storagev1.StorageClass{},
			expectedSelectors: map[string]string{},
			expectError:       false, // Function logs warning but doesn't error
		},
		{
			name:      "pod with PVC with no storage class",
			namespace: "default",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "my-pvc",
								},
							},
						},
					},
				},
			},
			existingPVCs: []*corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-pvc",
						Namespace: "default",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: nil,
					},
				},
			},
			existingSCs:       []*storagev1.StorageClass{},
			expectedSelectors: map[string]string{},
			expectError:       false,
		},
		{
			name:      "pod with PVC with empty storage class name",
			namespace: "default",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "my-pvc",
								},
							},
						},
					},
				},
			},
			existingPVCs: []*corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-pvc",
						Namespace: "default",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: stringPtr(""),
					},
				},
			},
			existingSCs:       []*storagev1.StorageClass{},
			expectedSelectors: map[string]string{},
			expectError:       false,
		},
		{
			name:      "pod with PVC with storage class that has no type parameter",
			namespace: "default",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "my-pvc",
								},
							},
						},
					},
				},
			},
			existingPVCs: []*corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-pvc",
						Namespace: "default",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: stringPtr("custom-sc"),
					},
				},
			},
			existingSCs: []*storagev1.StorageClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "custom-sc",
					},
					Parameters: map[string]string{
						"other": "parameter",
					},
				},
			},
			expectedSelectors: map[string]string{},
			expectError:       false,
		},
		{
			name:      "pod with mixed PVC and non-PVC volumes",
			namespace: "default",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "my-config",
									},
								},
							},
						},
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "my-pvc",
								},
							},
						},
					},
				},
			},
			existingPVCs: []*corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-pvc",
						Namespace: "default",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: stringPtr("pd-extreme-sc"),
					},
				},
			},
			existingSCs: []*storagev1.StorageClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pd-extreme-sc",
					},
					Parameters: map[string]string{
						"type": "pd-extreme",
					},
				},
			},
			expectedSelectors: map[string]string{
				"volume.scheduling.cast.ai/pd-extreme": "true",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake clientset with existing objects
			objects := []runtime.Object{}
			for _, pvc := range tt.existingPVCs {
				objects = append(objects, pvc)
			}
			for _, sc := range tt.existingSCs {
				objects = append(objects, sc)
			}

			fakeClient := fake.NewSimpleClientset(objects...)

			// Create handler with fake clientset
			h := &Handler{
				clientset: fakeClient,
			}

			// Call the function
			result, err := h.getNodeSelectorsForPod(context.Background(), tt.pod, tt.namespace)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check result
			if len(result) != len(tt.expectedSelectors) {
				t.Errorf("expected %d selectors, got %d", len(tt.expectedSelectors), len(result))
			}

			for key, expectedValue := range tt.expectedSelectors {
				if actualValue, ok := result[key]; !ok {
					t.Errorf("expected key %q not found in result", key)
				} else if actualValue != expectedValue {
					t.Errorf("for key %q: expected value %q, got %q", key, expectedValue, actualValue)
				}
			}

			// Check for unexpected keys
			for key := range result {
				if _, ok := tt.expectedSelectors[key]; !ok {
					t.Errorf("unexpected key %q found in result", key)
				}
			}
		})
	}
}

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}
