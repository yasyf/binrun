package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/yasyf/daemonkit/artifact"
	"github.com/yasyf/daemonkit/ghrelease"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want route
	}{
		{"descriptor with args", []string{"app.binrun", "a", "b"}, route{mode: modeExec, descriptor: "app.binrun", args: []string{"a", "b"}}},
		{"shebang absolute path", []string{"/opt/app.binrun"}, route{mode: modeExec, descriptor: "/opt/app.binrun"}},
		{"double dash selects verbs", []string{"--", "fetch", "f"}, route{mode: modeVerbs, args: []string{"fetch", "f"}}},
		{"version flag is runner meta", []string{"--version"}, route{mode: modeVerbs, args: []string{"--version"}}},
		{"short version flag normalizes", []string{"-v"}, route{mode: modeVerbs, args: []string{"--version"}}},
		{"help flag is runner meta", []string{"--help"}, route{mode: modeVerbs, args: []string{"--help"}}},
		{"short help flag normalizes", []string{"-h"}, route{mode: modeVerbs, args: []string{"--help"}}},
		{"leading-dash path is a descriptor", []string{"-weird.binrun"}, route{mode: modeExec, descriptor: "-weird.binrun"}},
		{"no args is a usage error", nil, route{mode: modeUsage}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classify(tt.args)
			if got.mode != tt.want.mode || got.descriptor != tt.want.descriptor || !slices.Equal(got.args, tt.want.args) {
				t.Errorf("classify(%q) = %+v, want %+v", tt.args, got, tt.want)
			}
		})
	}
}

func TestMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"schema", fmt.Errorf("wrap: %w", artifact.ErrSchemaVersion), "descriptor needs a newer binrun; upgrade binrun (brew upgrade binrun)"},
		{"dynamic integrity", artifact.ErrDynamicIntegrity, "dynamic version requires an independent integrity gate (python-tool or signed-app only)"},
		{"platform", artifact.ErrUnsupportedPlatform, "no artifact for this platform"},
		{"checksum", artifact.ErrChecksumMismatch, "artifact checksum mismatch"},
		{"size", artifact.ErrSizeMismatch, "artifact size mismatch"},
		{"format", artifact.ErrUnsupportedFormat, "unsupported artifact format"},
		{"unsafe archive", artifact.ErrUnsafeArchive, "unsafe archive entry"},
		{"manual upgrade stale", &artifact.ManualUpgradeError{Name: "cap", Cask: "captain-hook", Want: "1.2.0", Got: "1.1.0"}, `signed app "cap" is version 1.1.0, want 1.2.0; run: brew upgrade --cask captain-hook`},
		{"manual upgrade absent", &artifact.ManualUpgradeError{Name: "cap", Cask: "captain-hook"}, `signed app "cap" is not installed; run: brew upgrade --cask captain-hook`},
		{"plain passthrough", errors.New("boom"), "boom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Message(tt.err); got != tt.want {
				t.Errorf("Message() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseVerbNormalizes(t *testing.T) {
	descPath := writeDescriptor(t, "#!/usr/bin/env binrun\n"+
		`{"schema":1,"name":"capt-hook","kind":"python-tool","version":{"static":"0.21.3"},"tool":{"dist":"capt-hook","entrypoint":"hook"}}`)
	out := runVerb(t, "parse", descPath)
	if !strings.HasPrefix(out, "{") {
		t.Fatalf("parse output is not normalized JSON: %q", out)
	}
	for _, want := range []string{`"kind": "python-tool"`, `"name": "capt-hook"`, `"dist": "capt-hook"`} {
		if !strings.Contains(out, want) {
			t.Errorf("parse output missing %q; got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "#!/usr/bin/env") {
		t.Errorf("parse output retained the shebang:\n%s", out)
	}
}

func TestResolveVerbCacheHit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	script := []byte("#!/bin/sh\nexit 0\n")
	target := populateCache(t, home, script, "runme")
	descPath := writeDescriptor(t, releaseDescriptorJSON(t, digestOf(script), int64(len(script)), "runme"))

	if out := strings.TrimSpace(runVerb(t, "resolve", descPath)); out != target {
		t.Errorf("resolve = %q, want %q", out, target)
	}
}

func TestCacheDirVerb(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	want := filepath.Join(home, ".daemonkit", "cache")
	if out := strings.TrimSpace(runVerb(t, "cache-dir")); out != want {
		t.Errorf("cache-dir = %q, want %q", out, want)
	}
}

func TestLatestTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if want := "/repos/yasyf/demo/releases/latest"; r.URL.Path != want {
			t.Errorf("request path = %q, want %q", r.URL.Path, want)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v9.9.9", "assets": []any{}})
	}))
	defer srv.Close()

	desc, err := artifact.Parse([]byte(releaseDescriptorJSON(t, strings.Repeat("a", 64), 1, "demo")))
	if err != nil {
		t.Fatal(err)
	}
	tag, err := latestTag(context.Background(), ghrelease.Client{BaseURL: srv.URL}, desc)
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v9.9.9" {
		t.Errorf("latestTag = %q, want v9.9.9", tag)
	}
}

