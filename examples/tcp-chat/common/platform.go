package common

import (
	"os"
	"runtime"
)

// GetFileMode returns the appropriate file mode for the platform
func GetFileMode() os.FileMode {
	if runtime.GOOS == "windows" {
		return 0666 // Windows doesn't use Unix permissions
	}
	return 0644 // Unix-like systems
}

// GetDirMode returns the appropriate directory mode for the platform
func GetDirMode() os.FileMode {
	if runtime.GOOS == "windows" {
		return 0777 // Windows doesn't use Unix permissions
	}
	return 0755 // Unix-like systems
}
