//go:build !darwin

package cwdguard

// Return is a no-op: only darwin's package os calls getcwd at init.
func Return() error {
	return nil
}
