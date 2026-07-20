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
	unicode bool // true = Unicode edition, false = Classic
	bits    int  // 64 or 32; 0 if unknown (Darwin/Linux don't distinguish)
}

// compactVersionRe matches the compact version format "210U64": two-digit
// major, one-digit minor, edition (C=Classic, U=Unicode), bits (32 or 64).
var compactVersionRe = regexp.MustCompile(`^(\d{2})(\d)([CU])(32|64)$`)

// versionSpec is the parsed form of a -version argument.
type versionSpec struct {
	version string // dotted "X.Y", or "" for any/auto-discover
	kind    string // "C", "U", or "" for unspecified (defaults to Unicode)
	bits    int    // 32, 64, or 0 for unspecified (no filtering)
}

// parseVersionArg parses a -version argument. The compact format
// "210U64" (version+edition+bits) is recognised; anything else is passed
// through unchanged as a plain dotted version, matching prior behaviour.
func parseVersionArg(s string) versionSpec {
	if m := compactVersionRe.FindStringSubmatch(s); m != nil {
		bits, _ := strconv.Atoi(m[4])
		return versionSpec{version: m[1] + "." + m[2], kind: m[3], bits: bits}
	}
	return versionSpec{version: s}
}

// matches reports whether inst satisfies the spec's edition/bits constraints.
// Edition defaults to Unicode when unspecified, preserving the historical
// Unicode-only behaviour for plain "X.Y" version strings. Bits are only
// filtered when both the spec and the install report a known value.
func (s versionSpec) matches(inst dyalogInstall) bool {
	wantKind := s.kind
	if wantKind == "" {
		wantKind = "U"
	}
	if wantKind == "U" && !inst.unicode {
		return false
	}
	if wantKind == "C" && inst.unicode {
		return false
	}
	if s.bits != 0 && inst.bits != 0 && inst.bits != s.bits {
		return false
	}
	return true
}

// FindDyalog discovers installed Dyalog interpreters and returns the path
// to the best match.
//
// If version contains a path separator, it is treated as a direct path to
// the binary. If version is "X.Y", only that (Unicode) version is returned.
// The compact format "210U64" (version, edition C/U, bits 32/64) additionally
// selects Classic builds and a specific bitness — see parseVersionArg. If
// version is empty, the highest installed Unicode version is returned
// (checking PATH first).
func FindDyalog(version string) (string, error) {
	return findDyalog(version, false)
}

// FindDyalogBinary returns the path to the actual Dyalog binary, never a
// wrapper script (e.g. macOS `mapl` symlinked from /usr/local/bin/dyalog).
// Use this when the caller needs direct process control over the interpreter
// — without it, signals target the wrapper and `cmd.Wait()` reaps the wrong
// process while the real interpreter is orphaned to init.
func FindDyalogBinary(version string) (string, error) {
	return findDyalog(version, true)
}

func findDyalog(version string, skipPath bool) (string, error) {
	// Direct path (contains / or \)
	if strings.ContainsAny(version, `/\`) {
		if _, err := os.Stat(version); err != nil {
			return "", fmt.Errorf("dyalog binary not found: %s", version)
		}
		return version, nil
	}

	// If no version requested, try PATH first (unless explicitly skipped).
	if version == "" && !skipPath {
		if path, err := exec.LookPath("dyalog"); err == nil {
			return path, nil
		}
	}

	spec := parseVersionArg(version)

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
	if spec.version != "" {
		for _, inst := range installs {
			if inst.version == spec.version && spec.matches(inst) {
				return inst.path, nil
			}
		}
		return "", fmt.Errorf("Dyalog version %s not found (available: %s).\nSearched:\n  %s",
			version, availableVersions(installs), SearchedPaths())
	}

	for _, inst := range installs {
		if spec.matches(inst) {
			return inst.path, nil
		}
	}
	return "", fmt.Errorf("Dyalog not found matching %q.\nSearched:\n  %s", version, SearchedPaths())
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
			unicode: true, // macOS only ships Unicode builds
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
		for _, bitsStr := range []string{"64", "32"} {
			exe := filepath.Join("/opt/mdyalog", ver, bitsStr, "unicode", "dyalog")
			if _, err := os.Stat(exe); err == nil {
				bits, _ := strconv.Atoi(bitsStr)
				installs = append(installs, dyalogInstall{
					path:    exe,
					version: ver,
					major:   major,
					minor:   minor,
					unicode: true, // only unicode builds are discovered on Linux
					bits:    bits,
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

	// Captures: [1] "-64" or empty (bits), [2] version, [3] Classic or Unicode
	re := regexp.MustCompile(`^Dyalog APL(-64)? (\d+\.\d+) (Classic|Unicode)$`)

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
			ver := m[2]
			exe := filepath.Join(dir, entry.Name(), "dyalog.exe")
			if _, err := os.Stat(exe); err != nil {
				continue
			}
			major, minor, ok := parseVersion(ver)
			if !ok {
				continue
			}
			bits := 32
			if m[1] == "-64" {
				bits = 64
			}
			installs = append(installs, dyalogInstall{
				path:    exe,
				version: ver,
				major:   major,
				minor:   minor,
				unicode: m[3] == "Unicode",
				bits:    bits,
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
