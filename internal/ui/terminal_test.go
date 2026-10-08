package ui

import (
	"bytes"
	"errors"
	"testing"

	"charm.land/huh/v2"
)

func TestLogGateHoldsAndFlushesInOrder(t *testing.T) {
	var out bytes.Buffer
	g := &logGate{out: &out}
	write := func(s string) {
		if _, err := g.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}

	write("a\n")
	release := g.hold()
	write("b\n")
	write("c\n")
	if out.String() != "a\n" {
		t.Fatalf("held writes leaked: %q", out.String())
	}
	release()
	write("d\n")
	if out.String() != "a\nb\nc\nd\n" {
		t.Fatalf("got %q", out.String())
	}
}

func TestRunFormNoInput(t *testing.T) {
	NoInput = true
	t.Cleanup(func() { NoInput = false })
	if err := RunForm(huh.NewForm(huh.NewGroup(huh.NewConfirm()))); !errors.Is(err, ErrNoInput) {
		t.Fatalf("got %v, want ErrNoInput", err)
	}
}

func TestStaticWorkflowPrintsEachStateOnce(t *testing.T) {
	var out bytes.Buffer
	wf := NewWorkflow(&out)
	wf.Static = true
	i := wf.AddTask("Fetch")
	wf.Start()
	wf.StartTask(i, "")
	if !bytes.Contains(out.Bytes(), []byte("Fetch")) {
		t.Fatalf("start not printed: %q", out.String())
	}
	wf.CompleteTask(i, "done")
	wf.Stop()
	if n := bytes.Count(out.Bytes(), []byte("Fetch")); n != 2 {
		t.Fatalf("want 2 lines (start, done), got %d: %q", n, out.String())
	}
	if bytes.Contains(out.Bytes(), []byte("\033[A")) {
		t.Fatalf("static output has cursor escapes: %q", out.String())
	}
}
