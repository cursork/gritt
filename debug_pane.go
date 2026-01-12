package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// DebugPane displays the debug log with scrolling
type DebugPane struct {
	log       *[]string // Pointer to Model's debugLog
	scrollPos int       // Scroll offset from bottom (0 = at bottom)
}

// NewDebugPane creates a debug pane backed by the given log slice
func NewDebugPane(log *[]string) *DebugPane {
	return &DebugPane{
		log:       log,
		scrollPos: 0,
	}
}

func (d *DebugPane) Title() string {
	return "debug"
}

func (d *DebugPane) Render(w, h int) string {
	log := *d.log
	if len(log) == 0 {
		return strings.Repeat(strings.Repeat(" ", w)+"\n", h-1) + strings.Repeat(" ", w)
	}

	// Calculate viewport
	// scrollPos 0 = show last h lines
	// scrollPos N = show h lines ending at len-N
	endIdx := len(log) - d.scrollPos
	if endIdx < 0 {
		endIdx = 0
	}
	if endIdx > len(log) {
		endIdx = len(log)
	}
	startIdx := endIdx - h
	if startIdx < 0 {
		startIdx = 0
	}

	lines := make([]string, h)
	for i := 0; i < h; i++ {
		srcIdx := startIdx + i
		if srcIdx >= endIdx || srcIdx >= len(log) {
			lines[i] = strings.Repeat(" ", w)
			continue
		}
		line := log[srcIdx]
		runes := []rune(line)
		if len(runes) > w {
			runes = runes[:w-1]
			line = string(runes) + "…"
		}
		// Pad to width
		if len([]rune(line)) < w {
			line += strings.Repeat(" ", w-len([]rune(line)))
		}
		lines[i] = line
	}

	return strings.Join(lines, "\n")
}

func (d *DebugPane) HandleKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyUp:
		d.scrollPos++
		d.clampScroll()
		return true
	case tea.KeyDown:
		if d.scrollPos > 0 {
			d.scrollPos--
		}
		return true
	case tea.KeyPgUp:
		d.scrollPos += 10
		d.clampScroll()
		return true
	case tea.KeyPgDown:
		d.scrollPos -= 10
		if d.scrollPos < 0 {
			d.scrollPos = 0
		}
		return true
	case tea.KeyHome:
		d.scrollPos = len(*d.log)
		d.clampScroll()
		return true
	case tea.KeyEnd:
		d.scrollPos = 0
		return true
	}
	return false
}

func (d *DebugPane) HandleMouse(x, y int, msg tea.MouseMsg) bool {
	switch msg.Type {
	case tea.MouseWheelUp:
		d.scrollPos += 3
		d.clampScroll()
		return true
	case tea.MouseWheelDown:
		d.scrollPos -= 3
		if d.scrollPos < 0 {
			d.scrollPos = 0
		}
		return true
	}
	return false
}

func (d *DebugPane) clampScroll() {
	maxScroll := len(*d.log)
	if d.scrollPos > maxScroll {
		d.scrollPos = maxScroll
	}
}
