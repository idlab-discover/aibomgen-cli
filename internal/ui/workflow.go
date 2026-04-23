package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
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
	writer      io.Writer
	tasks       []*Task
	mu          sync.Mutex
	spinnerIdx  int
	stopChan    chan struct{}
	running     bool
	title       string
	showSpinner bool
	lastRender  string
	startTime   time.Time
	currentTask int
}

// NewWorkflow creates a new workflow tracker.
func NewWorkflow(w io.Writer, title string) *Workflow {
	return &Workflow{
		writer:      w,
		title:       title,
		tasks:       make([]*Task, 0),
		stopChan:    make(chan struct{}),
		showSpinner: true,
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
		wf.currentTask = idx
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
	wf.startTime = time.Now()
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
				wf.render()
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
	wf.renderFinal()
}

// render displays the current state (during animation).
func (wf *Workflow) render() {
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
		b.WriteString(wf.renderTask(task))
		b.WriteString("\n")
	}

	output := b.String()
	wf.lastRender = strings.TrimSuffix(output, "\n")
	fmt.Fprint(wf.writer, output)
}

// renderFinal renders the final state without animation.
func (wf *Workflow) renderFinal() {
	wf.mu.Lock()
	defer wf.mu.Unlock()

	var b strings.Builder

	// Clear previous output.
	if wf.lastRender != "" {
		lineCount := strings.Count(wf.lastRender, "\n") + 1
		for i := 0; i < lineCount; i++ {
			b.WriteString("\033[A\033[K")
		}
	}

	// Render final state of all tasks.
	for _, task := range wf.tasks {
		b.WriteString(wf.renderTaskFinal(task))
		b.WriteString("\n")
	}

	fmt.Fprint(wf.writer, b.String())
}

func (wf *Workflow) renderTask(task *Task) string {
	var icon string
	var nameStyle styleWrapper
	var msgStyle styleWrapper

	switch task.Status {
	case TaskPending:
		icon = Muted.Render("○")
		nameStyle = StepPending
		msgStyle = Dim
	case TaskRunning:
		icon = Secondary.Render(spinnerFrames[wf.spinnerIdx])
		nameStyle = StepRunning
		msgStyle = Secondary
	case TaskDone:
		icon = GetCheckMark()
		nameStyle = StepComplete
		msgStyle = Dim
	case TaskFailed:
		icon = GetCrossMark()
		nameStyle = StepFailed
		msgStyle = Error
	case TaskSkipped:
		icon = Warning.Render("⊘")
		nameStyle = StepSkipped
		msgStyle = Warning
	}

	line := fmt.Sprintf("%s %s", icon, nameStyle.Render(task.Name))
	if task.Message != "" {
		line += " " + msgStyle.Render(task.Message)
	}

	return line
}

func (wf *Workflow) renderTaskFinal(task *Task) string {
	var icon string
	var nameStyle styleWrapper

	switch task.Status {
	case TaskPending:
		icon = Muted.Render("○")
		nameStyle = StepPending
	case TaskRunning:
		// Shouldn't happen in final render, treat as pending.
		icon = Muted.Render("○")
		nameStyle = StepPending
	case TaskDone:
		icon = GetCheckMark()
		nameStyle = StepComplete
	case TaskFailed:
		icon = GetCrossMark()
		nameStyle = StepFailed
	case TaskSkipped:
		icon = Warning.Render("⊘")
		nameStyle = StepSkipped
	}

	line := fmt.Sprintf("%s %s", icon, nameStyle.Render(task.Name))

	// Show details for completed tasks.
	if task.Status == TaskDone && task.Details != "" {
		line += " " + Dim.Render("→ "+task.Details)
	} else if task.Status == TaskFailed && task.Message != "" {
		line += " " + Error.Render("→ "+task.Message)
	} else if task.Status == TaskSkipped && task.Message != "" {
		line += " " + Warning.Render("→ "+task.Message)
	}

	return line
}

// SimpleSpinner provides a simple inline spinner for short operations.
type SimpleSpinner struct {
	writer     io.Writer
	message    string
	stopChan   chan struct{}
	doneChan   chan struct{}
	running    bool
	mu         sync.Mutex
	spinnerIdx int
}

// NewSimpleSpinner creates a new simple spinner.
func NewSimpleSpinner(w io.Writer, message string) *SimpleSpinner {
	return &SimpleSpinner{
		writer:   w,
		message:  message,
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}
}

// Start begins the spinner animation.
func (s *SimpleSpinner) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		defer close(s.doneChan)

		for {
			select {
			case <-s.stopChan:
				return
			case <-ticker.C:
				s.mu.Lock()
				s.spinnerIdx = (s.spinnerIdx + 1) % len(spinnerFrames)
				frame := spinnerFrames[s.spinnerIdx]
				s.mu.Unlock()

				// Clear line and print spinner.
				fmt.Fprintf(s.writer, "\r\033[K%s %s",
					Secondary.Render(frame),
					s.message)
			}
		}
	}()
}

// Stop ends the spinner with a result.
func (s *SimpleSpinner) Stop(success bool, finalMessage string) {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopChan)
	<-s.doneChan // Wait for goroutine to finish

	// Clear the spinner line and print final result.
	fmt.Fprint(s.writer, "\r\033[K")
	if success {
		fmt.Fprintf(s.writer, "%s %s\n", GetCheckMark(), finalMessage)
	} else {
		fmt.Fprintf(s.writer, "%s %s\n", GetCrossMark(), Error.Render(finalMessage))
	}
}

// UpdateMessage updates the spinner message.
func (s *SimpleSpinner) UpdateMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = message
}
