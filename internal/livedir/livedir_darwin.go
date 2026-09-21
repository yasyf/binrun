package livedir

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// List enumerates this user's processes and their argv, read from the kernel
// through KERN_PROCARGS2.
func List() ([]Process, error) {
	kinfo, err := unix.SysctlKinfoProcSlice("kern.proc.uid", os.Getuid())
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}
	procs := make([]Process, 0, len(kinfo))
	for i := range kinfo {
		pid := int(kinfo[i].Proc.P_pid)
		buf, err := procArgs(pid)
		// EINVAL is the kernel's answer for a pid that is gone and for a zombie
		// or kernel task with no user stack. None of them is running out of a
		// directory; any other failure leaves liveness unknown.
		if errors.Is(err, unix.EINVAL) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read argv of pid %d: %w", pid, err)
		}
		args, err := parseProcArgs(buf)
		if err != nil {
			return nil, fmt.Errorf("read argv of pid %d: %w", pid, err)
		}
		procs = append(procs, Process{PID: pid, Args: args})
	}
	return procs, nil
}

// procArgs reads pid's argument area, retrying the one transient failure the
// kernel has: EIO is a failed copy out of the target's address space, which a
// process mid-fork, mid-execve, or mid-exit provokes and a settled one does
// not. A retry has to yield the scheduler for the target to settle.
func procArgs(pid int) ([]byte, error) {
	buf, err := unix.SysctlRaw("kern.procargs2", pid)
	for attempt := 0; attempt < 3 && errors.Is(err, unix.EIO); attempt++ {
		time.Sleep(time.Millisecond)
		buf, err = unix.SysctlRaw("kern.procargs2", pid)
	}
	return buf, err
}

// parseProcArgs decodes a KERN_PROCARGS2 buffer: an argc header, the executable
// path, NUL padding, then argc NUL-terminated arguments. A buffer too small for
// the whole area holds its tail rather than its head, so anything short of argc
// arguments is undecodable, not merely partial.
func parseProcArgs(buf []byte) ([]string, error) {
	if len(buf) < 4 {
		return nil, fmt.Errorf("argv buffer is %d bytes, want at least 4", len(buf))
	}
	argc := int(binary.NativeEndian.Uint32(buf[:4]))
	rest := buf[4:]
	path := bytes.IndexByte(rest, 0)
	if path < 0 {
		return nil, errors.New("argv buffer has no executable path")
	}
	rest = bytes.TrimLeft(rest[path:], "\x00")
	args := make([]string, 0, argc)
	for range argc {
		end := bytes.IndexByte(rest, 0)
		if end < 0 {
			return nil, fmt.Errorf("argv buffer holds %d of %d arguments", len(args), argc)
		}
		args = append(args, string(rest[:end]))
		rest = rest[end+1:]
	}
	return args, nil
}
