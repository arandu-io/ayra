//go:build !ios

package compiled

func targetIsIOSSimulator() bool {
	return false
}
