//go:build (unix && !arm64) || darwin

package logger

import (
	"log"
	"os"
	"syscall"
)

// redirectStderr to the file passed in
func redirectStderr(outFile, errFile *os.File) {
	err := syscall.Dup2(int(errFile.Fd()), int(os.Stderr.Fd()))
	if err != nil {
		log.Fatalf("Failed to redirect stderr to file: %v", err)
	}
	err = syscall.Dup2(int(outFile.Fd()), int(os.Stdout.Fd()))
	if err != nil {
		log.Fatalf("Failed to redirect stdout to file: %v", err)
	}
}
