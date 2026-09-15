package cwdguard

import (
	"syscall"
	"unsafe"
)

var origin = -1

func init() {
	fd, err := syscall.Open(".", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return
	}
	if present(fd) || syscall.Chdir("/") != nil {
		_ = syscall.Close(fd)
		return
	}
	origin, Departed = fd, true
}

// Return moves the process back into the directory it started in, so the
// exec'd artifact inherits the working directory binrun was given.
func Return() error {
	if !Departed {
		return nil
	}
	return syscall.Fchdir(origin)
}

func present(fd int) bool {
	var path [1024]byte
	if _, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETPATH, uintptr(unsafe.Pointer(&path[0]))); errno != 0 { //nolint:gosec // G103: F_GETPATH fills the caller-owned buffer
		return false
	}
	end := 0
	for path[end] != 0 {
		end++
	}
	var named, here syscall.Stat_t
	if syscall.Stat(string(path[:end]), &named) != nil || syscall.Fstat(fd, &here) != nil {
		return false
	}
	return named.Dev == here.Dev && named.Ino == here.Ino
}
