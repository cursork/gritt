package main

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// findAssignedVars scans function body lines for assignment targets.
// Returns a deduplicated, sorted list of variable names that are assigned to.
// Skips comments, system variables (⎕name), and namespace member assignments (name.member←).
func findAssignedVars(lines []string) []string {
	assigned := make(map[string]bool)

	for _, line := range lines {
		// Strip comment: find ⍝ outside strings
		code := stripComment(line)
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}

		// Check for :For variable :In pattern
		forVars := extractForVars(code)
		for _, v := range forVars {
			assigned[v] = true
		}

		// Find all name← patterns
		findAssignments(code, assigned)
	}

	// Collect and sort
	var result []string
	for name := range assigned {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

// stripComment removes the comment portion of an APL line (from ⍝ onward),
// respecting string literals delimited by single quotes.
func stripComment(line string) string {
	inString := false
	runes := []rune(line)
	for i, r := range runes {
		if r == '\'' {
			inString = !inString
		} else if r == '⍝' && !inString {
			return string(runes[:i])
		}
	}
	return line
}

// findAssignments finds name← patterns in a line of code and adds names to the set.
// Handles simple assignment (name←), destructuring ((a b c)←), and skips
// system variables (⎕name←), namespace members (name.member←), and
// assignments inside string literals.
func findAssignments(code string, assigned map[string]bool) {
	runes := []rune(code)
	inString := false
	for i, r := range runes {
		if r == '\'' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if r != '←' {
			continue
		}
		// Walk backward to find what's being assigned to
		j := i - 1
		// Skip whitespace before ←
		for j >= 0 && runes[j] == ' ' {
			j--
		}
		if j < 0 {
			continue
		}

		if runes[j] == ')' {
			// Destructuring: (a b c)←
			// Find matching (
			k := j - 1
			for k >= 0 && runes[k] != '(' {
				k--
			}
			if k >= 0 {
				inner := string(runes[k+1 : j])
				for _, name := range strings.Fields(inner) {
					if isValidVarName(name) {
						assigned[name] = true
					}
				}
			}
		} else if isIdentRune(runes[j]) {
			// Simple assignment: name←
			extractAssignTarget(runes, j, assigned)
		} else if !strings.ContainsRune("()[] {}'\"", runes[j]) {
			// Modified assignment: name+← or name×← etc.
			// Skip the operator glyph and look for identifier before it
			k := j - 1
			for k >= 0 && runes[k] == ' ' {
				k--
			}
			if k >= 0 && isIdentRune(runes[k]) {
				extractAssignTarget(runes, k, assigned)
			}
		}
	}
}

// extractAssignTarget walks backward from position j through identifier runes
// to extract a variable name, and adds it to the assigned set if valid.
func extractAssignTarget(runes []rune, j int, assigned map[string]bool) {
	end := j + 1
	for j >= 0 && isIdentRune(runes[j]) {
		j--
	}
	start := j + 1
	name := string(runes[start:end])

	// Skip system variables (⎕name)
	if strings.HasPrefix(name, "⎕") {
		return
	}

	// Skip namespace member assignments: check if preceded by .
	if start > 0 && runes[start-1] == '.' {
		return
	}

	if isValidVarName(name) {
		assigned[name] = true
	}
}

var forPattern = regexp.MustCompile(`(?i)^\s*:For\s+(\S+)\s+:In`)

// extractForVars extracts the loop variable from :For name :In patterns.
func extractForVars(code string) []string {
	m := forPattern.FindStringSubmatch(code)
	if m == nil {
		return nil
	}
	name := m[1]
	if isValidVarName(name) {
		return []string{name}
	}
	return nil
}

// isIdentRune returns true if the rune can be part of an APL identifier.
func isIdentRune(r rune) bool {
	if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
		return true
	}
	// APL special characters that can start identifiers
	if r == '⎕' || r == '⍺' || r == '⍵' || r == '∆' || r == '⍙' {
		return true
	}
	return false
}

// isValidVarName returns true if the string is a valid APL variable name
// (not empty, starts with a letter or APL char, not a system variable).
func isValidVarName(name string) bool {
	if name == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name)
	if r == '⎕' || r == '⍺' || r == '⍵' {
		return false
	}
	return unicode.IsLetter(r) || r == '∆' || r == '⍙'
}

