package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yasyf/daemonkit/artifact"
)

func TestExecRecordsWhereAStaticDescriptorResolved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DAEMONKIT_HOME", home)
	script := []byte("#!/bin/sh\nexit 0\n")
	target := populateCache(t, home, script, "runme")
	descPath := writeDescriptor(t, releaseDescriptorJSON(t, digestOf(script), int64(len(script)), "runme"))
	orig := execProcess
	t.Cleanup(func() { execProcess = orig })
	execProcess = func(string, []string, []string) error { return nil }
	if err := execDescriptor(context.Background(), descPath, nil); err != nil {
		t.Fatalf("execDescriptor: %v", err)
	}
	record := launchRecordPath(artifact.Store{Root: filepath.Join(home, ".daemonkit")}, descPath)
	if got, err := os.ReadFile(record); err != nil || string(got) != target+"\n" {
		t.Fatalf("launch record = %q, %v; want %q", got, err, target+"\n")
	}
}

func TestRecordLaunchSkipsDescriptorsThatCanResolveElsewhere(t *testing.T) {
	descPath := writeDescriptor(t, "{}")
	parsed, err := os.Stat(descPath)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		desc artifact.Descriptor
	}{
		{"dynamic version", artifact.Descriptor{Kind: artifact.ReleaseBinary, Version: artifact.VersionSource{Command: []string{"version"}}}},
		{"signed app", artifact.Descriptor{Kind: artifact.SignedApp, Version: artifact.VersionSource{Static: "1.0.0"}}},
		{"python tool", artifact.Descriptor{Kind: artifact.PythonTool, Version: artifact.VersionSource{Static: "1.0.0"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := artifact.Store{Root: t.TempDir()}
			recordLaunch(store, descPath, parsed, &tt.desc, "/bin/true")
			if _, err := os.Stat(launchRecordPath(store, descPath)); !os.IsNotExist(err) {
				t.Fatalf("stat launch record = %v; want not exist", err)
			}
		})
	}
}

func TestRecordLaunchSkipsADescriptorRewrittenWhileItResolved(t *testing.T) {
	descPath := writeDescriptor(t, "{}")
	parsed, err := os.Stat(descPath)
	if err != nil {
		t.Fatal(err)
	}
	later := parsed.ModTime().Add(time.Second)
	if err := os.Chtimes(descPath, later, later); err != nil {
		t.Fatal(err)
	}
	store := artifact.Store{Root: t.TempDir()}
	desc := &artifact.Descriptor{Kind: artifact.ReleaseBinary, Version: artifact.VersionSource{Static: "1.0.0"}}
	recordLaunch(store, descPath, parsed, desc, "/bin/true")
	if _, err := os.Stat(launchRecordPath(store, descPath)); !os.IsNotExist(err) {
		t.Fatalf("stat launch record = %v; want not exist", err)
	}
}

func TestLaunchRecordNamesAreInjective(t *testing.T) {
	store := artifact.Store{Root: "/h/.daemonkit"}
	tests := []struct{ descriptor, want string }{
		{"/p/a/b/x.binrun", "/h/.daemonkit/binrun/resolved/%2Fp%2Fa%2Fb%2Fx.binrun"},
		{"/p/a%b/x.binrun", "/h/.daemonkit/binrun/resolved/%2Fp%2Fa%25b%2Fx.binrun"},
	}
	for _, tt := range tests {
		if got := launchRecordPath(store, tt.descriptor); got != tt.want {
			t.Errorf("launchRecordPath(%q) = %q, want %q", tt.descriptor, got, tt.want)
		}
	}
}
