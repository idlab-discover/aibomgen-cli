package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/idlab-discover/aibomgen-cli/internal/apperr"
	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
)

// ModelSelectorConfig configures the model selector.
type ModelSelectorConfig struct {
	HFToken string
	Timeout time.Duration
}

// modelItem represents a model in the list.
type modelItem struct {
	id        string
	author    string
	downloads int
	likes     int
	tags      []string
	selected  bool
}

func (i modelItem) Title() string {
	var checkbox string
	if i.selected {
		checkbox = Success.Render("[✓] ")
	} else {
		checkbox = Dim.Render("[ ] ")
	}
	return checkbox + i.id
}

func (i modelItem) Description() string {
	return fmt.Sprintf("%s Downloads: %s · Likes: %d",
		Dim.Render(fmt.Sprintf("by %s ·", i.author)),
		Dim.Render(formatNumber(i.downloads)),
		i.likes,
	)
}

func (i modelItem) FilterValue() string { return i.id }

// modelSelectorModel is the Bubble Tea model for the interactive selector.
type modelSelectorModel struct {
	textInput textinput.Model
	list      list.Model
	searcher  *fetcher.ModelSearcher

	filteredItems []list.Item
	selected      map[string]bool
	searching     bool
	searchQuery   string
	err           error
	quitting      bool
	confirmed     bool
	width         int
	height        int
}

type searchResultMsg struct {
	results []fetcher.ModelSearchResult
	err     error
}

type searchDebounceMsg struct{}

// NewModelSelector creates a new interactive model selector.
func NewModelSelector(config ModelSelectorConfig) *modelSelectorModel {
	ti := textinput.New()
	ti.Placeholder = "Search Hugging Face models..."
	ti.Focus()
	ti.CharLimit = 156
	ti.SetWidth(50)

	searcher := &fetcher.ModelSearcher{
		Client: fetcher.NewHFClient(config.Timeout, config.HFToken),
	}

	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(3)
	delegate.SetSpacing(0)

	// Customize delegate styles.
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(ColorHighlight).
		BorderForeground(ColorPrimary)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(ColorTextDim).
		BorderForeground(ColorPrimary)

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Select Models"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(false) // We handle our own filtering
	l.SetShowHelp(true)
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		Padding(0, 0, 1, 0)

	return &modelSelectorModel{
		textInput: ti,
		list:      l,
		searcher:  searcher,
		selected:  make(map[string]bool),
		width:     80,
		height:    24,
	}
}

// Init initializes the model.
func (m *modelSelectorModel) Init() tea.Cmd {
	// Perform initial search with empty query to get popular models.
	return tea.Batch(
		textinput.Blink,
		m.performSearch(""),
	)
}

