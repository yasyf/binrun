package cli_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yasyf/daemonkit/artifact"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "binrun-bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binPath = filepath.Join(dir, "binrun")
	if out, err := exec.Command("go", "build", "-o", binPath, "github.com/yasyf/binrun/cmd/binrun").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build binrun: %v\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// TestExitCodesArtifactPassthrough proves a resolved artifact's own exit code
// becomes binrun's, for both success and failure codes.
func TestExitCodesArtifactPassthrough(t *testing.T) {
	tests := []struct {
		name string
		code int
	}{
		{"success passes through", 0},
		{"failure passes through", 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			script := []byte(fmt.Sprintf("#!/bin/sh\nexit %d\n", tt.code))
			descPath := seedReleaseDescriptor(t, home, script, "runme")
			if got := runBinrun(t, home, descPath); got != tt.code {
				t.Errorf("exit = %d, want %d", got, tt.code)
			}
		})
	}
}

// TestExitCodesRunnerFailures proves every runner-domain failure exits 1 and
// never 2 (2 is reserved for hook verdicts).
func TestExitCodesRunnerFailures(t *testing.T) {
	tests := []struct {
		name    string
		body    string // descriptor body; empty means "point at a missing file"
		wantMsg string
	}{
		{"missing descriptor file", "", "read descriptor"},
		{"schema too new", `{"schema":2,"name":"demo","kind":"python-tool","version":{"static":"1.0.0"},"tool":{"dist":"demo"}}`, "upgrade binrun"},
		{"unsupported platform", `{"schema":1,"name":"demo","kind":"release-binary","version":{"static":"1.0.0"},"platforms":{"plan9-sparc":{"size":1,"hash":"sha256","digest":"` + strings.Repeat("a", 64) + `","path":"x","providers":[{"type":"github-release","repo":"yasyf/demo","tag":"v1.0.0","name":"demo"}]}}}`, "no artifact for this platform"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			descPath := filepath.Join(home, "missing.binrun")
			if tt.body != "" {
				descPath = filepath.Join(home, "app.binrun")
				if err := os.WriteFile(descPath, []byte(tt.body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			code, stderr := runBinrunErr(t, home, descPath)
			if code != 1 {
				t.Errorf("exit = %d, want 1 (never 2)", code)
			}
			if !strings.Contains(stderr, tt.wantMsg) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, tt.wantMsg)
			}
		})
	}
}

// TestVerbsWriteMachineOutputToStdout guards the cobra gotcha that cmd.Println
// writes to stderr: a verb's machine-readable value must land on stdout so
// `$(binrun -- resolve FILE)` and pre-warm scripts capture it, with stderr clean.
func TestVerbsWriteMachineOutputToStdout(t *testing.T) {
	home := t.TempDir()

	t.Run("cache-dir", func(t *testing.T) {
		stdout, stderr, code := runBinrunCapture(t, home, "--", "cache-dir")
		if code != 0 {
			t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
		}
		if want := filepath.Join(home, ".daemonkit", "cache") + "\n"; stdout != want {
			t.Errorf("stdout = %q, want %q", stdout, want)
		}
		if stderr != "" {
			t.Errorf("stderr = %q, want empty", stderr)
		}
	})

	t.Run("resolve", func(t *testing.T) {
		descPath := seedReleaseDescriptor(t, home, []byte("#!/bin/sh\nexit 0\n"), "runme")
		stdout, stderr, code := runBinrunCapture(t, home, "--", "resolve", descPath)
		if code != 0 {
			t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
		}
		if !strings.HasSuffix(strings.TrimSpace(stdout), "/runme") {
			t.Errorf("stdout = %q, want a path ending in /runme", stdout)
		}
		if stderr != "" {
			t.Errorf("stderr = %q, want empty", stderr)
		}
	})
}

func runBinrunCapture(t *testing.T, home string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}
	var out, errBuf strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err == nil {
		return out.String(), errBuf.String(), 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return out.String(), errBuf.String(), ee.ExitCode()
	}
	t.Fatalf("run binrun: %v", err)
	return "", "", -1
}

func seedReleaseDescriptor(t *testing.T, home string, content []byte, entryPath string) string {
	t.Helper()
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	dir := filepath.Join(home, ".daemonkit", "cache", digest[:2], digest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, entryPath), content, 0o755); err != nil {
		t.Fatal(err)
	}
	platform, err := artifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	desc := map[string]any{
		"schema": 1, "name": "demo", "kind": "release-binary",
		"version": map[string]any{"static": "1.0.0"},
		"platforms": map[string]any{
			string(platform): map[string]any{
				"size": len(content), "hash": "sha256", "digest": digest, "path": entryPath,
				"providers": []any{map[string]any{"type": "github-release", "repo": "yasyf/demo", "tag": "v1.0.0", "name": "demo.tar.gz"}},
			},
		},
	}
	data, err := json.Marshal(desc)
	if err != nil {
		t.Fatal(err)
	}
	descPath := filepath.Join(home, "app.binrun")
	if err := os.WriteFile(descPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return descPath
}

func runBinrun(t *testing.T, home, descPath string) int {
	t.Helper()
	code, _ := runBinrunErr(t, home, descPath)
	return code
}

func runBinrunErr(t *testing.T, home, descPath string) (int, string) {
	t.Helper()
	cmd := exec.Command(binPath, descPath)
	cmd.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, stderr.String()
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), stderr.String()
	}
	t.Fatalf("run binrun: %v", err)
	return -1, ""
}
