package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yasyf/daemonkit/artifact"
)

type rewriteHost struct{ base *url.URL }

func (r rewriteHost) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = r.base.Scheme
	clone.URL.Host = r.base.Host
	return http.DefaultTransport.RoundTrip(clone)
}

func serveProvider(t *testing.T, handler http.HandlerFunc) *atomic.Int64 {
	t.Helper()
	t.Setenv("GITHUB_TOKEN", "test-token")
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	base, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	orig := resolveOptions
	t.Cleanup(func() { resolveOptions = orig })
	resolveOptions = func() []artifact.Option {
		return append(orig(), artifact.WithHTTPClient(&http.Client{Transport: rewriteHost{base: base}}))
	}
	return &hits
}

func seedVersion(t *testing.T, home, name, tag string, content []byte, fetchedAt time.Time) string {
	t.Helper()
	digest := digestOf(content)
	dir := filepath.Join(home, ".daemonkit", "cache", digest[:2], digest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "runme")
	if err := os.WriteFile(target, content, 0o755); err != nil {
		t.Fatal(err)
	}
	meta, err := json.Marshal(map[string]any{"name": name, "tag": tag, "digest": digest, "fetched_at": fetchedAt})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), meta, 0o600); err != nil {
		t.Fatal(err)
	}
	return target
}

func stubExec(t *testing.T) *[]string {
	t.Helper()
	orig := execProcess
	t.Cleanup(func() { execProcess = orig })
	var paths []string
	execProcess = func(path string, _, _ []string) error {
		paths = append(paths, path)
		return nil
	}
	return &paths
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logs bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	return &logs
}

func pinnedDescriptor(t *testing.T, content []byte) string {
	t.Helper()
	return writeDescriptor(t, releaseDescriptorJSON(t, digestOf(content), int64(len(content)), "runme"))
}

func TestExecRunsTheNewestCachedVersionWhenTheProviderAnswers504(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DAEMONKIT_HOME", home)
	now := time.Now()
	seedVersion(t, home, "demo", "v0.8.0", []byte("#!/bin/sh\necho 0.8\n"), now.Add(-2*time.Hour))
	cached := seedVersion(t, home, "demo", "v0.9.0", []byte("#!/bin/sh\necho 0.9\n"), now.Add(-time.Hour))
	seedVersion(t, home, "other", "v5.0.0", []byte("#!/bin/sh\necho other\n"), now)
	hits := serveProvider(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusGatewayTimeout) })
	paths := stubExec(t)
	logs := captureLogs(t)

	if err := execDescriptor(context.Background(), pinnedDescriptor(t, []byte("#!/bin/sh\necho 1.0\n")), nil); err != nil {
		t.Fatalf("execDescriptor() = %v", err)
	}
	if len(*paths) != 1 || (*paths)[0] != cached {
		t.Fatalf("exec'd %v, want [%s]", *paths, cached)
	}
	if hits.Load() < 2 {
		t.Errorf("provider hits = %d, want the 504 retried", hits.Load())
	}
	line := logs.String()
	if strings.Count(line, "\n") != 1 || !strings.Contains(line, "want=v1.0.0") || !strings.Contains(line, "running=v0.9.0") || !strings.Contains(line, "504") {
		t.Errorf("warning = %q, want one line naming v1.0.0, v0.9.0, and the 504", line)
	}
}

func TestExecNeverFallsBackOnAChecksumMismatch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DAEMONKIT_HOME", home)
	seedVersion(t, home, "demo", "v0.9.0", []byte("#!/bin/sh\necho 0.9\n"), time.Now())
	pinned := []byte("#!/bin/sh\necho 1.0\n")
	serveProvider(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(bytes.Repeat([]byte("x"), len(pinned))) })
	paths := stubExec(t)

	err := execDescriptor(context.Background(), pinnedDescriptor(t, pinned), nil)
	if !errors.Is(err, artifact.ErrChecksumMismatch) {
		t.Fatalf("execDescriptor() = %v, want ErrChecksumMismatch", err)
	}
	if len(*paths) != 0 {
		t.Fatalf("exec'd %v after a checksum mismatch", *paths)
	}
}

func TestExecFailsWhenNoOtherVersionIsCached(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DAEMONKIT_HOME", home)
	seedVersion(t, home, "other", "v5.0.0", []byte("#!/bin/sh\necho other\n"), time.Now())
	serveProvider(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	paths := stubExec(t)

	err := execDescriptor(context.Background(), pinnedDescriptor(t, []byte("#!/bin/sh\necho 1.0\n")), nil)
	if !errors.Is(err, artifact.ErrDownload) {
		t.Fatalf("execDescriptor() = %v, want ErrDownload", err)
	}
	if len(*paths) != 0 {
		t.Fatalf("exec'd %v with no cached version", *paths)
	}
}

func TestFetchVerbReportsAFailedDownloadInsteadOfFallingBack(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DAEMONKIT_HOME", home)
	seedVersion(t, home, "demo", "v0.9.0", []byte("#!/bin/sh\necho 0.9\n"), time.Now())
	serveProvider(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })

	root := newVerbRoot()
	root.SetArgs([]string{"fetch", pinnedDescriptor(t, []byte("#!/bin/sh\necho 1.0\n"))})
	if err := root.Execute(); !errors.Is(err, artifact.ErrDownload) {
		t.Fatalf("fetch = %v, want ErrDownload", err)
	}
}
