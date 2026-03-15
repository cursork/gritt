package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type dyalogInstall struct {
	path    string
	version string
	major   int
	minor   int
}

// FindDyalog discovers installed Dyalog interpreters and returns the path
// to the best match.
//
// If version contains a path separator, it is treated as a direct path to
// the binary. If version is "X.Y", only that version is returned. If version
// is empty, the highest installed version is returned (checking PATH first).
func FindDyalog(version string) (string, error) {
	// Direct path (contains / or \)
	if strings.ContainsAny(version, `/\`) {
		if _, err := os.Stat(version); err != nil {
			return "", fmt.Errorf("dyalog binary not found: %s", version)
		}
		return version, nil
	}

	// If no version requested, try PATH first
	if version == "" {
		if path, err := exec.LookPath("dyalog"); err == nil {
			return path, nil
		}
	}

	// Discovery
	var installs []dyalogInstall

	switch runtime.GOOS {
	case "darwin":
		installs = findDyalogDarwin()
	case "linux":
		installs = findDyalogLinux()
	case "windows":
		installs = findDyalogWindows()
	}

	if len(installs) == 0 {
		if version != "" {
			return "", fmt.Errorf("Dyalog version %s not found.\nSearched:\n  %s", version, SearchedPaths())
		}
		return "", fmt.Errorf("Dyalog not found in PATH or standard install locations.\nSearched:\n  %s\n  %s",
			"$PATH", SearchedPaths())
	}

	// Sort by version descending (highest first)
	sort.Slice(installs, func(i, j int) bool {
		if installs[i].major != installs[j].major {
			return installs[i].major > installs[j].major
		}
		return installs[i].minor > installs[j].minor
	})

	// Filter by version if requested
	if version != "" {
		for _, inst := range installs {
			if inst.version == version {
				return inst.path, nil
			}
		}
		return "", fmt.Errorf("Dyalog version %s not found (available: %s).\nSearched:\n  %s",
			version, availableVersions(installs), SearchedPaths())
	}

	return installs[0].path, nil
}

// SearchedPaths returns a human-readable list of paths that were searched,
// for error messages.
func SearchedPaths() string {
	switch runtime.GOOS {
	case "darwin":
		return "/Applications/Dyalog-*.app/Contents/Resources/Dyalog/dyalog"
	case "linux":
		return "/opt/mdyalog/<version>/{64,32}/unicode/dyalog"
	case "windows":
		paths := []string{
			`C:\Program Files\Dyalog\Dyalog APL-64 * Unicode\dyalog.exe`,
			`C:\Program Files (x86)\Dyalog\Dyalog APL * Unicode\dyalog.exe`,
		}
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			paths = append(paths, filepath.Join(localAppData, `Programs\Dyalog\Dyalog APL-64 * Unicode\dyalog.exe`))
		}
		return strings.Join(paths, "\n  ")
	default:
		return "(unsupported platform)"
	}
}

func availableVersions(installs []dyalogInstall) string {
	var versions []string
	for _, inst := range installs {
		versions = append(versions, inst.version)
	}
	return strings.Join(versions, ", ")
}

var versionRe = regexp.MustCompile(`(\d+)\.(\d+)`)

func parseVersion(s string) (major, minor int, ok bool) {
	m := versionRe.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, false
	}
	major, _ = strconv.Atoi(m[1])
	minor, _ = strconv.Atoi(m[2])
	return major, minor, true
}

func findDyalogDarwin() []dyalogInstall {
	var installs []dyalogInstall

	entries, err := os.ReadDir("/Applications")
	if err != nil {
		return nil
	}

	re := regexp.MustCompile(`^Dyalog-(\d+\.\d+)\.app$`)
	for _, entry := range entries {
		m := re.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		ver := m[1]
		exe := filepath.Join("/Applications", entry.Name(), "Contents/Resources/Dyalog/dyalog")
		if _, err := os.Stat(exe); err != nil {
			continue
		}
		major, minor, ok := parseVersion(ver)
		if !ok {
			continue
		}
		installs = append(installs, dyalogInstall{
			path:    exe,
			version: ver,
			major:   major,
			minor:   minor,
		})
	}

	return installs
}

func findDyalogLinux() []dyalogInstall {
	var installs []dyalogInstall

	entries, err := os.ReadDir("/opt/mdyalog")
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ver := entry.Name()
		major, minor, ok := parseVersion(ver)
		if !ok {
			continue
		}

		// Prefer 64-bit unicode, fall back to 32-bit unicode
		for _, bits := range []string{"64", "32"} {
			exe := filepath.Join("/opt/mdyalog", ver, bits, "unicode", "dyalog")
			if _, err := os.Stat(exe); err == nil {
				installs = append(installs, dyalogInstall{
					path:    exe,
					version: ver,
					major:   major,
					minor:   minor,
				})
				break
			}
		}
	}

	return installs
}

func findDyalogWindows() []dyalogInstall {
	var installs []dyalogInstall

	searchDirs := []string{
		`C:\Program Files\Dyalog`,
		`C:\Program Files (x86)\Dyalog`,
	}
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		searchDirs = append(searchDirs, filepath.Join(localAppData, "Programs", "Dyalog"))
	}

	re := regexp.MustCompile(`^Dyalog APL(?:-64)? (\d+\.\d+) Unicode$`)

	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			m := re.FindStringSubmatch(entry.Name())
			if m == nil {
				continue
			}
			ver := m[1]
			exe := filepath.Join(dir, entry.Name(), "dyalog.exe")
			if _, err := os.Stat(exe); err != nil {
				continue
			}
			major, minor, ok := parseVersion(ver)
			if !ok {
				continue
			}
			installs = append(installs, dyalogInstall{
				path:    exe,
				version: ver,
				major:   major,
				minor:   minor,
			})
		}
	}

	return installs
}

// DyalogEnv returns environment variables needed to run a discovered Dyalog binary.
func DyalogEnv(dyalogPath string) []string {
	dir := filepath.Dir(dyalogPath)
	env := []string{fmt.Sprintf("DYALOG=%s", dir)}

	if runtime.GOOS == "linux" {
		ldPath := os.Getenv("LD_LIBRARY_PATH")
		if ldPath == "" {
			env = append(env, fmt.Sprintf("LD_LIBRARY_PATH=%s", dir))
		} else {
			env = append(env, fmt.Sprintf("LD_LIBRARY_PATH=%s:%s", dir, ldPath))
		}
	}

	return env
}
