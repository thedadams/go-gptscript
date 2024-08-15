package gptscript

import (
	"context"
	"testing"
)

func TestListThread(t *testing.T) {
	_, err := g.ListThreads(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	threads, err := g.ListThreads(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(threads) == 0 {
		t.Fatal("No threads found")
	}

	for _, thread := range threads {
		if thread.ID == 0 {
			t.Fatal("Thread ID is zero")
		}
	}
}

func TestGetThread(t *testing.T) {
	thread, err := g.GetThread(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	if thread == nil || thread.ID == 0 {
		t.Fatal("Thread not found")
	}
}
