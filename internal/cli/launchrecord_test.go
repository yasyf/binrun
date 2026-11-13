package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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

func TestRecordLaunchSkipsADynamicVersion(t *testing.T) {
	store := artifact.Store{Root: t.TempDir()}
	descPath := writeDescriptor(t, "{}")
	desc := &artifact.Descriptor{Version: artifact.VersionSource{Command: []string{"version"}}}
	recordLaunch(store, descPath, desc, "/bin/true")
	if _, err := os.Stat(launchRecordPath(store, descPath)); !os.IsNotExist(err) {
		t.Fatalf("stat launch record = %v; want not exist", err)
	}
}

func TestLaunchRecordNameSpellsSlashesAsPercent(t *testing.T) {
	store := artifact.Store{Root: "/h/.daemonkit"}
	if got, want := launchRecordPath(store, "/p/bin/x.binrun"), "/h/.daemonkit/binrun/resolved/%p%bin%x.binrun"; got != want {
		t.Fatalf("launchRecordPath = %q, want %q", got, want)
	}
}
