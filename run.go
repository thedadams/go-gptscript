package gptscript

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os/exec"
	"strconv"
	"sync"
)

var errAbortRun = errors.New("run aborted")

type Run struct {
	url, toolPath string
	tools         []ToolDef
	opts          Options
	state         RunState
	cancel        context.CancelCauseFunc
	err           error
	wait          func()
	basicCommand  bool

	runFrame          *RunFrame
	calls             map[string]CallFrame
	parentCallFrameID string
	callsLock         *sync.RWMutex
	output, errput    string
	events            chan Frame
	lock              *sync.Mutex
}

// Text returns the text output of the gptscript. It blocks until the output is ready.
func (r *Run) Text() (string, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.runFrame != nil {
		return r.runFrame.Output, r.Err()
	}

	return r.output, r.Err()
}

// Bytes returns the output of the gptscript in bytes. It blocks until the output is ready.
func (r *Run) Bytes() ([]byte, error) {
	out, err := r.Text()
	return []byte(out), err
}

// State returns the current state of the gptscript.
func (r *Run) State() RunState {
	if r.runFrame != nil {
		return r.runFrame.State
	}

	return r.state
}

// Err returns the error that caused the gptscript to fail, if any.
func (r *Run) Err() error {
	if r.err != nil {
		return fmt.Errorf("run encountered an error: %w with error output: %s", r.err, r.errput)
	}
	return nil
}

// Program returns the gptscript program for the run.
func (r *Run) Program() *Program {
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.runFrame == nil || r.runFrame.Program.EntryToolID == "" {
		return nil
	}
	p := r.runFrame.Program
	return &p
}

// RespondingTool returns the name of the tool that produced the output.
func (r *Run) RespondingTool() Tool {
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.runFrame == nil {
		return Tool{}
	}

	s, ok := r.runFrame.RawOutput["toolID"].(string)
	if !ok {
		return Tool{}
	}

	return r.runFrame.Program.ToolSet[s]
}

// Calls will return a flattened array of the calls for this run.
func (r *Run) Calls() map[string]CallFrame {
	r.callsLock.RLock()
	defer r.callsLock.RUnlock()
	return maps.Clone(r.calls)
}

// ParentCallFrame returns the CallFrame for the top-level or "parent" call. The boolean indicates whether there is a parent CallFrame.
func (r *Run) ParentCallFrame() (CallFrame, bool) {
	r.callsLock.RLock()
	defer r.callsLock.RUnlock()

	if r.parentCallFrameID == "" {
		return CallFrame{}, false
	}

	return r.calls[r.parentCallFrameID], true
}

// ErrorOutput returns the stderr output of the gptscript.
// Should only be called after Bytes or Text has returned an error.
func (r *Run) ErrorOutput() string {
	return r.errput
}

// Events returns a channel that streams the gptscript events as they occur as Frames.
func (r *Run) Events() <-chan Frame {
	return r.events
}

// Close will stop the gptscript run, if it is running.
func (r *Run) Close() error {
	// If the command was not started, then report error.
	if r.cancel == nil {
		return fmt.Errorf("run not started")
	}

	r.cancel(errAbortRun)
	if r.wait == nil {
		return nil
	}

	r.wait()
	if !errors.Is(r.err, errAbortRun) && !errors.Is(r.err, context.Canceled) && !errors.As(r.err, new(*exec.ExitError)) {
		return r.err
	}

	return nil
}

// RawOutput returns the raw output of the gptscript. Most users should use Text or Bytes instead.
func (r *Run) RawOutput() (map[string]any, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.runFrame != nil {
		return r.runFrame.RawOutput, r.Err()
	}
	return nil, r.Err()
}

// ChatState returns the current chat state of the Run.
func (r *Run) ChatState() string {
	cs, _ := r.runFrame.ChatState.(string)
	return cs
}

