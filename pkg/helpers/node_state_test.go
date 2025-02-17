package helpers

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestGetOrCreatePreparedClaims(t *testing.T) {
	tests := []struct {
		name           string
		initialContent string
		expectError    bool
		expectedClaims ClaimPreparations
	}{
		{
			name:           "FileExists",
			initialContent: `{"claim1": [{"device_name": "device1"}]}`,
			expectError:    false,
			expectedClaims: ClaimPreparations{
				"claim1": {
					{DeviceName: "device1"},
				},
			},
		},
		{
			name:           "FileNotExist",
			initialContent: "",
			expectError:    false,
			expectedClaims: ClaimPreparations{},
		},
		{
			name:           "InvalidFileContent",
			initialContent: `{"claim1": [`,
			expectError:    true,
			expectedClaims: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := "test_prepared_claims.json"
			defer os.Remove(filePath)

			if tt.initialContent != "" {
				err := os.WriteFile(filePath, []byte(tt.initialContent), 0600)
				if err != nil {
					t.Fatalf("failed to write initial content to file: %v", err)
				}
			}

			preparedClaims, err := GetOrCreatePreparedClaims(filePath)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected an error but got none: %v", err)
				}
				if preparedClaims != nil {
					t.Fatalf("expected preparedClaims to be nil but got: %v", preparedClaims)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if preparedClaims == nil {
					t.Fatalf("expected preparedClaims but got nil: %v", preparedClaims)
				}
				if !reflect.DeepEqual(tt.expectedClaims, preparedClaims) {
					t.Fatalf("expected %v but got %v", tt.expectedClaims, preparedClaims)
				}

				// Verify file creation
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					t.Fatalf("expected file to exist but it does not: %v", err)
				}
			}
		})
	}
}

func TestWritePreparedClaimsToFile(t *testing.T) {
	tests := []struct {
		name           string
		claims         ClaimPreparations
		expectedError  bool
		expectedOutput string
	}{
		{
			name: "ValidClaims",
			claims: ClaimPreparations{
				"claim1": {
					{DeviceName: "device1"},
				},
			},
			expectedError:  false,
			expectedOutput: `{"claim1":[{"device_name":"device1"}]}`,
		},
		{
			name:           "EmptyClaims",
			claims:         ClaimPreparations{},
			expectedError:  false,
			expectedOutput: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := "test_write_prepared_claims.json"
			defer os.Remove(filePath)

			err := WritePreparedClaimsToFile(filePath, tt.claims)

			if tt.expectedError {
				if err == nil {
					t.Fatalf("expected an error but got none: %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				content, err := os.ReadFile(filePath)
				if err != nil {
					t.Fatalf("failed to read file: %v", err)
				}

				var actualOutput map[string]interface{}
				var expectedOutput map[string]interface{}

				if err := json.Unmarshal(content, &actualOutput); err != nil {
					t.Fatalf("failed to unmarshal actual output: %v", err)
				}

				if err := json.Unmarshal([]byte(tt.expectedOutput), &expectedOutput); err != nil {
					t.Fatalf("failed to unmarshal expected output: %v", err)
				}

				if !reflect.DeepEqual(actualOutput, expectedOutput) {
					t.Fatalf("expected %v but got %v", expectedOutput, actualOutput)
				}
			}
		})
	}
}
