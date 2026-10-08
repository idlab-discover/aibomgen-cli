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

	g.Write([]byte("a\n"))
	release := g.hold()
	g.Write([]byte("b\n"))
	g.Write([]byte("c\n"))
	if out.String() != "a\n" {
		t.Fatalf("held writes leaked: %q", out.String())
	}
	release()
	g.Write([]byte("d\n"))
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