// Update handles messages.
func (m *modelSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't match space when typing in text input.
		if m.textInput.Focused() {
			switch msg.String() {
			case "ctrl+c", "esc":
				m.quitting = true
				return m, tea.Quit
			case "enter":
				if m.textInput.Value() != "" {
					// Unfocus text input and focus list.
					m.textInput.Blur()
					return m, nil
				}
			case "down", "up":
				// If we have items, switch to list navigation.
				if len(m.filteredItems) > 0 {
					m.textInput.Blur()
					var cmd tea.Cmd
					m.list, cmd = m.list.Update(msg)
					return m, cmd
				}
			default:
				// Update text input and trigger debounced search.
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)

				query := m.textInput.Value()
				if query != m.searchQuery {
					m.searchQuery = query
					// Debounce search: wait 300ms after last keystroke.
					cmds = append(cmds, m.debounceSearch())
				}
				cmds = append(cmds, cmd)
				return m, tea.Batch(cmds...)
			}
		} else {
			// List is focused.
			switch msg.String() {
			case "ctrl+c", "esc":
				m.quitting = true
				return m, tea.Quit
			case "enter":
				m.confirmed = true
				m.quitting = true
				return m, tea.Quit
			case "s":
				// Toggle selection.
				if i, ok := m.list.SelectedItem().(modelItem); ok {
					m.selected[i.id] = !m.selected[i.id]
					m.updateItemSelection(i.id, m.selected[i.id])
				}
				return m, nil
			case "/", "i":
				// Focus back on search input.
				m.textInput.Focus()
				return m, textinput.Blink
			default:
				// Let list handle other keys (arrow keys, etc.).
				var cmd tea.Cmd
				m.list, cmd = m.list.Update(msg)
				return m, cmd
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-10)
		return m, nil

	case searchDebounceMsg:
		// Perform the search.
		return m, m.performSearch(m.searchQuery)

	case searchResultMsg:
		m.searching = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}

		// Convert results to list items.
		items := make([]list.Item, len(msg.results))
		for i, result := range msg.results {
			items[i] = modelItem{
				id:        result.ID,
				author:    result.Author,
				downloads: result.Downloads,
				likes:     result.Likes,
				tags:      result.Tags,
				selected:  m.selected[result.ID],
			}
		}
		m.filteredItems = items
		m.list.SetItems(items)
		return m, nil
	}

	// Update list.
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the model.
func (m *modelSelectorModel) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}

	var b strings.Builder

	// Title.
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Padding(1, 0)
	b.WriteString(titleStyle.Render("🤗 Hugging Face Model Selector"))
	b.WriteString("\n\n")

	// Search input.
	searchLabel := Dim.Render("Search: ")
	b.WriteString(searchLabel)
	b.WriteString(m.textInput.View())

	if m.searching {
		b.WriteString(Dim.Render(" (searching...)"))
	}
	b.WriteString("\n\n")

	// List of models.
	b.WriteString(m.list.View())
	b.WriteString("\n\n")

	// Selected models.
	var selectedIDs []string
	for id, selected := range m.selected {
		if selected {
			selectedIDs = append(selectedIDs, id)
		}
	}

	if len(selectedIDs) > 0 {
		b.WriteString(fmt.Sprintf("%s %s\n",
			Success.Render("Selected:"),
			Highlight.Render(fmt.Sprintf("%d model(s)", len(selectedIDs)))))
	}

	// Help text.
	helpStyle := lipgloss.NewStyle().Foreground(ColorTextDim)
	if m.textInput.Focused() {
		b.WriteString(helpStyle.Render("↑/↓: move to list · enter: finish search · esc: cancel"))
	} else {
		b.WriteString(helpStyle.Render("s: select · ↑/↓: navigate · enter: confirm · /: search · esc: cancel"))
	}

	// Error display.
	if m.err != nil {
		b.WriteString("\n\n")
		b.WriteString(Error.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	return tea.NewView(b.String())
}

// debounceSearch returns a command that triggers search after a delay.
func (m *modelSelectorModel) debounceSearch() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(300 * time.Millisecond)
		return searchDebounceMsg{}
	}
}

// performSearch executes the search.
func (m *modelSelectorModel) performSearch(query string) tea.Cmd {
	m.searching = true
	return func() tea.Msg {
		results, err := m.searcher.Search(query, 1000)
		return searchResultMsg{results: results, err: err}
	}
}

// updateItemSelection updates the selected state of an item.
func (m *modelSelectorModel) updateItemSelection(id string, selected bool) {
	for i, item := range m.filteredItems {
		if mi, ok := item.(modelItem); ok && mi.id == id {
			m.filteredItems[i] = modelItem{
				id:        mi.id,
				author:    mi.author,
				downloads: mi.downloads,
				likes:     mi.likes,
				tags:      mi.tags,
				selected:  selected,
			}
			break
		}
	}
	m.list.SetItems(m.filteredItems)
}

// GetSelectedModels returns the list of selected model IDs.
func (m *modelSelectorModel) GetSelectedModels() []string {
	var models []string
	for id, selected := range m.selected {
		if selected {
			models = append(models, id)
		}
	}
	return models
}

// WasConfirmed returns true if the user confirmed the selection.
func (m *modelSelectorModel) WasConfirmed() bool {
	return m.confirmed
}

// RunModelSelector runs the interactive model selector and returns selected model IDs.
func RunModelSelector(config ModelSelectorConfig) ([]string, error) {
	p := tea.NewProgram(NewModelSelector(config))
	m, err := p.Run()
	if err != nil {
		return nil, err
	}

	model := m.(*modelSelectorModel)
	if !model.WasConfirmed() {
		return nil, apperr.ErrCancelled
	}

	return model.GetSelectedModels(), nil
}

// formatNumber formats a number with commas for thousands.
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	str := fmt.Sprintf("%d", n)
	var result []rune
	for i, r := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, r)
	}
	return string(result)
}
