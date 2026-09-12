//go:build !android

package app

import "os"

func dataDir() (string, error) {
	return os.UserConfigDir()
}
