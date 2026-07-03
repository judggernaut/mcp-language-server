package diagnostics_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/csharp/internal"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestDiagnostics tests diagnostics functionality with the C# language server (csharp-ls)
func TestDiagnostics(t *testing.T) {
	// Helper function to open all files and wait for indexing
	openAllFilesAndWait := func(suite *common.TestSuite, ctx context.Context) {
		// Open all files to ensure csharp-ls loads the whole solution
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

	// Test with a clean file
	t.Run("CleanFile", func(t *testing.T) {
		// Get a test suite with clean code
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 60*time.Second)
		defer cancel()

		// Open all files and wait for csharp-ls to load the solution
		openAllFilesAndWait(suite, ctx)

		filePath := filepath.Join(suite.WorkspaceDir, "Clean.cs")
		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, filePath, 2, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed: %v", err)
		}

		// Verify we have no diagnostics
		if !strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected no diagnostics but got: %s", result)
		}

		common.SnapshotTest(t, "csharp", "diagnostics", "clean", result)
	})

	// Test with a file containing errors
	t.Run("FileWithError", func(t *testing.T) {
		// Get a test suite with code that contains errors
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 60*time.Second)
		defer cancel()

		// Open all files and wait for csharp-ls to load the solution
		openAllFilesAndWait(suite, ctx)

		filePath := filepath.Join(suite.WorkspaceDir, "Program.cs")
		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, filePath, 2, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed: %v", err)
		}

		// Verify we have diagnostics about unreachable code
		if strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected diagnostics but got none")
		}

		if !strings.Contains(result, "Unreachable code") {
			t.Errorf("Expected unreachable code warning but got: %s", result)
		}

		// The exact Roslyn diagnostic message format can shift between SDK
		// versions, similar to rust-analyzer's diagnostic snapshot. Skip the
		// snapshot to avoid brittle failures; the assertions above already
		// verify the important behavior.
		t.Skip("Flaky snapshot. If we have diagnostics then it's working, but the format changes often.")
		// common.SnapshotTest(t, "csharp", "diagnostics", "unreachable", result)
	})

	// Test file dependency: file A (Helper.cs) provides a function,
	// file B (Consumer.cs) uses it, then modify A to break B
	t.Run("FileDependency", func(t *testing.T) {
		// Get a test suite with clean code
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 60*time.Second)
		defer cancel()

		// Open all files and wait for csharp-ls to load the solution
		openAllFilesAndWait(suite, ctx)

		// Ensure the relevant paths are accessible
		helperPath := filepath.Join(suite.WorkspaceDir, "Helper.cs")
		consumerPath := filepath.Join(suite.WorkspaceDir, "Consumer.cs")

		// Get initial diagnostics for Consumer.cs
		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, consumerPath, 2, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed: %v", err)
		}

		// Should have no diagnostics initially
		if !strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected no diagnostics initially but got: %s", result)
		}

		// Now modify the helper function to cause an error in the consumer
		modifiedHelperContent := `namespace Workspace;

public static class Helper
{
    // HelperFunction now requires an int parameter
    public static string HelperFunction(int value)
    {
        return "hello world";
    }
}
`
		// Write the modified content to the file
		err = suite.WriteFile("Helper.cs", modifiedHelperContent)
		if err != nil {
			t.Fatalf("Failed to update Helper.cs: %v", err)
		}

		// Explicitly notify the LSP server about the change
		helperURI := fmt.Sprintf("file://%s", helperPath)

		// Notify the LSP server about the file change
		err = suite.Client.NotifyChange(ctx, helperPath)
		if err != nil {
			t.Fatalf("Failed to notify change to Helper.cs: %v", err)
		}

		// Also send a didChangeWatchedFiles notification for coverage
		// This simulates what the watcher would do
		fileChangeParams := protocol.DidChangeWatchedFilesParams{
			Changes: []protocol.FileEvent{
				{
					URI:  protocol.DocumentUri(helperURI),
					Type: protocol.FileChangeType(protocol.Changed),
				},
			},
		}

		err = suite.Client.DidChangeWatchedFiles(ctx, fileChangeParams)
		if err != nil {
			t.Fatalf("Failed to send DidChangeWatchedFiles: %v", err)
		}

		// Wait for LSP to process the change
		time.Sleep(6 * time.Second)

		// Force reopen the consumer file to ensure LSP reevaluates it
		err = suite.Client.CloseFile(ctx, consumerPath)
		if err != nil {
			t.Fatalf("Failed to close Consumer.cs: %v", err)
		}

		err = suite.Client.OpenFile(ctx, consumerPath)
		if err != nil {
			t.Fatalf("Failed to reopen Consumer.cs: %v", err)
		}

		// Wait for LSP to process the change
		time.Sleep(6 * time.Second)

		// Check diagnostics again on consumer file - should now have an error
		result, err = tools.GetDiagnosticsForFile(ctx, suite.Client, consumerPath, 2, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed after dependency change: %v", err)
		}

		// Should have diagnostics now
		if strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected diagnostics after dependency change but got none")
		}

		// Should contain an error about function arguments
		if !strings.Contains(result, "argument") {
			t.Errorf("Expected error about wrong arguments but got: %s", result)
		}

		common.SnapshotTest(t, "csharp", "diagnostics", "dependency", result)
	})
}
