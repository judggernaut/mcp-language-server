package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readTrajectory(t *testing.T, path string) Trajectory {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read trajectory file: %v", err)
	}
	var traj Trajectory
	if err := json.Unmarshal(data, &traj); err != nil {
		t.Fatalf("trajectory file is not valid JSON: %v", err)
	}
	return traj
}

func TestRecorderWritesATIF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traj.json")
	t.Setenv(EnvTrajectoryFile, path)

	rec := NewRecorder("v1.2.3")
	if !rec.FileEnabled() {
		t.Fatal("expected FileEnabled to be true when env var is set")
	}

	rec.Begin("1")
	rec.Record("1", "definition", map[string]any{"symbolName": "main.Foo"}, "func Foo() {}", false)
	rec.Begin("2")
	rec.Record("2", "edit_file", map[string]any{"filePath": "/x/y.go"}, "applied", true)
	rec.Close()

	traj := readTrajectory(t, path)
	if traj.SchemaVersion != SchemaVersion {
		t.Errorf("schema_version = %q, want %q", traj.SchemaVersion, SchemaVersion)
	}
	if traj.Agent.Name != "mcp-language-server" || traj.Agent.Version != "v1.2.3" {
		t.Errorf("unexpected agent: %+v", traj.Agent)
	}
	if len(traj.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(traj.Steps))
	}
	if traj.FinalMetrics == nil || traj.FinalMetrics.TotalSteps != 2 {
		t.Errorf("expected final_metrics.total_steps=2, got %+v", traj.FinalMetrics)
	}

	step := traj.Steps[0]
	if step.Source != "agent" || step.StepID != 1 {
		t.Errorf("unexpected step: %+v", step)
	}
	if len(step.ToolCalls) != 1 || step.ToolCalls[0].FunctionName != "definition" {
		t.Errorf("unexpected tool call: %+v", step.ToolCalls)
	}
	if step.ToolCalls[0].ToolCallID != "1" {
		t.Errorf("tool_call_id = %q, want 1", step.ToolCalls[0].ToolCallID)
	}
	if step.Observation == nil || len(step.Observation.Results) != 1 {
		t.Fatalf("expected one observation result, got %+v", step.Observation)
	}
	if step.Observation.Results[0].Content != "func Foo() {}" {
		t.Errorf("unexpected observation content: %q", step.Observation.Results[0].Content)
	}

	// Second step should be flagged as an error in observation extra.
	errStep := traj.Steps[1]
	if isErr, _ := errStep.Observation.Results[0].Extra["is_error"].(bool); !isErr {
		t.Errorf("expected second step to be an error, extra=%+v", errStep.Observation.Results[0].Extra)
	}
}

func TestRecorderTruncatesContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traj.json")
	t.Setenv(EnvTrajectoryFile, path)
	t.Setenv(EnvMaxContent, "10")

	rec := NewRecorder("v1")
	long := strings.Repeat("a", 500)
	rec.Begin("1")
	rec.Record("1", "edit_file", map[string]any{"newText": long}, long, false)
	rec.Close()

	traj := readTrajectory(t, path)
	gotArg, _ := traj.Steps[0].ToolCalls[0].Arguments["newText"].(string)
	if len(gotArg) >= len(long) {
		t.Errorf("expected argument to be truncated, got length %d", len(gotArg))
	}
	if !strings.Contains(gotArg, "truncated") {
		t.Errorf("expected truncation marker in argument, got %q", gotArg)
	}
	if !strings.Contains(traj.Steps[0].Observation.Results[0].Content, "truncated") {
		t.Errorf("expected observation content to be truncated")
	}
}

func TestRecorderWithoutFileDoesNotWrite(t *testing.T) {
	// Ensure no trajectory env is set.
	t.Setenv(EnvTrajectoryFile, "")

	rec := NewRecorder("v1")
	if rec.FileEnabled() {
		t.Fatal("expected FileEnabled to be false without env var")
	}
	// Should not panic and should be a no-op for file output.
	rec.Begin("1")
	rec.Record("1", "hover", map[string]any{"filePath": "x"}, "ok", false)
	rec.Close()
}

func TestNilRecorderIsSafe(t *testing.T) {
	var rec *Recorder
	rec.Begin("1")
	rec.Record("1", "hover", nil, "", false)
	rec.Close()
	if rec.FileEnabled() {
		t.Error("nil recorder should report FileEnabled() == false")
	}
}
