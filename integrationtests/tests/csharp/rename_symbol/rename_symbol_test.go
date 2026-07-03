package rename_symbol_test

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

// TestRenameSymbol tests the RenameSymbol functionality with the C# language server (csharp-ls)
func TestRenameSymbol(t *testing.T) {
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

		// Give csharp-ls time to index the solution
		time.Sleep(3 * time.Second)
	}

	// Test with a successful rename of a symbol that exists
	t.Run("SuccessfulRename", func(t *testing.T) {
		// Get a test suite with clean code
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 15*time.Second)
		defer cancel()

		// Open all files and wait for csharp-ls to load the solution
		openAllFilesAndWait(suite, ctx)

		// Ensure the file is open
		typesPath := filepath.Join(suite.WorkspaceDir, "Types.cs")

		// Request to rename SharedConstant to UpdatedConstant at its definition
		// The constant is defined at line 40, column 25 of Types.cs
		result, err := tools.RenameSymbol(ctx, suite.Client, typesPath, 40, 25, "UpdatedConstant")
		if err != nil {
			t.Fatalf("RenameSymbol failed: %v", err)
		}

		// Verify the constant was renamed
		if !strings.Contains(result, "Successfully renamed symbol") {
			t.Errorf("Expected success message but got: %s", result)
		}

		// Verify it's mentioned that it renamed multiple occurrences
		if !strings.Contains(result, "occurrences") {
			t.Errorf("Expected multiple occurrences to be renamed but got: %s", result)
		}

		common.SnapshotTest(t, "csharp", "rename_symbol", "successful", result)

		// Verify that the rename worked by checking for the updated constant name in the file
		fileContent, err := suite.ReadFile("Types.cs")
		if err != nil {
			t.Fatalf("Failed to read Types.cs: %v", err)
		}

		if !strings.Contains(fileContent, "UpdatedConstant") {
			t.Errorf("Expected to find renamed constant 'UpdatedConstant' in Types.cs")
		}

		// Also check that it was renamed in the consumer file
		consumerContent, err := suite.ReadFile("Consumer.cs")
		if err != nil {
			t.Fatalf("Failed to read Consumer.cs: %v", err)
		}

		if !strings.Contains(consumerContent, "UpdatedConstant") {
			t.Errorf("Expected to find renamed constant 'UpdatedConstant' in Consumer.cs")
		}
	})

	// Test with a symbol that doesn't exist
	t.Run("SymbolNotFound", func(t *testing.T) {
		// Get a test suite with clean code
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 15*time.Second)
		defer cancel()

		// Open all files and wait for csharp-ls to load the solution
		openAllFilesAndWait(suite, ctx)

		// Create a simple file with known content first
		simpleContent := `namespace Workspace;

// A simple class for testing
public static class DummyClass
{
    public static void DummyFunction()
    {
        // This is a dummy function
    }
}
`
		err := suite.WriteFile("PositionTest.cs", simpleContent)
		if err != nil {
			t.Fatalf("Failed to create PositionTest.cs: %v", err)
		}

		testFilePath := filepath.Join(suite.WorkspaceDir, "PositionTest.cs")
		err = suite.Client.OpenFile(ctx, testFilePath)
		if err != nil {
			t.Fatalf("Failed to open PositionTest.cs: %v", err)
		}

		time.Sleep(1 * time.Second) // Give time for the file to be processed

		// Request to rename a symbol at a position where no symbol exists (in whitespace)
		result, err := tools.RenameSymbol(ctx, suite.Client, testFilePath, 2, 1, "NewName")

		// The language server might actually succeed with no rename operations
		// In this case, we check if it reports no occurrences
		if err == nil {
			// Check if result indicates nothing was renamed
			if !strings.Contains(result, "0 occurrences") {
				t.Errorf("Expected 0 occurrences or error for non-existent symbol, but got: %s", result)
			}
			common.SnapshotTest(t, "csharp", "rename_symbol", "not_found", result)
		} else {
			// If there was an error, check it and snapshot that instead
			errorMessage := err.Error()
			if !strings.Contains(errorMessage, "failed to rename") &&
				!strings.Contains(errorMessage, "not found") &&
				!strings.Contains(errorMessage, "cannot rename") {
				t.Errorf("Expected error message about failed rename but got: %s", errorMessage)
			}
			common.SnapshotTest(t, "csharp", "rename_symbol", "not_found", errorMessage)
		}
	})
}
