package selector

import (
	"testing"

	storagev1 "k8s.io/api/storage/v1"
)

func TestGetNodeSelectorsForStorageClass(t *testing.T) {
	tests := []struct {
		name     string
		sc       *storagev1.StorageClass
		expected map[string]string
	}{
		{
			name: "nil parameters",
			sc: &storagev1.StorageClass{
				Parameters: nil,
			},
			expected: map[string]string{},
		},
		{
			name: "empty parameters",
			sc: &storagev1.StorageClass{
				Parameters: map[string]string{},
			},
			expected: map[string]string{},
		},
		{
			name: "hyperdisk-balanced",
			sc: &storagev1.StorageClass{
				Parameters: map[string]string{
					"type": "hyperdisk-balanced",
				},
			},
			expected: map[string]string{
				"volume.scheduling.cast.ai/hyperdisk-balanced": "true",
			},
		},
		{
			name: "hyperdisk-extreme",
			sc: &storagev1.StorageClass{
				Parameters: map[string]string{
					"type": "hyperdisk-extreme",
				},
			},
			expected: map[string]string{
				"volume.scheduling.cast.ai/hyperdisk-extreme": "true",
			},
		},
		{
			name: "hyperdisk-throughput",
			sc: &storagev1.StorageClass{
				Parameters: map[string]string{
					"type": "hyperdisk-throughput",
				},
			},
			expected: map[string]string{
				"volume.scheduling.cast.ai/hyperdisk-throughput": "true",
			},
		},
		{
			name: "pd-balanced",
			sc: &storagev1.StorageClass{
				Parameters: map[string]string{
					"type": "pd-balanced",
				},
			},
			expected: map[string]string{
				"volume.scheduling.cast.ai/pd-balanced": "true",
			},
		},
		{
			name: "pd-extreme",
			sc: &storagev1.StorageClass{
				Parameters: map[string]string{
					"type": "pd-extreme",
				},
			},
			expected: map[string]string{
				"volume.scheduling.cast.ai/pd-extreme": "true",
			},
		},
		{
			name: "pd-ssd",
			sc: &storagev1.StorageClass{
				Parameters: map[string]string{
					"type": "pd-ssd",
				},
			},
			expected: map[string]string{
				"volume.scheduling.cast.ai/pd-ssd": "true",
			},
		},
		{
			name: "pd-standard",
			sc: &storagev1.StorageClass{
				Parameters: map[string]string{
					"type": "pd-standard",
				},
			},
			expected: map[string]string{
				"volume.scheduling.cast.ai/pd-standard": "true",
			},
		},
		{
			name: "unknown disk type",
			sc: &storagev1.StorageClass{
				Parameters: map[string]string{
					"type": "unknown-disk-type",
				},
			},
			expected: map[string]string{},
		},
		{
			name: "missing type parameter",
			sc: &storagev1.StorageClass{
				Parameters: map[string]string{
					"otherParam": "value",
				},
			},
			expected: map[string]string{},
		},
		{
			name: "type parameter with additional parameters",
			sc: &storagev1.StorageClass{
				Parameters: map[string]string{
					"type":            "pd-ssd",
					"replication-type": "regional-pd",
					"fstype":          "ext4",
				},
			},
			expected: map[string]string{
				"volume.scheduling.cast.ai/pd-ssd": "true",
			},
		},
		{
			name: "empty type parameter",
			sc: &storagev1.StorageClass{
				Parameters: map[string]string{
					"type": "",
				},
			},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetNodeSelectorsForStorageClass(tt.sc)

			// Check if the length matches
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d selectors, got %d", len(tt.expected), len(result))
			}

			// Check each key-value pair
			for key, expectedValue := range tt.expected {
				if actualValue, ok := result[key]; !ok {
					t.Errorf("expected key %q not found in result", key)
				} else if actualValue != expectedValue {
					t.Errorf("for key %q: expected value %q, got %q", key, expectedValue, actualValue)
				}
			}

			// Check for unexpected keys
			for key := range result {
				if _, ok := tt.expected[key]; !ok {
					t.Errorf("unexpected key %q found in result", key)
				}
			}
		})
	}
}
