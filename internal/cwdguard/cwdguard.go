// Package cwdguard leaves a removed working directory for "/" before package os
// runs os/executable_darwin.go's `initCwd, initCwdErr = Getwd()`, whose libc
// fallback scans the former parent; importing only syscall puts this init ahead
// of os in Go's import-path initialization order.
package cwdguard

// Departed reports that the process started in a removed working directory
// and now runs from "/".
var Departed bool
