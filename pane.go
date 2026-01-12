package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DragMode represents the current drag operation
type DragMode int

const (
	DragNone DragMode = iota
	DragMove
	DragResizeN
	DragResizeS
	DragResizeE
	DragResizeW
	DragResizeNE
	DragResizeNW
	DragResizeSE
	DragResizeSW
)

// HitZone represents where a mouse click landed on a pane
type HitZone int

const (
	ZoneNone HitZone = iota
	ZoneTitleBar
	ZoneContent
	ZoneEdgeN
	ZoneEdgeS
	ZoneEdgeE
	ZoneEdgeW
	ZoneCornerNE
	ZoneCornerNW
	ZoneCornerSE
	ZoneCornerSW
)

// PaneContent defines what a pane can display
type PaneContent interface {
	// Render returns the content string to display within pane borders
	// w, h are the content area dimensions (inside borders)
	Render(w, h int) string

	// HandleKey processes keyboard input when this pane has focus
	// Returns true if the key was consumed
	HandleKey(msg tea.KeyMsg) bool

	// HandleMouse processes mouse input within this pane's bounds
	// x, y are relative to the pane's content area
	HandleMouse(x, y int, msg tea.MouseMsg) bool

	// Title returns the pane's title
	Title() string
}

// Pane represents a floating pane in the TUI
type Pane struct {
	ID      string
	X, Y    int // Top-left position
	Width   int // Including borders
	Height  int
	MinW    int // Minimum size constraints
	MinH    int
	ZIndex  int
	Focused bool
	Content PaneContent

	// Drag state
	dragging    bool
	dragMode    DragMode
	dragStartX  int
	dragStartY  int
	dragOffsetX int
	dragOffsetY int
}

// NewPane creates a new pane with sensible defaults
func NewPane(id string, content PaneContent, x, y, w, h int) *Pane {
	return &Pane{
		ID:      id,
		X:       x,
		Y:       y,
		Width:   w,
		Height:  h,
		MinW:    20,
		MinH:    5,
		Content: content,
	}
}

// HitZone determines where a point falls within the pane
func (p *Pane) HitZone(x, y int) HitZone {
	// Outside pane entirely
	if x < p.X || x >= p.X+p.Width || y < p.Y || y >= p.Y+p.Height {
		return ZoneNone
	}

	relX := x - p.X
	relY := y - p.Y

	// Corners (1 cell each)
	if relY == 0 && relX == 0 {
		return ZoneCornerNW
	}
	if relY == 0 && relX == p.Width-1 {
		return ZoneCornerNE
	}
	if relY == p.Height-1 && relX == 0 {
		return ZoneCornerSW
	}
	if relY == p.Height-1 && relX == p.Width-1 {
		return ZoneCornerSE
	}

	// Edges
	if relY == 0 {
		return ZoneTitleBar // Top edge is title bar
	}
	if relY == p.Height-1 {
		return ZoneEdgeS
	}
	if relX == 0 {
		return ZoneEdgeW
	}
	if relX == p.Width-1 {
		return ZoneEdgeE
	}

	return ZoneContent
}

// StartDrag begins a drag operation
func (p *Pane) StartDrag(mode DragMode, mouseX, mouseY int) {
	p.dragging = true
	p.dragMode = mode
	p.dragStartX = mouseX
	p.dragStartY = mouseY
	p.dragOffsetX = mouseX - p.X
	p.dragOffsetY = mouseY - p.Y
}

// UpdateDrag updates the pane position/size during a drag
func (p *Pane) UpdateDrag(mouseX, mouseY, screenW, screenH int) {
	if !p.dragging {
		return
	}

	switch p.dragMode {
	case DragMove:
		newX := mouseX - p.dragOffsetX
		newY := mouseY - p.dragOffsetY
		// Constrain to screen (allow title bar to stay visible)
		p.X = clamp(newX, -p.Width+5, screenW-5)
		p.Y = clamp(newY, 0, screenH-1)

	case DragResizeE:
		newW := mouseX - p.X + 1
		p.Width = max(newW, p.MinW)

	case DragResizeS:
		newH := mouseY - p.Y + 1
		p.Height = max(newH, p.MinH)

	case DragResizeW:
		delta := p.dragStartX - mouseX
		newW := p.Width + delta
		if newW >= p.MinW {
			p.X = mouseX
			p.Width = newW
			p.dragStartX = mouseX
		}

	case DragResizeN:
		delta := p.dragStartY - mouseY
		newH := p.Height + delta
		if newH >= p.MinH {
			p.Y = mouseY
			p.Height = newH
			p.dragStartY = mouseY
		}

	case DragResizeSE:
		p.Width = max(mouseX-p.X+1, p.MinW)
		p.Height = max(mouseY-p.Y+1, p.MinH)

	case DragResizeSW:
		newW := p.X + p.Width - mouseX
		if newW >= p.MinW {
			p.X = mouseX
			p.Width = newW
		}
		p.Height = max(mouseY-p.Y+1, p.MinH)

	case DragResizeNE:
		p.Width = max(mouseX-p.X+1, p.MinW)
		delta := p.dragStartY - mouseY
		newH := p.Height + delta
		if newH >= p.MinH {
			p.Y = mouseY
			p.Height = newH
			p.dragStartY = mouseY
		}

	case DragResizeNW:
		delta := p.dragStartX - mouseX
		newW := p.Width + delta
		if newW >= p.MinW {
			p.X = mouseX
			p.Width = newW
			p.dragStartX = mouseX
		}
		delta = p.dragStartY - mouseY
		newH := p.Height + delta
		if newH >= p.MinH {
			p.Y = mouseY
			p.Height = newH
			p.dragStartY = mouseY
		}
	}
}

