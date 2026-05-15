package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// externalEditFinishedMsg is sent when the external $EDITOR process exits.
type externalEditFinishedMsg struct {
	token int
	path  string
	err   error
}

// externalEditFocused launches $EDITOR on the focused editor pane's text.
// Returns the tea.Cmd that suspends bubbletea and runs the editor.
//
// Refuses unless the focused pane is an editor in edit mode:
//   - tracer in trace mode: must press 'e' first
//   - read-only variable: must press Enter first to convert to APLAN
//
// All refusals surface as a transient red error in the status line.
func (m *Model) externalEditFocused() tea.Cmd {
	ep := m.focusedEditorPane()
	if ep == nil {
		m.transientErr = "external-edit: no editor pane focused"
		return nil
	}

	w := ep.window
	if (w.Debugger || w.ReadOnly) && !ep.editMode {
		switch {
		case w.Debugger:
			m.transientErr = "external-edit: press 'e' to enter tracer edit mode first"
		default:
			m.transientErr = "external-edit: press Enter to convert read-only value first"
		}
		return nil
	}

	editorCmd := strings.TrimSpace(os.Getenv("EDITOR"))
	if editorCmd == "" {
		editorCmd = "vi"
	}
	parts := strings.Fields(editorCmd)

	path := externalEditTempPath(w.Name, w.Token, w.EntityType)
	content := strings.Join(w.Text, "\n")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		m.transientErr = fmt.Sprintf("external-edit: %v", err)
		return nil
	}

	c := exec.Command(parts[0], append(parts[1:], path)...)
	token := w.Token
	m.log("→ external-edit win=%d editor=%q file=%s", token, editorCmd, path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return externalEditFinishedMsg{token: token, path: path, err: err}
	})
}

// handleExternalEditFinished processes the result of an external editor session.
// If the file content differs from the editor window, the window is updated and
// SaveChanges is sent. The temp file is removed in either case.
func (m *Model) handleExternalEditFinished(msg externalEditFinishedMsg) {
	defer os.Remove(msg.path)

	if msg.err != nil {
		m.transientErr = fmt.Sprintf("external-edit: %v", msg.err)
		m.log("external-edit win=%d failed: %v", msg.token, msg.err)
		return
	}

	w, exists := m.editors[msg.token]
	if !exists {
		m.log("external-edit win=%d: window no longer exists", msg.token)
		return
	}

	data, err := os.ReadFile(msg.path)
	if err != nil {
		m.transientErr = fmt.Sprintf("external-edit: %v", err)
		return
	}
	text := string(data)
	// Drop the single trailing newline most editors add on save, but
	// preserve any further trailing blank lines the user actually typed.
	text = strings.TrimSuffix(text, "\n")
	newLines := strings.Split(text, "\n")

	if linesEqual(newLines, w.Text) {
		m.log("external-edit win=%d: no changes", msg.token)
		return
	}

	w.Text = newLines
	w.Modified = true
	w.CursorRow = 0
	w.CursorCol = 0
	m.saveEditor(msg.token)
}

func linesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// externalEditTempPath builds a descriptive temp-file path for the editor.
// Name is sanitised so namespace-qualified names (e.g. "#.foo") don't confuse
// the filesystem; extension matches Dyalog's source conventions so syntax
// highlighters pick the right mode.
func externalEditTempPath(name string, token, entityType int) string {
	clean := sanitizeForFilename(name)
	if clean == "" {
		clean = "edit"
	}
	ext := extensionForEntityType(entityType)
	return filepath.Join(os.TempDir(), fmt.Sprintf("gritt-%s-%d%s", clean, token, ext))
}

func sanitizeForFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func extensionForEntityType(et int) string {
	switch et {
	case 1, 2, 3:
		return ".aplf"
	case 256:
		return ".apln"
	case 262144:
		return ".apla"
	default:
		return ".apl"
	}
}

// focusedEditorPane returns the EditorPane to operate on, walking the same
// fallback chain as formatFocusedEditor: focused pane → tracer → any editor.
// The fallback matters when the command is dispatched from the palette, which
// clears focus on dismiss.
func (m *Model) focusedEditorPane() *EditorPane {
	if fp := m.panes.FocusedPane(); fp != nil {
		if ep, ok := fp.Content.(*EditorPane); ok {
			return ep
		}
	}
	if pane := m.panes.Get("tracer"); pane != nil {
		if ep, ok := pane.Content.(*EditorPane); ok {
			return ep
		}
	}
	for token := range m.editors {
		paneID := fmt.Sprintf("editor:%d", token)
		if pane := m.panes.Get(paneID); pane != nil {
			if ep, ok := pane.Content.(*EditorPane); ok {
				return ep
			}
		}
	}
	return nil
}
