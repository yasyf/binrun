package livedir

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// List enumerates this user's processes and their argv, read from
// /proc/<pid>/cmdline.
func List() ([]Process, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}
	uid := os.Getuid()
	procs := make([]Process, 0, len(entries))
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		dir := filepath.Join("/proc", entry.Name())
		info, err := os.Stat(dir)
		if gone(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", dir, err)
		}
		if int(info.Sys().(*syscall.Stat_t).Uid) != uid {
			continue
		}
		//nolint:gosec // G304: the path is /proc plus a pid strconv already parsed as an integer
		data, err := os.ReadFile(filepath.Join(dir, "cmdline"))
		if gone(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read argv of pid %d: %w", pid, err)
		}
		// A kernel thread has an empty cmdline and runs out of no directory.
		if len(data) == 0 {
			continue
		}
		args := bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0})
		proc := Process{PID: pid, Args: make([]string, 0, len(args))}
		for _, arg := range args {
			proc.Args = append(proc.Args, string(arg))
		}
		procs = append(procs, proc)
	}
	return procs, nil
}

// gone reports whether err means the process exited between the walk and the
// read. A running process whose argv is unreadable is not gone, and must not
// pass for one that holds no directory open.
func gone(err error) bool {
	return err != nil && (errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ESRCH))
}
