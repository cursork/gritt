package main

import (
	"os/exec"
	"strconv"
	"strings"
)

// procEntry is one node in a process tree, depth-first ordered with the
// root at depth 0.
type procEntry struct {
	pid   int
	cmd   string
	depth int
}

// processTree returns the root process plus all its descendants (DFS).
// Implemented via `ps -A -o pid=,ppid=,comm=` so it works on darwin and
// linux without extra deps. Returns nil on Windows or when ps fails.
func processTree(root int) []procEntry {
	out, err := exec.Command("ps", "-A", "-o", "pid=,ppid=,comm=").Output()
	if err != nil {
		return nil
	}

	type record struct {
		pid, ppid int
		cmd       string
	}
	var procs []record
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			continue
		}
		procs = append(procs, record{pid, ppid, strings.Join(fields[2:], " ")})
	}

	children := map[int][]record{}
	var rootRec *record
	for i := range procs {
		p := &procs[i]
		if p.pid == root {
			rootRec = p
		}
		children[p.ppid] = append(children[p.ppid], *p)
	}
	if rootRec == nil {
		return nil
	}

	var out2 []procEntry
	var walk func(r record, depth int)
	walk = func(r record, depth int) {
		out2 = append(out2, procEntry{pid: r.pid, cmd: r.cmd, depth: depth})
		for _, c := range children[r.pid] {
			walk(c, depth+1)
		}
	}
	walk(*rootRec, 0)
	return out2
}
