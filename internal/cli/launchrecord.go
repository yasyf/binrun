package cli

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/yasyf/daemonkit/artifact"
)

// The rendered launcher derives this name with ${path//\//%}.
func launchRecordPath(store artifact.Store, descriptor string) string {
	return filepath.Join(store.Root, "binrun", "resolved", strings.ReplaceAll(descriptor, "/", "%"))
}

func recordLaunch(store artifact.Store, descriptor string, desc *artifact.Descriptor, resolved string) {
	if desc.Version.Dynamic() || !filepath.IsAbs(descriptor) {
		return
	}
	if err := writeLaunchRecord(launchRecordPath(store, descriptor), descriptor, resolved); err != nil {
		slog.Debug("binrun: write launch record", "descriptor", descriptor, "error", err)
	}
}

func writeLaunchRecord(record, descriptor, resolved string) error {
	described, err := os.Stat(descriptor)
	if err != nil {
		return err
	}
	content := []byte(resolved + "\n")
	// The launcher's -nt compares whole seconds, so a record from the descriptor's own second is rewritten.
	if info, err := os.Stat(record); err == nil && info.ModTime().Unix() > described.ModTime().Unix() {
		if current, err := os.ReadFile(filepath.Clean(record)); err == nil && bytes.Equal(current, content) {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(record), 0o750); err != nil {
		return err
	}
	stage, err := os.CreateTemp(filepath.Dir(record), ".record-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(stage.Name()) }()
	if _, err := stage.Write(content); err != nil {
		_ = stage.Close()
		return err
	}
	if err := stage.Close(); err != nil {
		return err
	}
	return os.Rename(stage.Name(), record)
}
