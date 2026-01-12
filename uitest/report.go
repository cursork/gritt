package uitest

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Report collects test results and generates HTML
type Report struct {
	Timestamp  string
	Tests      []TestResult
	Snapshots  []Snapshot
	OutputDir  string
	currentIdx int
}

// TestResult represents a single test result
type TestResult struct {
	Name   string
	Passed bool
}

// Snapshot represents a captured screen state
type Snapshot struct {
	Label   string
	Content string
}

// NewReport creates a new test report
func NewReport(outputDir string) *Report {
	return &Report{
		Timestamp: time.Now().Format("20060102-150405"),
		OutputDir: outputDir,
	}
}

// AddResult adds a test result
func (r *Report) AddResult(name string, passed bool) {
	r.Tests = append(r.Tests, TestResult{Name: name, Passed: passed})
}

// AddSnapshot adds a screen snapshot
func (r *Report) AddSnapshot(label string, content string) {
	r.Snapshots = append(r.Snapshots, Snapshot{Label: label, Content: content})
}

// Passed returns count of passed tests
func (r *Report) Passed() int {
	count := 0
	for _, t := range r.Tests {
		if t.Passed {
			count++
		}
	}
	return count
}

// Failed returns count of failed tests
func (r *Report) Failed() int {
	return len(r.Tests) - r.Passed()
}

// ansiToHTML converts ANSI escape codes to HTML spans
func ansiToHTML(s string) string {
	// First escape HTML special chars (but not in a way that breaks our processing)
	s = html.EscapeString(s)

	// 256-color foreground: \x1b[38;5;Nm
	re256 := regexp.MustCompile(`\x1b\[38;5;(\d+)m`)
	s = re256.ReplaceAllStringFunc(s, func(match string) string {
		matches := re256.FindStringSubmatch(match)
		if len(matches) < 2 {
			return ""
		}
		colorNum, _ := strconv.Atoi(matches[1])
		hexColor := ansi256ToHex(colorNum)
		return fmt.Sprintf(`<span style="color:%s">`, hexColor)
	})

	// Bold: \x1b[1m
	s = strings.ReplaceAll(s, "\x1b[1m", `<span style="font-weight:bold">`)

	// Reset: \x1b[0m or \x1b[m
	s = strings.ReplaceAll(s, "\x1b[0m", `</span>`)
	s = strings.ReplaceAll(s, "\x1b[m", `</span>`)

	// Remove any remaining ANSI codes
	reAny := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	s = reAny.ReplaceAllString(s, "")

	return s
}

// ansi256ToHex converts 256-color ANSI code to hex
func ansi256ToHex(n int) string {
	// Standard colors 0-15
	standard := []string{
		"#000000", "#800000", "#008000", "#808000", "#000080", "#800080", "#008080", "#c0c0c0",
		"#808080", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", "#ffffff",
	}
	if n < 16 {
		return standard[n]
	}

	// 216 color cube (16-231)
	if n < 232 {
		n -= 16
		r := (n / 36) * 51
		g := ((n / 6) % 6) * 51
		b := (n % 6) * 51
		return fmt.Sprintf("#%02x%02x%02x", r, g, b)
	}

	// Grayscale (232-255)
	gray := (n-232)*10 + 8
	return fmt.Sprintf("#%02x%02x%02x", gray, gray, gray)
}

// Generate writes the HTML report to disk
func (r *Report) Generate() (string, error) {
	if err := os.MkdirAll(r.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output dir: %w", err)
	}

	filename := filepath.Join(r.OutputDir, fmt.Sprintf("test-%s.html", r.Timestamp))

	statusClass := "pass"
	statusText := "All tests passed"
	if r.Failed() > 0 {
		statusClass = "fail"
		statusText = fmt.Sprintf("%d test(s) failed", r.Failed())
	}

	var snapshots strings.Builder
	for _, snap := range r.Snapshots {
		snapshots.WriteString(fmt.Sprintf(`<div class="snapshot">
<h3>%s</h3>
<pre>%s</pre>
</div>
`, html.EscapeString(snap.Label), ansiToHTML(snap.Content)))
	}

	for _, t := range r.Tests {
		class := "pass"
		symbol := "✓"
		if !t.Passed {
			class = "fail"
			symbol = "✗"
		}
		snapshots.WriteString(fmt.Sprintf(`<div class="result %s">%s %s</div>
`, class, symbol, html.EscapeString(t.Name)))
	}

	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>gritt test report - %s</title>
    <style>
        body {
            font-family: 'SF Mono', 'Menlo', 'Monaco', 'Cascadia Code', 'Consolas', 'DejaVu Sans Mono', -apple-system, monospace;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background: #1a1a2e;
            color: #eee;
        }
        h1 { color: #00d9ff; }
        h2 { color: #888; border-bottom: 1px solid #333; padding-bottom: 10px; }
        h3 { color: #aaa; margin: 10px 0 5px 0; }
        .summary {
            background: #252540;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
        }
        .summary.pass { border-left: 4px solid #00ff88; }
        .summary.fail { border-left: 4px solid #ff4444; }
        .stats { display: flex; gap: 30px; margin-top: 15px; }
        .stat { text-align: center; }
        .stat-value { font-size: 2em; font-weight: bold; }
        .stat-label { color: #888; font-size: 0.9em; }
        .stat-value.pass { color: #00ff88; }
        .stat-value.fail { color: #ff4444; }
        .result {
            padding: 8px 15px;
            margin: 5px 0;
            border-radius: 4px;
        }
        .result.pass { background: #1a3d2a; color: #00ff88; }
        .result.fail { background: #3d1a1a; color: #ff4444; }
        .snapshot {
            margin: 20px 0;
            background: #252540;
            border-radius: 8px;
            overflow: hidden;
        }
        .snapshot h3 {
            background: #1a1a2e;
            margin: 0;
            padding: 10px 15px;
        }
        .snapshot pre {
            margin: 0;
            padding: 15px;
            overflow-x: auto;
            font-size: 14px;
            line-height: 1.2;
            background: #0a0a15;
            color: #00d9ff;
            font-family: 'SF Mono', 'Menlo', 'Monaco', 'Cascadia Code', 'Consolas', 'DejaVu Sans Mono', monospace;
        }
        .timestamp { color: #666; font-size: 0.9em; }
    </style>
</head>
<body>
    <h1>gritt test report</h1>
    <p class="timestamp">%s</p>

    <div class="summary %s">
        <strong>%s</strong>
        <div class="stats">
            <div class="stat">
                <div class="stat-value">%d</div>
                <div class="stat-label">Total</div>
            </div>
            <div class="stat">
                <div class="stat-value pass">%d</div>
                <div class="stat-label">Passed</div>
            </div>
            <div class="stat">
                <div class="stat-value fail">%d</div>
                <div class="stat-label">Failed</div>
            </div>
        </div>
    </div>

    <h2>Test Progress</h2>
    %s
</body>
</html>
`, r.Timestamp, r.Timestamp, statusClass, statusText, len(r.Tests), r.Passed(), r.Failed(), snapshots.String())

	if err := os.WriteFile(filename, []byte(htmlContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write report: %w", err)
	}

	return filename, nil
}
