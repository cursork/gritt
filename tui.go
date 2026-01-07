package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"gritt/ride"
)

// model is the bubbletea model for the gritt TUI.
type model struct {
	client  *ride.Client
	output  []string // Session output lines
	input   string   // Current input line
	cursor  int      // Cursor position in input
	history []string // Input history
	histIdx int      // Current position in history (-1 = current input)
	width   int
	height  int
	ready   bool // Interpreter ready for input
	err     error
}

// rideMsg is received when the RIDE client gets a message.
type rideMsg struct {
	msg *ride.Message
	err error
}

// initialModel creates a new model with the given client.
func initialModel(client *ride.Client) model {
	return model{
		client:  client,
		output:  []string{},
		history: []string{},
		histIdx: -1,
		ready:   true,
	}
}

// waitForRide returns a command that waits for the next RIDE message.
func waitForRide(client *ride.Client) tea.Cmd {
	return func() tea.Msg {
		msg, _, err := client.Recv()
		return rideMsg{msg: msg, err: err}
	}
}

func (m model) Init() tea.Cmd {
	return waitForRide(m.client)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case rideMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		return m.handleRide(msg.msg)
	}

	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit

	case tea.KeyEnter:
		if !m.ready || m.input == "" {
			return m, nil
		}
		// Save to history
		m.history = append(m.history, m.input)
		m.histIdx = -1
		// Echo input to output (with prompt)
		m.output = append(m.output, "      "+m.input)
		// Send to interpreter
		code := m.input
		m.input = ""
		m.cursor = 0
		m.ready = false
		err := m.client.Send("Execute", map[string]any{"text": code + "\n", "trace": 0})
		if err != nil {
			m.err = err
			return m, tea.Quit
		}
		return m, waitForRide(m.client)

	case tea.KeyBackspace:
		if m.cursor > 0 {
			m.input = m.input[:m.cursor-1] + m.input[m.cursor:]
			m.cursor--
		}
		return m, nil

	case tea.KeyDelete:
		if m.cursor < len(m.input) {
			m.input = m.input[:m.cursor] + m.input[m.cursor+1:]
		}
		return m, nil

	case tea.KeyLeft:
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case tea.KeyRight:
		if m.cursor < len(m.input) {
			m.cursor++
		}
		return m, nil

	case tea.KeyHome:
		m.cursor = 0
		return m, nil

	case tea.KeyEnd:
		m.cursor = len(m.input)
		return m, nil

	case tea.KeyUp:
		if len(m.history) == 0 {
			return m, nil
		}
		if m.histIdx == -1 {
			m.histIdx = len(m.history) - 1
		} else if m.histIdx > 0 {
			m.histIdx--
		}
		m.input = m.history[m.histIdx]
		m.cursor = len(m.input)
		return m, nil

	case tea.KeyDown:
		if m.histIdx == -1 {
			return m, nil
		}
		if m.histIdx < len(m.history)-1 {
			m.histIdx++
			m.input = m.history[m.histIdx]
		} else {
			m.histIdx = -1
			m.input = ""
		}
		m.cursor = len(m.input)
		return m, nil

	default:
		// Insert character
		if msg.Type == tea.KeyRunes {
			for _, r := range msg.Runes {
				m.input = m.input[:m.cursor] + string(r) + m.input[m.cursor:]
				m.cursor++
			}
		}
		return m, nil
	}
}

func (m model) handleRide(msg *ride.Message) (tea.Model, tea.Cmd) {
	if msg == nil {
		return m, waitForRide(m.client)
	}

	switch msg.Command {
	case "AppendSessionOutput":
		// type 14 is input echo - skip it
		if t, ok := msg.Args["type"].(float64); ok && t == 14 {
			return m, waitForRide(m.client)
		}
		if result, ok := msg.Args["result"].(string); ok {
			// Split into lines, removing trailing newline
			result = strings.TrimSuffix(result, "\n")
			lines := strings.Split(result, "\n")
			m.output = append(m.output, lines...)
		}

	case "SetPromptType":
		if t, ok := msg.Args["type"].(float64); ok && t > 0 {
			m.ready = true
		}
	}

	return m, waitForRide(m.client)
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	var b strings.Builder

	// Calculate available height for output (leave 2 lines for input + status)
	outputHeight := m.height - 2
	if outputHeight < 1 {
		outputHeight = 10
	}

	// Show last N lines of output
	startLine := 0
	if len(m.output) > outputHeight {
		startLine = len(m.output) - outputHeight
	}
	for i := startLine; i < len(m.output); i++ {
		b.WriteString(m.output[i])
		b.WriteString("\n")
	}

	// Pad with empty lines if needed
	for i := len(m.output); i < outputHeight; i++ {
		b.WriteString("\n")
	}

	// Input line with prompt
	prompt := "      "
	if !m.ready {
		prompt = "  ... "
	}
	b.WriteString(prompt)

	// Show input with cursor
	if m.cursor < len(m.input) {
		b.WriteString(m.input[:m.cursor])
		b.WriteString("\033[7m") // Reverse video for cursor
		b.WriteString(string(m.input[m.cursor]))
		b.WriteString("\033[0m")
		b.WriteString(m.input[m.cursor+1:])
	} else {
		b.WriteString(m.input)
		b.WriteString("\033[7m \033[0m") // Cursor at end
	}

	return b.String()
}
