package cwdguard_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/yasyf/binrun/internal/cwdguard"
)

const helperEnv = "CWDGUARD_HELPER"

func TestHelperReportsTheGuardsView(t *testing.T) {
	if os.Getenv(helperEnv) == "" {
		t.Skip("subprocess helper")
	}
	wd, err := os.Getwd()
	if err != nil {
		wd = "error: " + err.Error()
	}
	if err := cwdguard.Return(); err != nil {
		t.Fatal(err)
	}
	var here syscall.Stat_t
	if err := syscall.Stat(".", &here); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("departed=%t wd=%s ino=%d\n", cwdguard.Departed, wd, here.Ino)
}

func TestInitLeavesARemovedWorkingDirectoryBeforeOS(t *testing.T) {
	for _, tc := range []struct {
		name     string
		remove   bool
		departed bool
	}{
		{"removed", true, true},
		{"live", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "gone")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			var origin syscall.Stat_t
			if err := syscall.Stat(dir, &origin); err != nil {
				t.Fatal(err)
			}
			t.Chdir(dir)
			if tc.remove {
				if err := os.Remove(dir); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestHelperReportsTheGuardsView$")
			cmd.Env = append(os.Environ(), helperEnv+"=1", "GODEBUG=inittrace=1", "PWD="+dir)
			var stderr strings.Builder
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("helper: %v\n%s", err, stderr.String())
			}
			guard := strings.Index(stderr.String(), "init github.com/yasyf/binrun/internal/cwdguard ")
			osInit := strings.Index(stderr.String(), "init os ")
			if guard < 0 || osInit < 0 || guard > osInit {
				t.Fatalf("init order: guard at %d, os at %d\n%s", guard, osInit, stderr.String())
			}
			wantWD := dir
			if tc.departed {
				wantWD = "/"
			}
			want := fmt.Sprintf("departed=%t wd=%s ino=%d\n", tc.departed, wantWD, origin.Ino)
			if !strings.Contains(string(out), want) {
				t.Fatalf("helper reported %q, want %q", out, want)
			}
		})
	}
}
