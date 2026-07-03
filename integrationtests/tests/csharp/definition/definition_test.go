package definition_test

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

// TestReadDefinition tests the ReadDefinition tool with C# type definitions.
//
// csharp-ls only reports bare (unqualified) names in workspace/symbol results
// for type-level symbols (classes, interfaces, enums). Methods and fields are
// reported with their full signature as the name (e.g. "string Foo.Bar()"),
// which does not match the fork's exact-name lookup used by this tool. See
// integrationtests/tests/csharp/README.md for details. As a result, this test
// only covers class/interface/enum lookups, unlike the other language suites
// which also cover functions, methods, constants and variables.
func TestReadDefinition(t *testing.T) {
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

	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 15*time.Second)
	defer cancel()

	// Open all files and wait for csharp-ls to load the solution
	openAllFilesAndWait(suite, ctx)

	tests := []struct {
		name         string
		symbolName   string
		expectedText string
		snapshotName string
	}{
		{
			name:         "Class",
			symbolName:   "SharedClass",
			expectedText: "class SharedClass",
			snapshotName: "shared-class",
		},
		{
			name:         "Interface",
			symbolName:   "SharedInterface",
			expectedText: "interface SharedInterface",
			snapshotName: "shared-interface",
		},
		{
			name:         "Enum",
			symbolName:   "SharedType",
			expectedText: "enum SharedType",
			snapshotName: "shared-type",
		},
		{
			name:         "StaticClass",
			symbolName:   "Helper",
			expectedText: "class Helper",
			snapshotName: "helper",
		},
		{
			name:         "AnotherClass",
			symbolName:   "TestClass",
			expectedText: "class TestClass",
			snapshotName: "test-class",
		},
		{
			name:         "AnotherInterface",
			symbolName:   "TestInterface",
			expectedText: "interface TestInterface",
			snapshotName: "test-interface",
		},
		{
			name:         "ClassImplementingInterface",
			symbolName:   "CustomImplementor",
			expectedText: "class CustomImplementor",
			snapshotName: "custom-implementor",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Call the ReadDefinition tool
			result, err := tools.ReadDefinition(ctx, suite.Client, tc.symbolName)
			if err != nil {
				t.Fatalf("Failed to read definition: %v", err)
			}

			// Check that the result contains relevant information
			if !strings.Contains(result, tc.expectedText) {
				t.Errorf("Definition does not contain expected text: %s", tc.expectedText)
			}

			// Use snapshot testing to verify exact output
			common.SnapshotTest(t, "csharp", "definition", tc.snapshotName, result)
		})
	}
}
