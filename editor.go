package main

// EditorWindow holds state for an open editor/tracer window from Dyalog
type EditorWindow struct {
	Token      int      // Unique window identifier from Dyalog
	Name       string   // Function/operator name
	Text       []string // Lines of text
	EntityType int      // Type: 1=function, 256=namespace, etc.
	Stop       []int    // Breakpoint line numbers (0-based)
	Monitor    []int    // Monitored lines
	Trace      []int    // Trace points
	CurrentRow int      // Initial cursor position
	ReadOnly   bool     // Whether editor is read-only
	Debugger   bool     // True if this is a tracer window

	// Editor state (local to gritt)
	Modified     bool
	PendingClose bool // True if we're waiting for ReplySaveChanges before closing
	CursorRow    int
	CursorCol    int
}

// NewEditorWindow creates an EditorWindow from OpenWindow/UpdateWindow message args
func NewEditorWindow(args map[string]any) *EditorWindow {
	w := &EditorWindow{}

	if token, ok := args["token"].(float64); ok {
		w.Token = int(token)
	}
	if name, ok := args["name"].(string); ok {
		w.Name = name
	}
	if entityType, ok := args["entityType"].(float64); ok {
		w.EntityType = int(entityType)
	}
	if currentRow, ok := args["currentRow"].(float64); ok {
		w.CurrentRow = int(currentRow)
		w.CursorRow = int(currentRow)
	}
	// debugger is 0/1 integer, not boolean
	if debugger, ok := args["debugger"].(float64); ok {
		w.Debugger = debugger != 0
	}
	// readOnly is 0/1 integer, not boolean
	if readOnly, ok := args["readOnly"].(float64); ok {
		w.ReadOnly = readOnly != 0
	}

	// Parse text array
	if text, ok := args["text"].([]any); ok {
		w.Text = make([]string, len(text))
		for i, line := range text {
			if s, ok := line.(string); ok {
				w.Text[i] = s
			}
		}
	}

	// Parse stop array (breakpoints)
	if stop, ok := args["stop"].([]any); ok {
		w.Stop = make([]int, len(stop))
		for i, line := range stop {
			if n, ok := line.(float64); ok {
				w.Stop[i] = int(n)
			}
		}
	}

	// Parse monitor array
	if monitor, ok := args["monitor"].([]any); ok {
		w.Monitor = make([]int, len(monitor))
		for i, line := range monitor {
			if n, ok := line.(float64); ok {
				w.Monitor[i] = int(n)
			}
		}
	}

	// Parse trace array
	if trace, ok := args["trace"].([]any); ok {
		w.Trace = make([]int, len(trace))
		for i, line := range trace {
			if n, ok := line.(float64); ok {
				w.Trace[i] = int(n)
			}
		}
	}

	return w
}

// Update refreshes window content from UpdateWindow message args
func (w *EditorWindow) Update(args map[string]any) {
	if text, ok := args["text"].([]any); ok {
		w.Text = make([]string, len(text))
		for i, line := range text {
			if s, ok := line.(string); ok {
				w.Text[i] = s
			}
		}
	}
	if currentRow, ok := args["currentRow"].(float64); ok {
		w.CurrentRow = int(currentRow)
	}
	if debugger, ok := args["debugger"].(float64); ok {
		w.Debugger = debugger != 0
	}
	if stop, ok := args["stop"].([]any); ok {
		w.Stop = make([]int, len(stop))
		for i, line := range stop {
			if n, ok := line.(float64); ok {
				w.Stop[i] = int(n)
			}
		}
	}
}

// HasStop returns true if the given line has a breakpoint
func (w *EditorWindow) HasStop(line int) bool {
	for _, s := range w.Stop {
		if s == line {
			return true
		}
	}
	return false
}

// ToggleStop adds or removes a breakpoint on the given line
func (w *EditorWindow) ToggleStop(line int) {
	// Check if already present
	for i, s := range w.Stop {
		if s == line {
			// Remove it
			w.Stop = append(w.Stop[:i], w.Stop[i+1:]...)
			return
		}
	}
	// Not present - add it
	w.Stop = append(w.Stop, line)
}