// parseHeader parses a tradfn header line into signature and locals.
// The header format is: signature;local1;local2;... ⍝ optional comment
// Returns the signature part, the list of local names, and any trailing comment.
func parseHeader(header string) (signature string, locals []string, comment string) {
	// Separate trailing comment (matching RIDE: lt.match(/^(.*?)(\s*⍝.*)?$/))
	if idx := strings.Index(header, "⍝"); idx >= 0 {
		comment = header[idx:]
		header = strings.TrimRight(header[:idx], " ")
	}

	parts := strings.Split(header, ";")
	signature = strings.TrimRight(parts[0], " ")

	for i := 1; i < len(parts); i++ {
		local := strings.TrimSpace(parts[i])
		if local != "" {
			locals = append(locals, local)
		}
	}
	return
}

// headerVars extracts variable names from the function signature
// (everything except the function name itself).
// E.g. "r←MyFn x" with fnName="MyFn" returns ["r", "x"].
func headerVars(signature string, fnName string) []string {
	// Extract all identifier-like tokens from the signature
	tokens := splitIdentifiers(signature)

	var vars []string
	for _, tok := range tokens {
		if tok == fnName {
			continue
		}
		vars = append(vars, tok)
	}
	return vars
}

// splitIdentifiers extracts all identifier tokens from a string,
// ignoring punctuation like ← { } ( ).
func splitIdentifiers(s string) []string {
	var result []string
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		if isIdentRune(runes[i]) {
			start := i
			for i < len(runes) && isIdentRune(runes[i]) {
				i++
			}
			tok := string(runes[start:i])
			if tok != "" {
				result = append(result, tok)
			}
		} else {
			i++
		}
	}
	return result
}

var globalsPattern = regexp.MustCompile(`(?i)^\s*⍝\s*GLOBALS:\s*(.*)$`)

// parseGlobalsComment searches lines for a ⍝ GLOBALS: comment and extracts
// the space-separated variable names listed after the colon.
func parseGlobalsComment(lines []string) (globals []string, lineIdx int) {
	for i, line := range lines {
		m := globalsPattern.FindStringSubmatch(line)
		if m != nil {
			names := strings.Fields(m[1])
			if len(names) == 0 {
				return nil, i
			}
			return names, i
		}
	}
	return nil, -1
}

// autolocaliseText analyses a tradfn's text and updates the header to include
// all assigned variables that aren't already localised, in the signature, or
// marked as globals. Existing locals are preserved in order; new ones are
// appended sorted alphabetically.
func autolocaliseText(text []string, fnName string) []string {
	if len(text) == 0 {
		return text
	}

	signature, existingLocals, comment := parseHeader(text[0])
	globals, _ := parseGlobalsComment(text)
	sigVars := headerVars(signature, fnName)

	// Build body lines (everything after header)
	var bodyLines []string
	for i := 1; i < len(text); i++ {
		bodyLines = append(bodyLines, text[i])
	}
	assigned := findAssignedVars(bodyLines)

	// Build exclusion set: signature vars + globals + existing locals
	exclude := make(map[string]bool)
	for _, v := range sigVars {
		exclude[v] = true
	}
	for _, v := range globals {
		exclude[v] = true
	}
	for _, v := range existingLocals {
		exclude[v] = true
	}

	// Find new locals to add
	var newLocals []string
	for _, v := range assigned {
		if !exclude[v] {
			newLocals = append(newLocals, v)
		}
	}

	if len(newLocals) == 0 {
		return text
	}

	// Merge: existing order preserved, new ones appended sorted
	allLocals := append(existingLocals, newLocals...)

	// Rebuild header
	result := make([]string, len(text))
	copy(result, text)
	result[0] = buildHeader(signature, allLocals, comment)
	return result
}

// localiseText is an on-demand cleanup: adds missing locals AND removes stale ones.
// A local is stale if it's not assigned anywhere in the body and not in GLOBALS.
// Returns the modified text.
func localiseText(text []string, fnName string) []string {
	if len(text) == 0 {
		return text
	}

	signature, existingLocals, comment := parseHeader(text[0])
	globals, _ := parseGlobalsComment(text)
	sigVars := headerVars(signature, fnName)

	var bodyLines []string
	for i := 1; i < len(text); i++ {
		bodyLines = append(bodyLines, text[i])
	}
	assigned := findAssignedVars(bodyLines)

	// Build sets for quick lookup
	sigSet := make(map[string]bool)
	for _, v := range sigVars {
		sigSet[v] = true
	}
	globalSet := make(map[string]bool)
	for _, v := range globals {
		globalSet[v] = true
	}
	assignedSet := make(map[string]bool)
	for _, v := range assigned {
		assignedSet[v] = true
	}

	// Keep existing locals that are still assigned and not in globals (preserve order)
	var keptLocals []string
	for _, l := range existingLocals {
		if assignedSet[l] && !globalSet[l] {
			keptLocals = append(keptLocals, l)
		}
	}

	// Add new locals (assigned but not in header signature, globals, or kept locals)
	keptSet := make(map[string]bool)
	for _, l := range keptLocals {
		keptSet[l] = true
	}
	var newLocals []string
	for _, v := range assigned {
		if !sigSet[v] && !globalSet[v] && !keptSet[v] {
			newLocals = append(newLocals, v)
		}
	}

	allLocals := append(keptLocals, newLocals...)

	if len(allLocals) == len(existingLocals) {
		// Check if they're the same (no change)
		same := true
		for i, l := range allLocals {
			if i >= len(existingLocals) || l != existingLocals[i] {
				same = false
				break
			}
		}
		if same {
			return text
		}
	}

	result := make([]string, len(text))
	copy(result, text)
	result[0] = buildHeader(signature, allLocals, comment)
	return result
}

