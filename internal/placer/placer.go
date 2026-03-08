package placer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

type Mode string

const (
	ModeHardlink Mode = "hardlink"
	ModeSymlink  Mode = "symlink"
	ModeCopy     Mode = "copy"
)

func SupportedModes() []string {
	return []string{string(ModeHardlink), string(ModeSymlink), string(ModeCopy)}
}

type Placer struct {
	mode Mode
}

func New(mode string) *Placer {
	return &Placer{mode: Mode(mode)}
}

func (filePlacer *Placer) Place(sourcePath, destinationDirectory string) (destinationPath string, err error) {
	destinationPath = filepath.Join(destinationDirectory, filepath.Base(sourcePath))

	if _, err := os.Lstat(destinationPath); err == nil {
		return destinationPath, nil
	}

	parentDir := filepath.Dir(destinationPath)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return "", fmt.Errorf("creating parent directory %s: %w", parentDir, err)
	}
	if err := chownLike(parentDir, sourcePath); err != nil {
		return "", fmt.Errorf("chown parent directory %s: %w", parentDir, err)
	}

	switch filePlacer.mode {
	case ModeHardlink:
		return destinationPath, hardlink(sourcePath, destinationPath)
	case ModeSymlink:
		return destinationPath, symlink(sourcePath, destinationPath)
	case ModeCopy:
		return destinationPath, copyAll(sourcePath, destinationPath)
	}
	panic("unreachable: unsupported mode " + string(filePlacer.mode))
}

func (filePlacer *Placer) Mode() string { return string(filePlacer.mode) }

func chownLike(destPath, sourcePath string) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return nil
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	return os.Lchown(destPath, int(stat.Uid), int(stat.Gid))
}

// ---------------------------------------------------------------------------
// hardlink
// ---------------------------------------------------------------------------

// hardlink creates a hard link for a single file, or recursively hard-links
// every file inside a directory tree (equivalent to `cp -rl src dst`).
func hardlink(sourcePath, destinationPath string) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", sourcePath, err)
	}

	if !info.IsDir() {
		return os.Link(sourcePath, destinationPath)
	}

	return filepath.Walk(sourcePath, func(currentSourcePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(sourcePath, currentSourcePath)
		if err != nil {
			return err
		}
		currentDestinationPath := filepath.Join(destinationPath, relativePath)

		if fileInfo.IsDir() {
			if err := os.MkdirAll(currentDestinationPath, fileInfo.Mode()); err != nil {
				return err
			}
			return chownLike(currentDestinationPath, currentSourcePath)
		}

		return os.Link(currentSourcePath, currentDestinationPath)
	})
}

// ---------------------------------------------------------------------------
// symlink
// ---------------------------------------------------------------------------

func symlink(sourcePath, destinationPath string) error {
	if err := os.Symlink(sourcePath, destinationPath); err != nil {
		return err
	}
	return chownLike(destinationPath, sourcePath)
}

// ---------------------------------------------------------------------------
// copy
// ---------------------------------------------------------------------------

// copyAll copies a file or directory tree from src to dst.
func copyAll(sourcePath, destinationPath string) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", sourcePath, err)
	}

	if !info.IsDir() {
		if err := copyFile(sourcePath, destinationPath); err != nil {
			return err
		}
		return chownLike(destinationPath, sourcePath)
	}

	return filepath.Walk(sourcePath, func(currentSourcePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(sourcePath, currentSourcePath)
		if err != nil {
			return err
		}
		currentDestinationPath := filepath.Join(destinationPath, relativePath)

		if fileInfo.IsDir() {
			if err := os.MkdirAll(currentDestinationPath, fileInfo.Mode()); err != nil {
				return err
			}
			return chownLike(currentDestinationPath, currentSourcePath)
		}

		if err := copyFile(currentSourcePath, currentDestinationPath); err != nil {
			return err
		}
		return chownLike(currentDestinationPath, currentSourcePath)
	})
}

// copyFile copies a single regular file from src to dst, preserving permissions.
func copyFile(sourcePath, destinationPath string) error {
	inFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer inFile.Close()

	info, err := inFile.Stat()
	if err != nil {
		return err
	}

	outFile, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, inFile); err != nil {
		return err
	}
	return outFile.Close()
}
