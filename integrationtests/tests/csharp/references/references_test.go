package references_test

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

// TestFindReferences tests the FindReferences tool with C# symbols that have
// references across different files.
//
// As with the definition tool, symbol lookup here goes through
// workspace/symbol, which csharp-ls only reports bare names for at the
// class/interface/enum level. See integrationtests/tests/csharp/README.md.
func TestFindReferences(t *testing.T) {
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
		name          string
		symbolName    string
		expectedText  string
		expectedFiles int // Number of files where references should be found
		snapshotName  string
	}{
		{
			name:          "Class with references across files",
			symbolName:    "SharedClass",
			expectedText:  "SharedClass",
			expectedFiles: 2, // Consumer.cs and AnotherConsumer.cs
			snapshotName:  "shared-class",
		},
		{
			name:          "Interface with references across files",
			symbolName:    "SharedInterface",
			expectedText:  "iface",
			expectedFiles: 2, // Consumer.cs and AnotherConsumer.cs
			snapshotName:  "shared-interface",
		},
		{
			name:          "Enum with references across files",
			symbolName:    "SharedType",
			expectedText:  "SharedType",
			expectedFiles: 2, // Consumer.cs and AnotherConsumer.cs
			snapshotName:  "shared-type",
		},
		{
			name:          "Static class with references across files",
			symbolName:    "Helper",
			expectedText:  "HelperFunction",
			expectedFiles: 2, // Consumer.cs and AnotherConsumer.cs
			snapshotName:  "helper",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Call the FindReferences tool
			result, err := tools.FindReferences(ctx, suite.Client, tc.symbolName)
			if err != nil {
				t.Fatalf("Failed to find references: %v", err)
			}

			// Check that the result contains relevant information
			if !strings.Contains(result, tc.expectedText) {
				t.Errorf("References do not contain expected text: %s", tc.expectedText)
			}

			// Count how many different files are mentioned in the result
			fileCount := countFilesInResult(result)
			if fileCount < tc.expectedFiles {
				t.Errorf("Expected references in at least %d files, but found in %d files",
					tc.expectedFiles, fileCount)
			}

			// Use snapshot testing to verify exact output
			common.SnapshotTest(t, "csharp", "references", tc.snapshotName, result)
		})
	}
}

// countFilesInResult counts the number of unique files mentioned in the result
func countFilesInResult(result string) int {
	fileMap := make(map[string]bool)

	// Any line containing "workspace" and ".cs" is a file path
	for line := range strings.SplitSeq(result, "\n") {
		if strings.Contains(line, "workspace") && strings.Contains(line, ".cs") {
			fileMap[line] = true
		}
	}

	return len(fileMap)
}