func TestExecRetriesOnPrunedEntry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	script := []byte("#!/bin/sh\nexit 0\n")
	target := populateCache(t, home, script, "runme")
	descPath := writeDescriptor(t, releaseDescriptorJSON(t, digestOf(script), int64(len(script)), "runme"))

	orig := execProcess
	t.Cleanup(func() { execProcess = orig })
	var calls int
	var lastPath string
	execProcess = func(path string, _, _ []string) error {
		calls++
		lastPath = path
		if calls == 1 {
			return syscall.ENOENT // a concurrent gc pruned the entry between resolve and exec
		}
		return nil // the retry's re-resolve refetched it; second exec succeeds
	}
	if err := execDescriptor(context.Background(), descPath, nil); err != nil {
		t.Fatalf("execDescriptor after ENOENT retry: %v", err)
	}
	if calls != 2 {
		t.Errorf("execProcess called %d times, want 2 (initial + one retry)", calls)
	}
	if lastPath != target {
		t.Errorf("retried exec path = %q, want %q", lastPath, target)
	}
}

func TestExecLeadingDashPath(t *testing.T) {
	t.Run("existing leading-dash path execs", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		script := []byte("#!/bin/sh\nexit 0\n")
		populateCache(t, home, script, "runme")
		dir := t.TempDir()
		descName := "-lead.binrun"
		if err := os.WriteFile(filepath.Join(dir, descName), []byte(releaseDescriptorJSON(t, digestOf(script), int64(len(script)), "runme")), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)

		orig := execProcess
		t.Cleanup(func() { execProcess = orig })
		var called bool
		execProcess = func(string, []string, []string) error { called = true; return nil }
		if err := execDescriptor(context.Background(), descName, nil); err != nil {
			t.Fatalf("execDescriptor(%q): %v", descName, err)
		}
		if !called {
			t.Error("expected exec of an existing leading-dash descriptor")
		}
	})

	t.Run("absent leading-dash path errors with guidance", func(t *testing.T) {
		err := execDescriptor(context.Background(), "-nope.binrun", nil)
		if err == nil || !strings.Contains(err.Error(), "is not a descriptor file") {
			t.Errorf("err = %v, want a guidance error", err)
		}
	})

	t.Run("non-ENOENT stat error surfaces the real cause", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "-notadir"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		// Stat traverses "-notadir" (a regular file) as a directory → ENOTDIR,
		// which is not "does not exist" and must not be relabeled as routing.
		err := execDescriptor(context.Background(), "-notadir/child", nil)
		if err == nil {
			t.Fatal("expected a stat error")
		}
		if strings.Contains(err.Error(), "is not a descriptor file") {
			t.Errorf("misdiagnosed a non-ENOENT stat error as routing guidance: %v", err)
		}
		if !errors.Is(err, syscall.ENOTDIR) {
			t.Errorf("err = %v, want the underlying ENOTDIR stat error", err)
		}
	})
}

