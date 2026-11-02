//go:build !android && !darwin && !js

package shell

// systemReduceMotion has nothing to ask on this platform: the animations run.
func systemReduceMotion() bool { return false }
