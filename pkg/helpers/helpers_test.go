package helpers

import (
	"github.com/urfave/cli/v2"

	"context"
	"flag"
	"os"
	"testing"
)

func TestNewAppWithFlags(t *testing.T) {
	driverName := "test-driver"
	newDriver := func(ctx context.Context, config *Config) (*Driver, error) {
		return &Driver{}, nil
	}

	app := NewApp(driverName, newDriver)
	set := flag.NewFlagSet("test", 0)
	set.String("node-name", "test-node", "doc")
	set.String("cdi-root", "/test/cdi", "doc")
	set.Int("num-devices", 10, "doc")

	ctx := cli.NewContext(app, set, nil)

	err := app.Before(ctx)
	if err != nil {
		t.Fatalf("Before function failed: %v", err)
	}

	if ctx.String("node-name") != "test-node" {
		t.Errorf("Expected node-name to be 'test-node', got %v", ctx.String("node-name"))
	}

	if ctx.String("cdi-root") != "/test/cdi" {
		t.Errorf("Expected cdi-root to be '/test/cdi', got %v", ctx.String("cdi-root"))
	}

	if ctx.Int("num-devices") != 10 {
		t.Errorf("Expected num-devices to be 10, got %v", ctx.Int("num-devices"))
	}
}

func TestWriteFile(t *testing.T) {
	tests := []struct {
		name         string
		filePath     string
		fileContents string
		expectError  bool
	}{
		{
			name:         "Valid file path and contents",
			filePath:     "testfile.txt",
			fileContents: "Hello, World!",
			expectError:  false,
		},
		{
			name:         "Invalid file path",
			filePath:     "/invalidpath/testfile.txt",
			fileContents: "Hello, World!",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := WriteFile(tt.filePath, tt.fileContents)
			if (err != nil) != tt.expectError {
				t.Errorf("WriteFile() error = %v, expectError %v", err, tt.expectError)
			}

			if !tt.expectError {
				content, err := os.ReadFile(tt.filePath)
				if err != nil {
					t.Fatalf("Failed to read file: %v", err)
				}
				if string(content) != tt.fileContents {
					t.Errorf("Expected file contents to be %v, got %v", tt.fileContents, string(content))
				}
				os.Remove(tt.filePath)
			}
		})
	}
}
