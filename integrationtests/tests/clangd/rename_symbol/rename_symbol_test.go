package rename_symbol_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/clangd/internal"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestRenameSymbol tests the RenameSymbol functionality with the clangd language server
func TestRenameSymbol(t *testing.T) {
	// Helper function to open a file and wait for indexing. clangd indexes
	// the whole project (via compile_commands.json) once one file in it is
	// opened, so opening main.cpp alone is enough to trigger a full index.
	openAndWait := func(suite *common.TestSuite, ctx context.Context) {
		filePath := filepath.Join(suite.WorkspaceDir, "src/main.cpp")
		if err := suite.Client.OpenFile(ctx, filePath); err != nil {
			t.Logf("Note: Failed to open src/main.cpp: %v", err)
		}
		time.Sleep(10 * time.Second)
	}

	// Test with a successful rename of a symbol that exists
	t.Run("SuccessfulRename", func(t *testing.T) {
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 20*time.Second)
		defer cancel()

		openAndWait(suite, ctx)

		// Rename helperFunction at its definition in helper.cpp (line 7, column 6)
		helperPath := filepath.Join(suite.WorkspaceDir, "src/helper.cpp")
		result, err := tools.RenameSymbol(ctx, suite.Client, helperPath, 7, 6, "renamedHelperFunction")
		if err != nil {
			t.Fatalf("RenameSymbol failed: %v", err)
		}

		if !strings.Contains(result, "Successfully renamed symbol") {
			t.Errorf("Expected success message but got: %s", result)
		}

		if !strings.Contains(result, "occurrences") {
			t.Errorf("Expected multiple occurrences to be renamed but got: %s", result)
		}

		common.SnapshotTest(t, "clangd", "rename_symbol", "successful", result)

		// Verify the rename propagated to the declaration, definition and all call sites
		for _, f := range []string{"include/helper.hpp", "src/helper.cpp", "src/consumer.cpp", "src/main.cpp"} {
			content, err := suite.ReadFile(f)
			if err != nil {
				t.Fatalf("Failed to read %s: %v", f, err)
			}
			if !strings.Contains(content, "renamedHelperFunction") {
				t.Errorf("Expected to find renamed function 'renamedHelperFunction' in %s", f)
			}
			if strings.Contains(content, "helperFunction()") {
				t.Errorf("Expected old name 'helperFunction' to be gone from %s", f)
			}
		}
	})

	// Test with a symbol that doesn't exist
	t.Run("SymbolNotFound", func(t *testing.T) {
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 20*time.Second)
		defer cancel()

		openAndWait(suite, ctx)

		// Request to rename a symbol at a position where no symbol exists (in whitespace)
		typesPath := filepath.Join(suite.WorkspaceDir, "src/types.cpp")
		if err := suite.Client.OpenFile(ctx, typesPath); err != nil {
			t.Fatalf("Failed to open types.cpp: %v", err)
		}
		time.Sleep(1 * time.Second)

		result, err := tools.RenameSymbol(ctx, suite.Client, typesPath, 2, 1, "NewName")

		if err == nil {
			if !strings.Contains(result, "0 occurrences") {
				t.Errorf("Expected 0 occurrences or error for non-existent symbol, but got: %s", result)
			}
			common.SnapshotTest(t, "clangd", "rename_symbol", "not_found", result)
		} else {
			errorMessage := err.Error()
			if !strings.Contains(errorMessage, "failed to rename") &&
				!strings.Contains(errorMessage, "not found") &&
				!strings.Contains(errorMessage, "no symbol") &&
				!strings.Contains(errorMessage, "cannot rename") {
				t.Errorf("Expected error message about failed rename but got: %s", errorMessage)
			}
			common.SnapshotTest(t, "clangd", "rename_symbol", "not_found", errorMessage)
		}
	})
}