// NextChat will pass input and create the next run in a chat.
// The new Run will be returned.
func (r *Run) NextChat(ctx context.Context, input string) (*Run, error) {
	runState := r.State()
	if runState != Creating && runState != Continue && runState != Error {
		return nil, fmt.Errorf("run must be in creating, continue, or error state not %q", runState)
	}

	run := &Run{
		url:       r.url,
		state:     Creating,
		opts:      r.opts,
		lock:      new(sync.Mutex),
		callsLock: new(sync.RWMutex),
	}

	if r.runFrame != nil && r.tools == nil && r.toolPath == "" {
		run.tools = make([]ToolDef, 1, len(r.runFrame.Program.ToolSet))
		for id, tool := range r.runFrame.Program.ToolSet {
			if id == r.runFrame.Program.EntryToolID {
				run.tools[0] = tool.ToolDef
			} else {
				run.tools = append(run.tools, tool.ToolDef)
			}
		}
	} else {
		run.tools = r.tools
		run.toolPath = r.toolPath
	}

	var (
		previousRunID uint64
		threadID      uint64
	)
	run.opts.Input = input
	if r.runFrame != nil {
		if r.runFrame.ChatState != "" && runState != Error {
			// If the previous run errored, then don't update the chat state.
			// opts.ChatState will be the last chat state where an error did not occur.
			run.opts.ChatState = r.runFrame.ChatState
		}

		previousRunID, _ = strconv.ParseUint(r.runFrame.ID, 10, 64)
		threadID = r.runFrame.ThreadID
	}

	var (
		payload     any
		requestPath string
	)
	if len(run.tools) != 0 {
		requestPath = "evaluate"
		payload = requestPayload{
			ToolDefs:      run.tools,
			Input:         input,
			Options:       run.opts,
			ThreadID:      threadID,
			PreviousRunID: previousRunID,
		}
	} else if run.toolPath != "" {
		requestPath = "run"
		payload = requestPayload{
			File:          run.toolPath,
			Input:         input,
			Options:       run.opts,
			ThreadID:      threadID,
			PreviousRunID: previousRunID,
		}
	}

	return run, run.request(ctx, requestPath, payload)
}

func (r *Run) request(ctx context.Context, requestPath string, payload any) (err error) {
	var (
		req               *http.Request
		url               = fmt.Sprintf("%s/%s", r.url, requestPath)
		cancelCtx, cancel = context.WithCancelCause(ctx)
	)

	r.cancel = cancel
	defer func() {
		if err != nil {
			cancel(err)
		}
	}()

	if payload == nil {
		req, err = http.NewRequestWithContext(cancelCtx, http.MethodGet, url, nil)
	} else {
		var b []byte
		b, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}

		req, err = http.NewRequestWithContext(cancelCtx, http.MethodPost, url, bytes.NewReader(b))
	}
	if err != nil {
		r.setState(Error)
		r.err = fmt.Errorf("failed to create request: %w", err)
		return r.err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		r.setState(Error)
		r.err = fmt.Errorf("failed to make request: %w", err)
		return r.err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		r.setState(Error)
		r.err = fmt.Errorf("run encountered an error, unexpected status code: %d", resp.StatusCode)
	} else {
		r.setState(Continue)
	}

	r.events = make(chan Frame, 100)
	r.lock.Lock()

	r.wait = func() {
		<-cancelCtx.Done()
		currentRunState := r.State()
		if err := context.Cause(cancelCtx); !errors.Is(err, context.Canceled) && r.err == nil {
			currentRunState = Error
			r.err = err
		} else if currentRunState != Continue && currentRunState != Error {
			currentRunState = Finished
		}

		r.setState(currentRunState)
	}

	go func() {
		var (
			err  error
			frag []byte

			done = true
			buf  = make([]byte, 64*1024)
		)
		defer func() {
			resp.Body.Close()
			close(r.events)
			cancel(r.err)
			r.wait()
			r.lock.Unlock()
		}()

		r.callsLock.Lock()
		r.calls = make(map[string]CallFrame)
		r.callsLock.Unlock()

		for n := 0; n != 0 || err == nil; n, err = resp.Body.Read(buf) {
			for _, line := range bytes.Split(bytes.TrimSpace(append(frag, buf[:n]...)), []byte("\n\n")) {
				line = bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data: ")))
				if len(line) == 0 || bytes.Equal(line, []byte("[DONE]")) {
					frag = frag[:0]
					continue
				}

				// Is this a JSON object?
				var m map[string]any
				if err := json.Unmarshal(line, &m); err != nil {
					// If not, then wait until we get the rest of the output.
					frag = line[:]
					continue
				}

				frag = frag[:0]

				if out, ok := m["stdout"]; ok {
					switch out := out.(type) {
					case string:
						if unquoted, err := strconv.Unquote(out); err == nil {
							r.output = unquoted
						} else {
							r.output = out
						}
					case map[string]any:
						if r.basicCommand {
							b, err := json.Marshal(out)
							if err != nil {
								r.setState(Error)
								r.err = fmt.Errorf("failed to process basic command output: %w", err)
								return
							}

							r.output = string(b)
						}

						chatState, err := json.Marshal(out["state"])
						if err != nil {
							r.setState(Error)
							r.err = fmt.Errorf("failed to process chat state: %w", err)
						}
						if r.runFrame != nil {
							r.runFrame.ChatState = string(chatState)
						}

						if content, ok := out["content"].(string); ok {
							r.output = content
						}

						done, _ = out["done"].(bool)
					default:
						r.setState(Error)
						r.err = fmt.Errorf("failed to process stdout, invalid type: %T", out)
						return
					}
				} else if stderr, ok := m["stderr"]; ok {
					switch out := stderr.(type) {
					case string:
						if unquoted, err := strconv.Unquote(out); err == nil {
							r.errput = unquoted
						} else {
							r.errput = out
						}
					default:
						r.setState(Error)
						r.err = fmt.Errorf("failed to process stderr, invalid type: %T", out)
					}
				} else {
					var event Frame
					if err := json.Unmarshal(line, &event); err != nil {
						slog.Debug("failed to unmarshal event", "error", err, "event", string(line))
					}

					if event.Prompt != nil && !r.opts.Prompt {
						r.setState(Error)
						r.err = fmt.Errorf("prompt event occurred when prompt was not allowed: %s", event.Prompt)
						// Ignore the error because it is the same as the above error.
						_ = r.Close()

						return
					}

					if event.Call != nil {
						r.callsLock.Lock()
						r.calls[event.Call.ID] = *event.Call
						if r.parentCallFrameID == "" && event.Call.ParentID == "" {
							r.parentCallFrameID = event.Call.ID
						}
						r.callsLock.Unlock()
					} else if event.Run != nil {
						r.callsLock.Lock()
						r.runFrame = event.Run
						r.callsLock.Unlock()
						if event.Run.Type == EventTypeRunFinish && event.Run.Error != "" {
							r.setState(Error)
							r.err = fmt.Errorf("%s", event.Run.Error)
						}
					}

					if r.opts.IncludeEvents {
						r.events <- event
					}
				}
			}
		}

		if err != nil && !errors.Is(err, io.EOF) {
			slog.Debug("failed to read events from response", "error", err)
			r.err = fmt.Errorf("failed to read events: %w", err)
		}

		if r.err != nil {
			r.setState(Error)
		} else if done {
			r.setState(Finished)
		} else {
			r.setState(Continue)
		}
	}()

	return nil
}

