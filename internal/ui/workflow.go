package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
)

// spinnerFrames defines the spinner animation frames.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// TaskStatus represents the status of a task.
type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskRunning
	TaskDone
	TaskFailed
	TaskSkipped
)

// Task represents a single task in the workflow.
type Task struct {
	Name    string
	Status  TaskStatus
	Message string
	Details string // Additional details shown when complete
}

// Workflow manages a list of tasks with visual progress.
type Workflow struct {
	writer     io.Writer
	tasks      []*Task
	mu         sync.Mutex
	spinnerIdx int
	stopChan   chan struct{}
	running    bool
	lastRender string
}

// NewWorkflow creates a new workflow tracker.
func NewWorkflow(w io.Writer) *Workflow {
	return &Workflow{
		writer:   w,
		tasks:    make([]*Task, 0),
		stopChan: make(chan struct{}),
	}
}

// AddTask adds a new task to the workflow.
func (wf *Workflow) AddTask(name string) int {
	wf.mu.Lock()
	defer wf.mu.Unlock()

	task := &Task{
		Name:   name,
		Status: TaskPending,
	}
	wf.tasks = append(wf.tasks, task)
	return len(wf.tasks) - 1
}

// StartTask marks a task as running.
func (wf *Workflow) StartTask(idx int, message string) {
	wf.mu.Lock()
	defer wf.mu.Unlock()

	if idx >= 0 && idx < len(wf.tasks) {
		wf.tasks[idx].Status = TaskRunning
		wf.tasks[idx].Message = message
	}
}

// CompleteTask marks a task as done.
func (wf *Workflow) CompleteTask(idx int, details string) {
	wf.mu.Lock()
	defer wf.mu.Unlock()

	if idx >= 0 && idx < len(wf.tasks) {
		wf.tasks[idx].Status = TaskDone
		wf.tasks[idx].Details = details
	}
}

// FailTask marks a task as failed.
func (wf *Workflow) FailTask(idx int, errMsg string) {
	wf.mu.Lock()
	defer wf.mu.Unlock()

	if idx >= 0 && idx < len(wf.tasks) {
		wf.tasks[idx].Status = TaskFailed
		wf.tasks[idx].Message = errMsg
	}
}

// SkipTask marks a task as skipped.
func (wf *Workflow) SkipTask(idx int, reason string) {
	wf.mu.Lock()
	defer wf.mu.Unlock()

	if idx >= 0 && idx < len(wf.tasks) {
		wf.tasks[idx].Status = TaskSkipped
		wf.tasks[idx].Message = reason
	}
}

// UpdateMessage updates the message of the current running task.
func (wf *Workflow) UpdateMessage(idx int, message string) {
	wf.mu.Lock()
	defer wf.mu.Unlock()

	if idx >= 0 && idx < len(wf.tasks) {
		wf.tasks[idx].Message = message
	}
}

// Start begins the workflow display with animation.
func (wf *Workflow) Start() {
	wf.mu.Lock()
	if wf.running {
		wf.mu.Unlock()
		return
	}
	wf.running = true
	wf.mu.Unlock()

	// Start spinner animation.
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-wf.stopChan:
				return
			case <-ticker.C:
				wf.mu.Lock()
				wf.spinnerIdx = (wf.spinnerIdx + 1) % len(spinnerFrames)
				wf.mu.Unlock()
				wf.render(false)
			}
		}
	}()
}

// Stop ends the workflow display.
func (wf *Workflow) Stop() {
	wf.mu.Lock()
	if !wf.running {
		wf.mu.Unlock()
		return
	}
	wf.running = false
	wf.mu.Unlock()

	close(wf.stopChan)
	wf.render(true)
}

// render displays the current state; final renders the end state without
// spinner, showing task details/errors instead of live messages.
func (wf *Workflow) render(final bool) {
	wf.mu.Lock()
	defer wf.mu.Unlock()

	var b strings.Builder

	// Clear previous output (move cursor up and clear lines).
	if wf.lastRender != "" {
		lineCount := strings.Count(wf.lastRender, "\n") + 1
		for i := 0; i < lineCount; i++ {
			b.WriteString("\033[A\033[K") // Move up and clear line
		}
	}

	// Render tasks.
	for _, task := range wf.tasks {
		b.WriteString(wf.renderTask(task, final))
		b.WriteString("\n")
	}

	output := b.String()
	wf.lastRender = strings.TrimSuffix(output, "\n")
	fmt.Fprint(wf.writer, output)
}

func (wf *Workflow) renderTask(task *Task, final bool) string {
	var icon string
	var nameStyle lipgloss.Style
	var msgStyle lipgloss.Style

	switch {
	case task.Status == TaskRunning && !final:
		icon = Secondary.Render(spinnerFrames[wf.spinnerIdx])
		nameStyle = StepRunning
		msgStyle = Secondary
	case task.Status == TaskDone:
		icon = GetCheckMark()
		nameStyle = StepComplete
		msgStyle = Dim
	case task.Status == TaskFailed:
		icon = GetCrossMark()
		nameStyle = StepFailed
		msgStyle = Error
	case task.Status == TaskSkipped:
		icon = Warning.Render("⊘")
		nameStyle = StepSkipped
		msgStyle = Warning
	default: // pending, or still running at final render
		icon = Muted.Render("○")
		nameStyle = StepPending
		msgStyle = Dim
	}

	line := fmt.Sprintf("%s %s", icon, nameStyle.Render(task.Name))

	msg := task.Message
	if final {
		// Show details for completed tasks, the message for failed/skipped ones.
		switch task.Status {
		case TaskDone:
			msg = task.Details
		case TaskFailed, TaskSkipped:
		default:
			msg = ""
		}
		if msg != "" {
			msg = "→ " + msg
		}
	}
	if msg != "" {
		line += " " + msgStyle.Render(msg)
	}

	return line
}
