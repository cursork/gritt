package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// LogBuffer is a shared buffer that survives Model copies
type LogBuffer struct {
	Lines []string
}

// DebugPane displays the debug log using a viewport
type DebugPane struct {
	viewport    viewport.Model
	log         *LogBuffer
	lastContent string // Track content to detect changes
}

// NewDebugPane creates a debug pane backed by the given log buffer
func NewDebugPane(log *LogBuffer) *DebugPane {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true
	return &DebugPane{viewport: vp, log: log}
}

func (d *DebugPane) Title() string {
	return "debug"
}

func (d *DebugPane) Render(w, h int) string {
	// Update viewport dimensions
	d.viewport.Width = w
	d.viewport.Height = h

	// Build and set content every time
	content := strings.Join(d.log.Lines, "\n")

	// Check if content changed for auto-scroll decision
	contentChanged := content != d.lastContent
	wasAtBottom := d.viewport.AtBottom()

	d.viewport.SetContent(content)
	d.lastContent = content

	// Auto-scroll if content changed and we were at bottom (or content fits)
	if contentChanged && (wasAtBottom || d.viewport.TotalLineCount() <= h) {
		d.viewport.GotoBottom()
	}

	return d.viewport.View()
}

func (d *DebugPane) HandleKey(msg tea.KeyMsg) bool {
	var cmd tea.Cmd
	d.viewport, cmd = d.viewport.Update(msg)
	return cmd != nil
}

func (d *DebugPane) HandleMouse(x, y int, msg tea.MouseMsg) bool {
	var cmd tea.Cmd
	d.viewport, cmd = d.viewport.Update(msg)
	return cmd != nil
}
