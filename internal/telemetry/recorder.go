package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/logging"
)

const (
	// EnvTrajectoryFile, when set, enables writing the full ATIF trajectory to
	// the given path. When unset, only lightweight summary logs are emitted.
	EnvTrajectoryFile = "MCP_TRAJECTORY_FILE"
	// EnvMaxContent caps the number of characters kept for any single argument
	// or result value written to the trajectory file.
	EnvMaxContent = "MCP_TRAJECTORY_MAX_CONTENT"
	// EnvMaxSteps caps how many steps are retained in memory / written to the
	// trajectory file to bound memory on long-running servers.
	EnvMaxSteps = "MCP_TRAJECTORY_MAX_STEPS"

	defaultMaxContent = 4096
	defaultMaxSteps   = 10000
)

var telemetryLogger = logging.NewLogger(logging.Tools)

// Recorder captures MCP tool invocations as an ATIF trajectory. It is safe for
// concurrent use.
type Recorder struct {
	mu sync.Mutex

	filePath   string
	maxContent int
	maxSteps   int

	traj        *Trajectory
	totalSteps  int
	droppedSeen bool
	inFlight    map[string]time.Time
}

// NewRecorder builds a Recorder configured from the environment. agentVersion
// identifies this server build in the trajectory.
func NewRecorder(agentVersion string) *Recorder {
	maxContent := defaultMaxContent
	if v := os.Getenv(EnvMaxContent); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			maxContent = n
		}
	}
	maxSteps := defaultMaxSteps
	if v := os.Getenv(EnvMaxSteps); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxSteps = n
		}
	}

	r := &Recorder{
		filePath:   os.Getenv(EnvTrajectoryFile),
		maxContent: maxContent,
		maxSteps:   maxSteps,
		inFlight:   make(map[string]time.Time),
		traj: &Trajectory{
			SchemaVersion: SchemaVersion,
			SessionID:     newID(),
			TrajectoryID:  newID(),
			Agent: Agent{
				Name:    "mcp-language-server",
				Version: agentVersion,
			},
			Steps: []Step{},
			Notes: "Each step is one MCP tool call observed by mcp-language-server. " +
				"LLM-side fields (model_name, reasoning, token/cost metrics) are not " +
				"available to this server and are intentionally omitted.",
		},
	}
	return r
}

// FileEnabled reports whether the full trajectory is being written to disk.
func (r *Recorder) FileEnabled() bool {
	return r != nil && r.filePath != ""
}

// Begin marks the start of a tool call so its duration can be measured.
func (r *Recorder) Begin(callID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.inFlight[callID] = time.Now()
	r.mu.Unlock()
}

// Record finalizes a tool call: it emits an always-on summary log and, when a
// trajectory file is configured, appends an ATIF step and rewrites the file.
func (r *Recorder) Record(callID, tool string, args map[string]any, resultContent string, isError bool) {
	if r == nil {
		return
	}

	r.mu.Lock()
	start, ok := r.inFlight[callID]
	if ok {
		delete(r.inFlight, callID)
	}
	durationMs := int64(0)
	if ok {
		durationMs = time.Since(start).Milliseconds()
	}

	r.totalSteps++

	// Always-on summary log: tool, timing and sizes only, no payload content.
	telemetryLogger.Info("tool call: name=%s ok=%t duration_ms=%d args_bytes=%d result_bytes=%d",
		tool, !isError, durationMs, approxSize(args), len(resultContent))

	if r.filePath == "" {
		r.mu.Unlock()
		return
	}

	if len(r.traj.Steps) < r.maxSteps {
		step := Step{
			StepID:    r.totalSteps,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Source:    "agent",
			Message:   "",
			ToolCalls: []ToolCall{{
				ToolCallID:   callID,
				FunctionName: tool,
				Arguments:    r.truncateArgs(args),
				Extra:        map[string]any{"duration_ms": durationMs},
			}},
			Observation: &Observation{
				Results: []ObservationResult{{
					SourceCallID: callID,
					Content:      truncate(resultContent, r.maxContent),
					Extra:        map[string]any{"is_error": isError, "duration_ms": durationMs},
				}},
			},
		}
		r.traj.Steps = append(r.traj.Steps, step)
	} else if !r.droppedSeen {
		r.droppedSeen = true
		telemetryLogger.Warn("trajectory step cap (%d) reached; further steps are counted but not written", r.maxSteps)
	}

	trajCopy := r.snapshotLocked()
	r.mu.Unlock()

	if err := writeTrajectory(r.filePath, trajCopy); err != nil {
		telemetryLogger.Error("failed to write trajectory file %s: %v", r.filePath, err)
	}
}

// Close performs a final flush of the trajectory file.
func (r *Recorder) Close() {
	if r == nil || r.filePath == "" {
		return
	}
	r.mu.Lock()
	trajCopy := r.snapshotLocked()
	r.mu.Unlock()
	if err := writeTrajectory(r.filePath, trajCopy); err != nil {
		telemetryLogger.Error("failed to write trajectory file %s: %v", r.filePath, err)
	}
}

// snapshotLocked returns a shallow copy of the trajectory with final metrics
// populated. Callers must hold r.mu.
func (r *Recorder) snapshotLocked() Trajectory {
	t := *r.traj
	t.FinalMetrics = &FinalMetrics{TotalSteps: r.totalSteps}
	return t
}

func writeTrajectory(path string, traj Trajectory) error {
	data, err := json.MarshalIndent(traj, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal trajectory: %w", err)
	}
	// Write atomically so a concurrent reader never sees a partial document.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// truncateArgs returns a deep copy of args with long string values truncated.
func (r *Recorder) truncateArgs(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(args))
	for k, v := range args {
		out[k] = r.truncateValue(v)
	}
	return out
}

func (r *Recorder) truncateValue(v any) any {
	switch val := v.(type) {
	case string:
		return truncate(val, r.maxContent)
	case map[string]any:
		return r.truncateArgs(val)
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = r.truncateValue(item)
		}
		return out
	default:
		return val
	}
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + fmt.Sprintf("…[truncated %d bytes]", len(s)-max)
}

// approxSize returns a rough byte size of the arguments for summary logging.
func approxSize(args map[string]any) int {
	if len(args) == 0 {
		return 0
	}
	data, err := json.Marshal(args)
	if err != nil {
		return 0
	}
	return len(data)
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}

// DefaultTrajectoryPath is a convenience for resolving a relative trajectory
// path against a base directory; unused by default but handy for callers.
func DefaultTrajectoryPath(baseDir, name string) string {
	return filepath.Join(baseDir, name)
}
