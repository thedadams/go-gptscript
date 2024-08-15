package gptscript

import (
	"context"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestRestartingErrorRun(t *testing.T) {
	instructions := "#!/bin/bash\nexit ${EXIT_CODE}"
	if runtime.GOOS == "windows" {
		instructions = "#!/usr/bin/env powershell.exe\n\n$e = $env:EXIT_CODE;\nif ($e) { Exit 1; }"
	}
	tool := ToolDef{
		Context:      []string{"my-context"},
		Instructions: "Say hello",
	}
	contextTool := ToolDef{
		Name:         "my-context",
		Instructions: instructions,
	}

	run, err := g.Evaluate(context.Background(), Options{GlobalOptions: GlobalOptions{Env: []string{"EXIT_CODE=1"}}, IncludeEvents: true}, tool, contextTool)
	if err != nil {
		t.Errorf("Error executing tool: %v", err)
	}

	// Wait for the run to complete
	_, err = run.Text()
	if err == nil {
		t.Fatalf("no error returned from run")
	}

	run.opts.Env = nil
	run, err = run.NextChat(context.Background(), "")
	if err != nil {
		t.Errorf("Error executing next run: %v", err)
	}

	_, err = run.Text()
	if err != nil {
		t.Errorf("executing run with input of 0 should not fail: %v", err)
	}
}

func TestListRuns(t *testing.T) {
	runs, err := g.ListRuns(context.Background(), 1)
	if err != nil {
		t.Errorf("Error listing runs: %v", err)
	}

	if len(runs) == 0 {
		t.Error("No runs found")
	}

	for _, run := range runs {
		if run.runFrame == nil || run.runFrame.ID == "" || run.runFrame.ThreadID != 1 {
			t.Errorf("Invalid run: %v", run)
		}
	}
}

func TestRestoreRun(t *testing.T) {
	tool := ToolDef{
		Instructions: "Say hello to the user and wait for further instructions.",
		Chat:         true,
	}
	run, err := g.Evaluate(context.Background(), Options{}, tool)
	if err != nil {
		t.Errorf("Error executing tool: %v", err)
	}

	// Wait for the run to complete
	_, err = run.Text()
	if err != nil {
		t.Errorf("Error reading output: %v", err)
	}

	if err = run.Err(); err != nil {
		t.Errorf("Run had an unexpected error: %v", err)
	}

	restoredRun, err := g.RestoreRun(context.Background(), run.runFrame.ThreadID, run.runFrame.ID)
	if err != nil || restoredRun.runFrame == nil {
		t.Fatalf("Error restoring run: %v", err)
	}

	// The type is not stored in the server, so set it here so we can compare.
	restoredRun.runFrame.Type = run.runFrame.Type

	if !reflect.DeepEqual(run.runFrame, restoredRun.runFrame) {
		t.Errorf("Restored run did not match original run")
	}

	if !reflect.DeepEqual(run.calls, restoredRun.calls) {
		t.Errorf("Restored run did not match original run")
	}

	if run.parentCallFrameID != restoredRun.parentCallFrameID {
		t.Errorf("Parent call frame ID did not match, expected %q, got %q", run.parentCallFrameID, restoredRun.parentCallFrameID)
	}
}

func TestChatWithRestoredRun(t *testing.T) {
	tool := ToolDef{
		Instructions: "Say hello to the user and wait for further instructions.",
		Chat:         true,
	}
	run, err := g.Evaluate(context.Background(), Options{DisableCache: true}, tool)
	if err != nil {
		t.Errorf("Error executing tool: %v", err)
	}

	// Wait for the run to complete
	_, err = run.Text()
	if err != nil {
		t.Errorf("Error reading output: %v", err)
	}

	if err = run.Err(); err != nil {
		t.Errorf("Run had an unexpected error: %v", err)
	}

	run, err = run.NextChat(context.Background(), "What is today's date?")
	if err != nil {
		t.Errorf("Error executing next run: %v", err)
	}

	_, err = run.Text()
	if err != nil {
		t.Fatalf("Error reading output: %v", err)
	}

	restoredRun, err := g.RestoreRun(context.Background(), run.runFrame.ThreadID, run.runFrame.ID)
	if err != nil {
		t.Fatalf("Error restoring run: %v", err)
	}

	// I should be able to continue chatting with the restored run.
	run, err = restoredRun.NextChat(context.Background(), "What was my last question?")
	if err != nil {
		t.Errorf("Error executing next run: %v", err)
	}

	out, err := run.Text()
	if err != nil {
		t.Fatalf("Error reading output: %v", err)
	}

	if !strings.Contains(out, "What is today's date?") {
		t.Errorf("Unexpected output: %s", out)
	}
}
