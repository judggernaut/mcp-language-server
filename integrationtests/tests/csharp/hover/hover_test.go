package hover_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/csharp/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestHover tests hover functionality with the C# language server (csharp-ls)
func TestHover(t *testing.T) {
	// Helper function to open all files and wait for indexing
	openAllFilesAndWait := func(suite *common.TestSuite, ctx context.Context) {
		filesToOpen := []string{
			"Program.cs",
			"Types.cs",
			"Helper.cs",
			"Consumer.cs",
			"AnotherConsumer.cs",
			"Clean.cs",
		}

		for _, file := range filesToOpen {
			filePath := filepath.Join(suite.WorkspaceDir, file)
			err := suite.Client.OpenFile(ctx, filePath)
			if err != nil {
				// Don't fail the test, some files might not exist in certain tests
				t.Logf("Note: Failed to open %s: %v", file, err)
			}
		}
	}

	tests := []struct {
		name           string
		file           string
		line           int
		column         int
		expectedText   string // Text that should be in the hover result
		unexpectedText string // Text that should NOT be in the hover result (optional)
		snapshotName   string
	}{
		// Tests using Types.cs file
		{
			name:         "Class",
			file:         "Types.cs",
			line:         4,
			column:       14,
			expectedText: "SharedClass",
			snapshotName: "class-type",
		},
		{
			name:         "ClassMethod",
			file:         "Types.cs",
			line:         12,
			column:       19,
			expectedText: "Method",
			snapshotName: "class-method",
		},
		{
			name:         "Interface",
			file:         "Types.cs",
			line:         31,
			column:       18,
			expectedText: "SharedInterface",
			snapshotName: "interface-type",
		},
		{
			name:         "Constant",
			file:         "Types.cs",
			line:         40,
			column:       25,
			expectedText: "SharedConstant",
			snapshotName: "constant",
		},
		{
			name:         "Enum",
			file:         "Types.cs",
			line:         44,
			column:       13,
			expectedText: "SharedType",
			snapshotName: "enum",
		},
		{
			name:         "Field",
			file:         "Types.cs",
			line:         9,
			column:       25,
			expectedText: "Constants",
			snapshotName: "field",
		},
		// Tests using Clean.cs file
		{
			name:         "Variable",
			file:         "Clean.cs",
			line:         31,
			column:       23,
			expectedText: "TestVariable",
			snapshotName: "variable",
		},
		{
			name:         "Function",
			file:         "Clean.cs",
			line:         34,
			column:       24,
			expectedText: "TestFunction",
			snapshotName: "function",
		},
		// Test for a location without hover info (comment line)
		{
			name:           "NoHoverInfo",
			file:           "Types.cs",
			line:           2, // blank line
			column:         1,
			unexpectedText: "class",
			snapshotName:   "no-hover-info",
		},
		// Test for a location outside the file
		{
			name:           "OutsideFile",
			file:           "Types.cs",
			line:           1000, // Line number beyond file length
			column:         1,
			unexpectedText: "class",
			snapshotName:   "outside-file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get a test suite
			suite := internal.GetTestSuite(t)

			ctx, cancel := context.WithTimeout(suite.Context, 15*time.Second)
			defer cancel()

			// Open all files and wait for csharp-ls to load the solution
			openAllFilesAndWait(suite, ctx)

			filePath := filepath.Join(suite.WorkspaceDir, tt.file)
			err := suite.Client.OpenFile(ctx, filePath)
			if err != nil {
				t.Fatalf("Failed to open %s: %v", tt.file, err)
			}

			// Get hover info
			result, err := tools.GetHoverInfo(ctx, suite.Client, filePath, tt.line, tt.column)
			if err != nil {
				// For the "OutsideFile" test, we expect an error
				if tt.name == "OutsideFile" {
					// Create a snapshot even for error case
					common.SnapshotTest(t, "csharp", "hover", tt.snapshotName, err.Error())
					return
				}
				t.Fatalf("GetHoverInfo failed: %v", err)
			}

			// Verify expected content
			if tt.expectedText != "" && !strings.Contains(result, tt.expectedText) {
				t.Errorf("Expected hover info to contain %q but got: %s", tt.expectedText, result)
			}

			// Verify unexpected content is absent
			if tt.unexpectedText != "" && strings.Contains(result, tt.unexpectedText) {
				t.Errorf("Expected hover info NOT to contain %q but it was found: %s", tt.unexpectedText, result)
			}

			common.SnapshotTest(t, "csharp", "hover", tt.snapshotName, result)
		})
	}
}
