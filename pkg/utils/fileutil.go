package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	modkernel32    = syscall.NewLazyDLL("kernel32.dll")
	procCreateDirW = modkernel32.NewProc("CreateDirectoryW")
)

func createDirWin32(path string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	utf16Path, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	r1, _, errSys := procCreateDirW.Call(uintptr(unsafe.Pointer(utf16Path)), 0)
	if r1 != 0 {
		return true
	}
	if errno, ok := errSys.(syscall.Errno); ok && errno == 183 { // ERROR_ALREADY_EXISTS
		return true
	}
	return false
}

// EnsureDir ensures that the directory or the parent directory for targetPath exists.
// Handles both file paths (e.g. ./data/appendonly.aof) and directory paths (e.g. data, ./data).
// Operates with multi-tier strategy: fast os.Stat, fast os.MkdirAll on absolute/relative paths,
// direct Win32 CreateDirectoryW, micro-backoff retries, and OS shell fallback (for Windows Defender
// Controlled Folder Access on Desktop/Documents folders).
func EnsureDir(targetPath string) error {
	cleanPath := filepath.Clean(targetPath)
	if cleanPath == "." || cleanPath == "" {
		return nil
	}

	dir := cleanPath
	if filepath.Ext(cleanPath) != "" && !strings.HasSuffix(targetPath, "/") && !strings.HasSuffix(targetPath, "\\") {
		dir = filepath.Dir(cleanPath)
		if dir == "." || dir == "" {
			return nil
		}
	}

	absDir, _ := filepath.Abs(dir)

	// 1. Fast path: check if already exists
	if isDir(dir) || (absDir != "" && isDir(absDir)) {
		return nil
	}

	// 2. Fast path: direct creation attempts
	if err := os.MkdirAll(dir, 0755); err == nil || os.IsExist(err) {
		if isDir(dir) || isDir(absDir) {
			return nil
		}
	}
	if absDir != "" && absDir != dir {
		if err := os.MkdirAll(absDir, 0755); err == nil || os.IsExist(err) {
			if isDir(dir) || isDir(absDir) {
				return nil
			}
		}
	}
	if createDirWin32(dir) || (absDir != "" && createDirWin32(absDir)) {
		if isDir(dir) || isDir(absDir) {
			return nil
		}
	}

	// 3. Fast micro-backoff retry for transient Windows NTFS delete-pending states (max ~150ms)
	for attempt := 0; attempt < 6; attempt++ {
		time.Sleep(25 * time.Millisecond)

		if isDir(dir) || (absDir != "" && isDir(absDir)) {
			return nil
		}
		if err := os.MkdirAll(dir, 0755); err == nil || os.IsExist(err) {
			if isDir(dir) || isDir(absDir) {
				return nil
			}
		}
		if absDir != "" && absDir != dir {
			if err := os.MkdirAll(absDir, 0755); err == nil || os.IsExist(err) {
				if isDir(dir) || isDir(absDir) {
					return nil
				}
			}
		}
		if createDirWin32(dir) || (absDir != "" && createDirWin32(absDir)) {
			if isDir(dir) || isDir(absDir) {
				return nil
			}
		}
	}

	if isDir(dir) || (absDir != "" && isDir(absDir)) {
		return nil
	}

	// 4. Windows Defender / Controlled Folder Access fallback:
	// When Desktop/Documents paths are protected against unsigned .exe binaries,
	// invoke the system shell which has built-in OS permissions.
	if runtime.GOOS == "windows" {
		target := absDir
		if target == "" {
			target = dir
		}
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
			"-Command", fmt.Sprintf(`New-Item -ItemType Directory -Path "%s" -Force`, target))
		_ = cmd.Run()

		if isDir(dir) || isDir(absDir) {
			return nil
		}
	}

	if isDir(dir) || (absDir != "" && isDir(absDir)) {
		return nil
	}

	return fmt.Errorf("mkdir %s: access denied or path protected", dir)
}

func isDir(path string) bool {
	if path == "" {
		return false
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return true
	}
	return false
}

// EnsureDirExists ensures that the specified directory exists.
func EnsureDirExists(dirPath string) error {
	return EnsureDir(dirPath)
}

// AtomicWriteFile writes data to targetPath atomically by writing to a temporary file first and then renaming it.
func AtomicWriteFile(targetPath string, data []byte, perm os.FileMode) error {
	if err := EnsureDir(targetPath); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
	}

	tmpPath := targetPath + ".tmp"
	tmpFile, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("failed to create temp file %s: %w", tmpPath, err)
	}

	_, writeErr := tmpFile.Write(data)
	syncErr := tmpFile.Sync()
	closeErr := tmpFile.Close()

	if writeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write to temp file %s: %w", tmpPath, writeErr)
	}
	if syncErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to sync temp file %s: %w", tmpPath, syncErr)
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close temp file %s: %w", tmpPath, closeErr)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file to %s: %w", targetPath, err)
	}

	return nil
}
