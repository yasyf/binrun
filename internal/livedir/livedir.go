// Package livedir reports which directories a running process is executing out
// of, judged from process argv rather than from the executable the kernel
// reports.
package livedir

import (
	"path/filepath"
	"strings"
)

// Process is one running process and the argv the kernel holds for it.
type Process struct {
	PID  int
	Args []string
}

// Lister enumerates the processes whose argv this user may read.
type Lister func() ([]Process, error)

// InUse reports which of dirs a live process names in its argv. A uv-installed
// python worker runs the interpreter uv manages outside the environment it
// imports from, so the environment's directory reaches the kernel only through
// argv — argv[0] or a "-m" module path — never through the executable.
func InUse(list Lister, dirs []string) (map[string]bool, error) {
	if len(dirs) == 0 {
		return nil, nil
	}
	procs, err := list()
	if err != nil {
		return nil, err
	}
	live := make(map[string]bool, len(dirs))
	for _, proc := range procs {
		for _, arg := range proc.Args {
			for _, dir := range dirs {
				if !live[dir] && under(arg, dir) {
					live[dir] = true
				}
			}
		}
	}
	return live, nil
}

// under reports whether arg names dir or a path inside it. An argument carries
// more than a bare path — "--config=/a/b", a module path, a whole `sh -c`
// command line — so the match is a substring one, ending where a path component
// can, which keeps an env at ".../12.28.0" clear of a worker in ".../12.28.01".
func under(arg, dir string) bool {
	for i := 0; ; {
		j := strings.Index(arg[i:], dir)
		if j < 0 {
			return false
		}
		if end := i + j + len(dir); end == len(arg) || ends(arg[end]) {
			return true
		}
		i += j + 1
	}
}

func ends(b byte) bool {
	return b == filepath.Separator || strings.IndexByte(" \t\n\"'`,;:)]}&|<>", b) >= 0
}