// StopDrag ends the current drag operation
func (p *Pane) StopDrag() {
	p.dragging = false
	p.dragMode = DragNone
}

// Render renders the pane with borders and content
func (p *Pane) Render() string {
	// Border characters
	var tl, tr, bl, br, h, v string
	var borderColor lipgloss.Color

	if p.Focused {
		tl, tr, bl, br = "╔", "╗", "╚", "╝"
		h, v = "═", "║"
		borderColor = lipgloss.Color("63") // Bright blue
	} else {
		tl, tr, bl, br = "┌", "┐", "└", "┘"
		h, v = "─", "│"
		borderColor = lipgloss.Color("240") // Gray
	}

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	titleStyle := lipgloss.NewStyle().Foreground(borderColor).Bold(true)

	contentW := p.Width - 2
	contentH := p.Height - 2
	if contentW < 1 {
		contentW = 1
	}
	if contentH < 1 {
		contentH = 1
	}

	// Render content
	content := ""
	if p.Content != nil {
		content = p.Content.Render(contentW, contentH)
	}
	contentLines := strings.Split(content, "\n")

	// Build the box
	var lines []string

	// Title bar
	title := ""
	if p.Content != nil {
		title = p.Content.Title()
	}
	titleLen := len([]rune(title))
	padding := contentW - titleLen - 2
	if padding < 0 {
		// Truncate title
		runes := []rune(title)
		if len(runes) > contentW-2 {
			title = string(runes[:contentW-2])
		}
		padding = 0
	}
	topBar := borderStyle.Render(tl) +
		titleStyle.Render(" "+title+" ") +
		borderStyle.Render(strings.Repeat(h, padding)+tr)
	lines = append(lines, topBar)

	// Content lines
	for i := 0; i < contentH; i++ {
		line := ""
		if i < len(contentLines) {
			line = contentLines[i]
		}
		// Pad to width
		lineRunes := []rune(line)
		if len(lineRunes) < contentW {
			line = line + strings.Repeat(" ", contentW-len(lineRunes))
		} else if len(lineRunes) > contentW {
			line = string(lineRunes[:contentW])
		}
		lines = append(lines, borderStyle.Render(v)+line+borderStyle.Render(v))
	}

	// Bottom border
	lines = append(lines, borderStyle.Render(bl+strings.Repeat(h, contentW)+br))

	return strings.Join(lines, "\n")
}

// PaneManager tracks all floating panes
type PaneManager struct {
	panes     map[string]*Pane
	zOrder    []string // Ordered by z-index, last = topmost
	focusedID string
	screenW   int
	screenH   int
}

// NewPaneManager creates a new pane manager
func NewPaneManager(screenW, screenH int) *PaneManager {
	return &PaneManager{
		panes:   make(map[string]*Pane),
		zOrder:  []string{},
		screenW: screenW,
		screenH: screenH,
	}
}

// Add adds a pane to the manager
func (pm *PaneManager) Add(pane *Pane) {
	pm.panes[pane.ID] = pane
	pm.zOrder = append(pm.zOrder, pane.ID)
	pane.ZIndex = len(pm.zOrder)
}

// Remove removes a pane from the manager
func (pm *PaneManager) Remove(id string) {
	delete(pm.panes, id)
	for i, pid := range pm.zOrder {
		if pid == id {
			pm.zOrder = append(pm.zOrder[:i], pm.zOrder[i+1:]...)
			break
		}
	}
	if pm.focusedID == id {
		pm.focusedID = ""
	}
}

// Get returns a pane by ID
func (pm *PaneManager) Get(id string) *Pane {
	return pm.panes[id]
}

// Focus focuses a pane and raises it to the top
func (pm *PaneManager) Focus(id string) {
	// Unfocus current
	if pm.focusedID != "" {
		if p := pm.panes[pm.focusedID]; p != nil {
			p.Focused = false
		}
	}

	pm.focusedID = id

	// Focus and raise new pane
	if p := pm.panes[id]; p != nil {
		p.Focused = true
		pm.raise(id)
	}
}