func (r *Run) setState(s RunState) {
	r.state = s
	if r.runFrame != nil {
		r.runFrame.State = s
	}
}

type RunState string

func (rs RunState) IsTerminal() bool {
	return rs == Finished || rs == Error
}

const (
	Creating RunState = "creating"
	Running  RunState = "running"
	Continue RunState = "continue"
	Finished RunState = "finished"
	Error    RunState = "error"
)

type requestPayload struct {
	Options       `json:",inline"`
	File          string    `json:"file"`
	Input         string    `json:"input"`
	ToolDefs      []ToolDef `json:"toolDefs,inline"`
	ThreadID      uint64    `json:"threadID"`
	PreviousRunID uint64    `json:"previousRunID"`
}

type restoreRunResponse struct {
	RunFrame          *RunFrame            `json:"runFrame"`
	Calls             map[string]CallFrame `json:"calls"`
	ParentCallFrameID string               `json:"parentCallFrameID"`
}

type listRunsResponse struct {
	Items []restoreRunResponse `json:"items"`
}

func restoreRun(data, url string) (*Run, error) {
	restoredRun := new(restoreRunResponse)
	if err := json.Unmarshal([]byte(data), restoredRun); err != nil {
		return nil, err
	}

	return &Run{
		url:               url,
		runFrame:          restoredRun.RunFrame,
		calls:             restoredRun.Calls,
		parentCallFrameID: restoredRun.ParentCallFrameID,
	}, nil
}

func restoreRuns(data, url string) ([]Run, error) {
	restoredRuns := new(listRunsResponse)
	if err := json.Unmarshal([]byte(data), &restoredRuns); err != nil {
		return nil, err
	}

	runs := make([]Run, 0, len(restoredRuns.Items))
	for _, restoredRun := range restoredRuns.Items {
		runs = append(runs, Run{
			url:               url,
			runFrame:          restoredRun.RunFrame,
			calls:             restoredRun.Calls,
			parentCallFrameID: restoredRun.ParentCallFrameID,
		})
	}

	return runs, nil
}