func TestToPrune(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	at := func(name, digest string, hoursAgo int) artifact.CacheEntry {
		return artifact.CacheEntry{Name: name, Digest: digest, FetchedAt: base.Add(-time.Duration(hoursAgo) * time.Hour)}
	}
	entries := []artifact.CacheEntry{
		at("demo", "d-new", 1), at("demo", "d-mid", 2), at("demo", "d-old", 3),
		at("other", "o-only", 1),
		at("", "x-new", 1), at("", "x-old", 2), // damaged (no meta) share the empty-name group
	}
	tests := []struct {
		name string
		keep int
		want []string // digests expected to be pruned
	}{
		{"keep 2 drops only groups over 2 (demo has 3)", 2, []string{"d-old"}},
		{"keep 1 keeps only the newest per name", 1, []string{"d-mid", "d-old", "x-old"}},
		{"keep 3 prunes nothing", 3, nil},
		{"keep 0 prunes everything", 0, []string{"d-mid", "d-new", "d-old", "o-only", "x-new", "x-old"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := make([]string, 0)
			for _, e := range toPrune(entries, tt.keep) {
				got = append(got, e.Digest)
			}
			slices.Sort(got)
			want := append([]string(nil), tt.want...)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("toPrune(keep=%d) pruned %v, want %v", tt.keep, got, want)
			}
		})
	}
}

func TestGCVerbKeepsNewestPerName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	base := time.Now()
	demoOld := seedCacheEntry(t, home, "demo", "v1", base.Add(-3*time.Hour))
	demoMid := seedCacheEntry(t, home, "demo", "v2", base.Add(-2*time.Hour))
	demoNew := seedCacheEntry(t, home, "demo", "v3", base.Add(-1*time.Hour))
	other := seedCacheEntry(t, home, "other", "v1", base)

	runVerb(t, "gc", "--keep", "1")

	for _, dir := range []string{demoOld, demoMid} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("expected %q pruned, stat err = %v", dir, err)
		}
	}
	for _, dir := range []string{demoNew, other} {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("expected %q kept, stat err = %v", dir, err)
		}
	}
}

func seedCacheEntry(t *testing.T, home, name, tag string, fetchedAt time.Time) string {
	t.Helper()
	digest := digestOf([]byte(name + "@" + tag))
	dir := filepath.Join(home, ".daemonkit", "cache", digest[:2], digest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	meta, err := json.Marshal(map[string]any{"name": name, "tag": tag, "digest": digest, "fetched_at": fetchedAt})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), meta, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func writeDescriptor(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.binrun")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func releaseDescriptorJSON(t *testing.T, digest string, size int64, path string) string {
	t.Helper()
	platform, err := artifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	desc := map[string]any{
		"schema":  1,
		"name":    "demo",
		"kind":    "release-binary",
		"version": map[string]any{"static": "1.0.0"},
		"platforms": map[string]any{
			string(platform): map[string]any{
				"size": size, "hash": "sha256", "digest": digest, "path": path,
				"providers": []any{map[string]any{"type": "github-release", "repo": "yasyf/demo", "tag": "v1.0.0", "name": "demo.tar.gz"}},
			},
		},
	}
	data, err := json.Marshal(desc)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func populateCache(t *testing.T, home string, content []byte, entryPath string) string {
	t.Helper()
	digest := digestOf(content)
	dir := filepath.Join(home, ".daemonkit", "cache", digest[:2], digest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, entryPath)
	if err := os.WriteFile(target, content, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCacheMeta(t, dir, "demo", "v1.0.0", digest)
	return target
}

func writeCacheMeta(t *testing.T, dir, name, tag, digest string) {
	t.Helper()
	meta, err := json.Marshal(map[string]any{"name": name, "tag": tag, "digest": digest, "fetched_at": time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), meta, 0o600); err != nil {
		t.Fatal(err)
	}
}

func runVerb(t *testing.T, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	root := newVerbRoot()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("verb %q: %v", args, err)
	}
	return out.String()
}