// raise moves a pane to the top of z-order
func (pm *PaneManager) raise(id string) {
	for i, pid := range pm.zOrder {
		if pid == id {
			pm.zOrder = append(pm.zOrder[:i], pm.zOrder[i+1:]...)
			pm.zOrder = append(pm.zOrder, id)
			break
		}
	}
}

// FocusNext cycles focus to the next pane
func (pm *PaneManager) FocusNext() {
	if len(pm.zOrder) == 0 {
		return
	}

	if pm.focusedID == "" {
		pm.Focus(pm.zOrder[0])
		return
	}

	for i, id := range pm.zOrder {
		if id == pm.focusedID {
			nextIdx := (i + 1) % len(pm.zOrder)
			pm.Focus(pm.zOrder[nextIdx])
			return
		}
	}
}

// FocusPrev cycles focus to the previous pane
func (pm *PaneManager) FocusPrev() {
	if len(pm.zOrder) == 0 {
		return
	}

	if pm.focusedID == "" {
		pm.Focus(pm.zOrder[len(pm.zOrder)-1])
		return
	}

	for i, id := range pm.zOrder {
		if id == pm.focusedID {
			prevIdx := (i - 1 + len(pm.zOrder)) % len(pm.zOrder)
			pm.Focus(pm.zOrder[prevIdx])
			return
		}
	}
}

// FocusedPane returns the currently focused pane
func (pm *PaneManager) FocusedPane() *Pane {
	return pm.panes[pm.focusedID]
}

// PaneAt returns the topmost pane at the given coordinates
func (pm *PaneManager) PaneAt(x, y int) *Pane {
	// Iterate from top of z-order (last) to bottom (first)
	for i := len(pm.zOrder) - 1; i >= 0; i-- {
		pane := pm.panes[pm.zOrder[i]]
		if pane != nil && pane.HitZone(x, y) != ZoneNone {
			return pane
		}
	}
	return nil
}

// UpdateSize updates the screen dimensions
func (pm *PaneManager) UpdateSize(w, h int) {
	pm.screenW = w
	pm.screenH = h

	// Adjust pane positions to stay visible
	for _, pane := range pm.panes {
		// Ensure at least 5 chars visible horizontally
		if pane.X > w-5 {
			pane.X = w - 5
		}
		// Ensure title bar visible
		if pane.Y >= h {
			pane.Y = h - 1
		}
	}
}

// HasPanes returns true if there are any panes
func (pm *PaneManager) HasPanes() bool {
	return len(pm.zOrder) > 0
}

// Render composites all panes over the base content
func (pm *PaneManager) Render(base string) string {
	if len(pm.zOrder) == 0 {
		return base
	}

	// Convert base to grid
	grid := stringToGrid(base, pm.screenW, pm.screenH)

	// Render panes in z-order (lowest first)
	for _, id := range pm.zOrder {
		pane := pm.panes[id]
		if pane != nil {
			paneStr := pane.Render()
			overlayOnGrid(grid, paneStr, pane.X, pane.Y)
		}
	}

	return gridToString(grid)
}

// stringToGrid converts a rendered string to a 2D rune grid
func stringToGrid(s string, w, h int) [][]rune {
	grid := make([][]rune, h)
	for i := range grid {
		grid[i] = make([]rune, w)
		for j := range grid[i] {
			grid[i][j] = ' '
		}
	}

	lines := strings.Split(s, "\n")
	for y, line := range lines {
		if y >= h {
			break
		}
		x := 0
		for _, r := range line {
			if x >= w {
				break
			}
			grid[y][x] = r
			x++
		}
	}

	return grid
}

// overlayOnGrid paints a string onto the grid at the given position
func overlayOnGrid(grid [][]rune, s string, x, y int) {
	lines := strings.Split(s, "\n")
	h := len(grid)
	w := 0
	if h > 0 {
		w = len(grid[0])
	}

	for dy, line := range lines {
		gy := y + dy
		if gy < 0 || gy >= h {
			continue
		}
		gx := x
		for _, r := range line {
			if gx >= 0 && gx < w {
				grid[gy][gx] = r
			}
			gx++
		}
	}
}

// gridToString converts a 2D rune grid back to a string
func gridToString(grid [][]rune) string {
	lines := make([]string, len(grid))
	for i, row := range grid {
		lines[i] = string(row)
	}
	return strings.Join(lines, "\n")
}

// zoneToDragMode converts a hit zone to a drag mode
func zoneToDragMode(zone HitZone) DragMode {
	switch zone {
	case ZoneTitleBar:
		return DragMove
	case ZoneEdgeN:
		return DragResizeN
	case ZoneEdgeS:
		return DragResizeS
	case ZoneEdgeE:
		return DragResizeE
	case ZoneEdgeW:
		return DragResizeW
	case ZoneCornerNE:
		return DragResizeNE
	case ZoneCornerNW:
		return DragResizeNW
	case ZoneCornerSE:
		return DragResizeSE
	case ZoneCornerSW:
		return DragResizeSW
	default:
		return DragNone
	}
}

// Helper functions
func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
