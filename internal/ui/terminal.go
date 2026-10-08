package ui

import (
	"bytes"
	"errors"
	"io"
	"os"
	"sync"

	"charm.land/huh/v2"
	"github.com/charmbracelet/x/term"
	"github.com/idlab-discover/aibomgen-cli/internal/apperr"
)

// NoInput disables all prompts (set from --no-input).
var NoInput bool

// ErrNoInput is returned when a prompt is needed but cannot be shown.
var ErrNoInput = errors.New("cannot prompt: not running in a terminal (or --no-input set)")

// IsTTY reports whether f is a terminal.
func IsTTY(f *os.File) bool { return term.IsTerminal(f.Fd()) }

// CanPrompt reports whether interactive prompts can be shown: stdin and stderr
// (where prompts render) must be terminals and --no-input must be off.
func CanPrompt() bool { return !NoInput && IsTTY(os.Stdin) && IsTTY(os.Stderr) }

// logGate is the log destination. While held, writes are buffered so they don't
// corrupt a TUI that owns the terminal; release flushes them in order.
type logGate struct {
	mu   sync.Mutex
	out  io.Writer
	held bool
	buf  bytes.Buffer
}

func (g *logGate) Write(p []byte) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.held {
		return g.buf.Write(p)
	}
	return g.out.Write(p)
}

func (g *logGate) hold() (release func()) {
	g.mu.Lock()
	g.held = true
	g.mu.Unlock()
	return func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		g.held = false
		_, _ = g.buf.WriteTo(g.out)
	}
}

var logs = &logGate{out: os.Stderr}

// LogWriter returns the writer logs should go to (stderr, gated by HoldLogs).
func LogWriter() io.Writer { return logs }

// HoldLogs buffers log output until the returned release func is called.
func HoldLogs() (release func()) { return logs.hold() }

// RunForm runs a huh form with logs held. It returns ErrNoInput when prompting
// is not possible and apperr.ErrCancelled when the user aborts.
func RunForm(f *huh.Form) error {
	if !CanPrompt() {
		return ErrNoInput
	}
	defer HoldLogs()()
	if err := f.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return apperr.ErrCancelled
		}
		return err
	}
	return nil
}
