package livedir

import (
	"errors"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
)

const (
	envDir     = "/Users/u/.daemonkit/tools/capt-hook/12.28.0"
	siblingDir = "/Users/u/.daemonkit/tools/capt-hook/12.28.01"
	staleDir   = "/Users/u/.daemonkit/tools/capt-hook/12.21.3"
)

func fixed(procs ...Process) Lister {
	return func() ([]Process, error) { return procs, nil }
}

func TestInUse(t *testing.T) {
	tests := []struct {
		name  string
		procs []Process
		want  []string
	}{
		{
			name:  "argv0 under the env",
			procs: []Process{{PID: 1, Args: []string{envDir + "/lib/python3.13/site-packages/../../../bin/python", "-c", "x"}}},
			want:  []string{envDir},
		},
		{
			name:  "module path under the env, executable elsewhere",
			procs: []Process{{PID: 2, Args: []string{"/Users/u/.local/share/uv/python/cpython-3.13/bin/python3", "-m", envDir + "/bin/capt_hookd"}}},
			want:  []string{envDir},
		},
		{
			name:  "the env dir itself as an argument",
			procs: []Process{{PID: 3, Args: []string{"/bin/ls", envDir}}},
			want:  []string{envDir},
		},
		{
			name:  "embedded in a flag value",
			procs: []Process{{PID: 4, Args: []string{"/bin/sh", "-c", "exec --root=" + staleDir + "/bin/hook serve"}}},
			want:  []string{staleDir},
		},
		{
			name:  "quoted in a shell command line",
			procs: []Process{{PID: 10, Args: []string{"/bin/sh", "-c", "cd '" + envDir + "'; ./bin/python -m worker"}}},
			want:  []string{envDir},
		},
		{
			name:  "followed by whitespace in a shell command line",
			procs: []Process{{PID: 11, Args: []string{"/bin/sh", "-c", "PYTHONHOME=" + staleDir + " exec python3"}}},
			want:  []string{staleDir},
		},
		{
			name:  "a longer version does not hold the shorter one open",
			procs: []Process{{PID: 5, Args: []string{siblingDir + "/bin/python"}}},
			want:  []string{siblingDir},
		},
		{
			name:  "unrelated processes hold nothing",
			procs: []Process{{PID: 6, Args: []string{"/usr/bin/vim", "notes.md"}}, {PID: 7, Args: nil}},
			want:  nil,
		},
		{
			name: "every live env is reported",
			procs: []Process{
				{PID: 8, Args: []string{envDir + "/bin/python"}},
				{PID: 9, Args: []string{staleDir + "/bin/python"}},
			},
			want: []string{envDir, staleDir},
		},
	}
	dirs := []string{envDir, siblingDir, staleDir}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			live, err := InUse(fixed(tt.procs...), dirs)
			if err != nil {
				t.Fatalf("InUse() = %v", err)
			}
			got := slices.Sorted(maps.Keys(live))
			want := slices.Sorted(slices.Values(tt.want))
			if !slices.Equal(got, want) {
				t.Errorf("InUse() = %v, want %v", got, want)
			}
		})
	}
}

func TestInUseWithoutCandidatesNeverLists(t *testing.T) {
	list := func() ([]Process, error) { t.Fatal("listed processes with no candidate directories"); return nil, nil }
	live, err := InUse(list, nil)
	if live != nil || err != nil {
		t.Errorf("InUse(nil dirs) = %v, %v; want nil, nil", live, err)
	}
}

func TestInUsePropagatesListerFailure(t *testing.T) {
	boom := errors.New("boom")
	list := func() ([]Process, error) { return nil, boom }
	if _, err := InUse(list, []string{envDir}); !errors.Is(err, boom) {
		t.Errorf("InUse() = %v, want %v", err, boom)
	}
}

func TestListReportsThisProcess(t *testing.T) {
	procs, err := List()
	if err != nil {
		t.Fatalf("List() = %v", err)
	}
	self, ok := findPID(procs, os.Getpid())
	if !ok {
		t.Fatalf("List() returned %d processes, none of them this one (pid %d)", len(procs), os.Getpid())
	}
	if len(self.Args) == 0 {
		t.Fatal("List() reported this process with no argv")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := exe[:strings.LastIndexByte(exe, '/')]
	live, err := InUse(fixed(procs...), []string{dir})
	if err != nil {
		t.Fatalf("InUse() = %v", err)
	}
	if !live[dir] {
		t.Errorf("InUse() did not see this process running out of %q; argv = %q", dir, self.Args)
	}
}

func findPID(procs []Process, pid int) (Process, bool) {
	for _, proc := range procs {
		if proc.PID == pid {
			return proc, true
		}
	}
	return Process{}, false
}