// toggleLocal toggles a variable name in/out of the tradfn header's local list.
// If the variable is already local, it's removed. If not, it's added (sorted).
//
// GLOBALS management:
//   - When removing a local and createGlobals is true: always add to ⍝ GLOBALS:
//     (creates the comment after the header if it doesn't exist)
//   - When removing a local and createGlobals is false: add to ⍝ GLOBALS: only
//     if the comment already exists
//   - When adding a local: remove from ⍝ GLOBALS: if present
func toggleLocal(text []string, fnName string, varName string, createGlobals bool) []string {
	if len(text) == 0 || varName == "" {
		return text
	}

	signature, locals, comment := parseHeader(text[0])

	// Check if already in locals
	idx := -1
	for i, l := range locals {
		if l == varName {
			idx = i
			break
		}
	}

	result := make([]string, len(text))
	copy(result, text)

	if idx >= 0 {
		// Removing from locals
		locals = append(locals[:idx], locals[idx+1:]...)
		result[0] = buildHeader(signature, locals, comment)
		result = addToGlobals(result, varName, createGlobals)
	} else {
		// Adding to locals (sorted, matching RIDE's TL behavior)
		locals = append(locals, varName)
		sort.Strings(locals)
		result[0] = buildHeader(signature, locals, comment)
		result = removeFromGlobals(result, varName)
	}

	return result
}

// addToGlobals adds a variable name to the ⍝ GLOBALS: comment.
// If createIfMissing is true, creates the comment on line 1 (after header).
// If createIfMissing is false, only adds if the comment already exists.
func addToGlobals(text []string, varName string, createIfMissing bool) []string {
	globals, lineIdx := parseGlobalsComment(text)

	if lineIdx < 0 {
		// No GLOBALS comment exists
		if !createIfMissing {
			return text
		}
		// Insert ⍝ GLOBALS: varName after header (line 1)
		newLine := "⍝ GLOBALS: " + varName
		result := make([]string, 0, len(text)+1)
		result = append(result, text[0])
		result = append(result, newLine)
		result = append(result, text[1:]...)
		return result
	}

	// Check if already in globals
	for _, g := range globals {
		if g == varName {
			return text // already there
		}
	}

	// Add to existing GLOBALS comment
	globals = append(globals, varName)
	sort.Strings(globals)
	text[lineIdx] = "⍝ GLOBALS: " + strings.Join(globals, " ")
	return text
}

// removeFromGlobals removes a variable name from the ⍝ GLOBALS: comment if present.
// An empty ⍝ GLOBALS: line is kept as a signal that the user wants the mechanism.
func removeFromGlobals(text []string, varName string) []string {
	globals, lineIdx := parseGlobalsComment(text)
	if lineIdx < 0 {
		return text
	}

	// Find and remove
	idx := -1
	for i, g := range globals {
		if g == varName {
			idx = i
			break
		}
	}
	if idx < 0 {
		return text // not in globals
	}

	globals = append(globals[:idx], globals[idx+1:]...)
	if len(globals) == 0 {
		text[lineIdx] = "⍝ GLOBALS:"
	} else {
		text[lineIdx] = "⍝ GLOBALS: " + strings.Join(globals, " ")
	}
	return text
}

// buildHeader reconstructs a tradfn header from signature, locals, and comment.
func buildHeader(signature string, locals []string, comment string) string {
	var b strings.Builder
	b.WriteString(strings.TrimRight(signature, " "))
	for _, l := range locals {
		b.WriteByte(';')
		b.WriteString(l)
	}
	if comment != "" {
		b.WriteByte(' ')
		b.WriteString(comment)
	}
	return b.String()
}
