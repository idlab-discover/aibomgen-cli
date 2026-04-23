package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// StepStatus represents the status of a step.
type StepStatus int

const (
	StatusPending StepStatus = iota
	StatusRunning
	StatusComplete
	StatusFailed
	StatusSkipped
)

// Step represents a single step in the progress.
type Step struct {
	Name    string
	Status  StepStatus
	Message string // Optional message (e.g., error details)
}

// ProgressModel is the Bubble Tea model for progress display.
type ProgressModel struct {
	spinner    spinner.Model
	steps      []Step
	current    int
	title      string
	done       bool
	err        error
	width      int
	quitting   bool
	showSteps  bool
	subMessage string // Additional message below spinner
}

// ProgressOption is a function that configures the progress model.
type ProgressOption func(*ProgressModel)

// WithTitle sets the title for the progress display.
func WithTitle(title string) ProgressOption {
	return func(m *ProgressModel) {
		m.title = title
	}
}

// WithSteps initializes the progress with predefined steps.
func WithSteps(steps []string) ProgressOption {
	return func(m *ProgressModel) {
		m.steps = make([]Step, len(steps))
		for i, name := range steps {
			m.steps[i] = Step{Name: name, Status: StatusPending}
		}
		m.showSteps = true
	}
}

// NewProgressModel creates a new progress model.
func NewProgressModel(opts ...ProgressOption) ProgressModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorSecondary)

	m := ProgressModel{
		spinner:   s,
		width:     80,
		showSteps: false,
	}

	for _, opt := range opts {
		opt(&m)
	}

	return m
}

// Init initializes the model.
func (m ProgressModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// ProgressMsg is sent to update progress.
type ProgressMsg struct {
	StepIndex  int
	Status     StepStatus
	Message    string
	SubMessage string
}

// DoneMsg signals that the operation is complete.
type DoneMsg struct {
	Err error
}

// Update handles messages.
func (m ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case ProgressMsg:
		if msg.StepIndex >= 0 && msg.StepIndex < len(m.steps) {
			m.steps[msg.StepIndex].Status = msg.Status
			m.steps[msg.StepIndex].Message = msg.Message
			m.current = msg.StepIndex
		}
		m.subMessage = msg.SubMessage
		return m, nil

	case DoneMsg:
		m.done = true
		m.err = msg.Err
		return m, tea.Quit
	}

	return m, nil
}

// View renders the progress display.
func (m ProgressModel) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}

	var b strings.Builder

	// Title.
	if m.title != "" {
		b.WriteString(Title.Render(m.title))
		b.WriteString("\n\n")
	}

	// Steps display.
	if m.showSteps && len(m.steps) > 0 {
		for i, step := range m.steps {
			var icon string
			var style styleWrapper

			switch step.Status {
			case StatusPending:
				icon = Muted.Render("○")
				style = StepPending
			case StatusRunning:
				icon = m.spinner.View()
				style = StepRunning
			case StatusComplete:
				icon = GetCheckMark()
				style = StepComplete
			case StatusFailed:
				icon = GetCrossMark()
				style = StepFailed
			case StatusSkipped:
				icon = Warning.Render("⊘")
				style = StepSkipped
			}

			line := fmt.Sprintf("%s %s", icon, style.Render(step.Name))
			b.WriteString(line)

			// Show message if present.
			if step.Message != "" && (step.Status == StatusFailed || step.Status == StatusComplete) {
				b.WriteString(Dim.Render(" → " + step.Message))
			}

			if i < len(m.steps)-1 {
				b.WriteString("\n")
			}
		}
	} else {
		// Simple spinner mode.
		b.WriteString(m.spinner.View())
		if m.subMessage != "" {
			b.WriteString(" " + m.subMessage)
		}
	}

	// Sub-message for steps mode.
	if m.showSteps && m.subMessage != "" {
		b.WriteString("\n\n")
		b.WriteString(Dim.Render(m.subMessage))
	}

	// Final status.
	if m.done {
		b.WriteString("\n\n")
		if m.err != nil {
			b.WriteString(ErrorBox.Render(GetCrossMark() + " " + m.err.Error()))
		} else {
			completed := 0
			for _, s := range m.steps {
				if s.Status == StatusComplete {
					completed++
				}
			}
			if len(m.steps) > 0 {
				b.WriteString(Success.Render(fmt.Sprintf("✓ Completed %d/%d steps", completed, len(m.steps))))
			}
		}
	}

	return tea.NewView(b.String())
}

// ProgressTracker provides a simple interface for tracking progress.
// without directly using Bubble Tea in the calling code.
type ProgressTracker struct {
	program    *tea.Program
	steps      []string
	mu         sync.Mutex
	running    bool
	useSpinner bool
}

// NewProgressTracker creates a new progress tracker.
func NewProgressTracker(title string, steps []string) *ProgressTracker {
	return &ProgressTracker{
		steps:      steps,
		useSpinner: true,
	}
}

// Start begins the progress display.
func (pt *ProgressTracker) Start() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.running {
		return
	}

	opts := []ProgressOption{
		WithTitle(""),
	}
	if len(pt.steps) > 0 {
		opts = append(opts, WithSteps(pt.steps))
	}

	model := NewProgressModel(opts...)
	pt.program = tea.NewProgram(model, tea.WithoutSignalHandler())
	pt.running = true

	go func() {
		_, _ = pt.program.Run()
	}()

	// Give the program a moment to start.
	time.Sleep(50 * time.Millisecond)
}

// UpdateStep updates a specific step's status.
func (pt *ProgressTracker) UpdateStep(index int, status StepStatus, message string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.program == nil || !pt.running {
		return
	}

	pt.program.Send(ProgressMsg{
		StepIndex: index,
		Status:    status,
		Message:   message,
	})
}

// SetMessage sets the sub-message displayed below the spinner.
func (pt *ProgressTracker) SetMessage(message string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.program == nil || !pt.running {
		return
	}

	pt.program.Send(ProgressMsg{
		StepIndex:  -1,
		SubMessage: message,
	})
}

// Complete marks the progress as complete.
func (pt *ProgressTracker) Complete(err error) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.program == nil || !pt.running {
		return
	}

	pt.program.Send(DoneMsg{Err: err})
	pt.running = false

	// Give it time to render the final state.
	time.Sleep(100 * time.Millisecond)
}

// Stop stops the progress display without marking complete.
func (pt *ProgressTracker) Stop() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.program == nil || !pt.running {
		return
	}

	pt.program.Quit()
	pt.running = false
	time.Sleep(50 * time.Millisecond)
}
