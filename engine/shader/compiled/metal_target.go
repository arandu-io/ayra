package compiled

// metalLibraryFor selects bytecode compiled for the platform loading it.
//
// An arm64 architecture does not distinguish an iPhone from an Apple silicon
// simulator. The simulator decision comes from the target environment instead,
// before Metal sees bytecode for an incompatible operating system.
func metalLibraryFor(goos string, simulator bool, macOS, iOS, iOSSimulator string) string {
	switch goos {
	case "darwin":
		return macOS
	case "ios":
		if simulator {
			return iOSSimulator
		}
		return iOS
	default:
		return ""
	}
}
