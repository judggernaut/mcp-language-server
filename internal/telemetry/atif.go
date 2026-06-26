// Package telemetry records MCP tool invocations against this server.
//
// Each tool call is captured as a step in an ATIF (Agent Trajectory Interchange
// Format) trajectory. See:
// https://github.com/harbor-framework/harbor/blob/main/rfcs/0001-trajectory-format.md
//
// ATIF is designed for full agent trajectories that also carry LLM reasoning
// and token/cost metrics. This server only observes the tool calls made against
// it, so we populate the tool_calls and observation portions of each step and
// leave the LLM metrics empty (explained in the trajectory notes).
package telemetry

// SchemaVersion is the ATIF schema version this package emits.
const SchemaVersion = "ATIF-v1.7"

// Trajectory is the root ATIF document.
type Trajectory struct {
	SchemaVersion string         `json:"schema_version"`
	SessionID     string         `json:"session_id,omitempty"`
	TrajectoryID  string         `json:"trajectory_id,omitempty"`
	Agent         Agent          `json:"agent"`
	Steps         []Step         `json:"steps"`
	Notes         string         `json:"notes,omitempty"`
	FinalMetrics  *FinalMetrics  `json:"final_metrics,omitempty"`
	Extra         map[string]any `json:"extra,omitempty"`
}

// Agent identifies the system that produced the trajectory.
type Agent struct {
	Name    string         `json:"name"`
	Version string         `json:"version"`
	Extra   map[string]any `json:"extra,omitempty"`
}

// FinalMetrics holds aggregate counts for the trajectory.
type FinalMetrics struct {
	TotalSteps int            `json:"total_steps"`
	Extra      map[string]any `json:"extra,omitempty"`
}

// Step is a single interaction turn. For this server every recorded step has
// source "agent" (the calling agent invoked a tool) and carries one tool call
// plus its observation.
type Step struct {
	StepID      int          `json:"step_id"`
	Timestamp   string       `json:"timestamp,omitempty"`
	Source      string       `json:"source"`
	Message     string       `json:"message"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	Observation *Observation `json:"observation,omitempty"`
}

// ToolCall describes a single tool invocation.
type ToolCall struct {
	ToolCallID   string         `json:"tool_call_id"`
	FunctionName string         `json:"function_name"`
	Arguments    map[string]any `json:"arguments"`
	Extra        map[string]any `json:"extra,omitempty"`
}

// Observation holds the results produced by a step's tool calls.
type Observation struct {
	Results []ObservationResult `json:"results"`
}

// ObservationResult is the outcome of a single tool call.
type ObservationResult struct {
	SourceCallID string         `json:"source_call_id,omitempty"`
	Content      string         `json:"content,omitempty"`
	Extra        map[string]any `json:"extra,omitempty"`
}
