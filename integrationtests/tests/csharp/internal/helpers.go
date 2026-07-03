// Package internal contains shared helpers for C# tests
package internal

import (
	"path/filepath"
	"testing"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
)

// GetTestSuite returns a test suite for C# language server tests
func GetTestSuite(t *testing.T) *common.TestSuite {
	// Configure C# LSP (csharp-ls)
	repoRoot, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}

	config := common.LSPTestConfig{
		Name:             "csharp",
		Command:          "csharp-ls",
		Args:             []string{},
		WorkspaceDir:     filepath.Join(repoRoot, "integrationtests/workspaces/csharp"),
		InitializeTimeMs: 4000,
	}

	// Create a test suite
	suite := common.NewTestSuite(t, config)

	// Set up the suite
	err = suite.Setup()
	if err != nil {
		t.Fatalf("Failed to set up test suite: %v", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		suite.Cleanup()
	})

	return suite
}
