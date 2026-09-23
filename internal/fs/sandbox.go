package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Sandbox struct {
	rootDir      string
	ReadOnly     bool
	DisableDelete bool
}

func NewSandbox() (*Sandbox, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &Sandbox{
		rootDir:      cwd,
		ReadOnly:     false,
		DisableDelete: false,
	}, nil
}

// sanitizePath ensures paths stay strictly inside the initialization directory and end in .md
func (s *Sandbox) sanitizePath(filename string) (string, error) {
	// Clean and resolve path relative to rootDir
	cleanName := filepath.Clean(filename)
	absPath := filepath.Join(s.rootDir, cleanName)

	// Security check: ensure path starts with rootDir
	rel, err := filepath.Rel(s.rootDir, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errors.New("security error: path escapes working directory")
	}

	// Extension check: must be .md
	if !strings.HasSuffix(strings.ToLower(absPath), ".md") {
		return "", errors.New("security error: operations permitted only on .md files")
	}

	return absPath, nil
}

func (s *Sandbox) ListFiles() ([]string, error) {
	var files []string
	err := filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip hidden directories like .git
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") && path != s.rootDir {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			rel, _ := filepath.Rel(s.rootDir, path)
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}

func (s *Sandbox) ReadFile(filename string) (string, error) {
	path, err := s.sanitizePath(filename)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (s *Sandbox) WriteFile(filename string, content string) error {
	if s.ReadOnly {
		return errors.New("operation denied: tool is currently in read-only mode (/read-only)")
	}
	path, err := s.sanitizePath(filename)
	if err != nil {
		return err
	}
	// Ensure parent directories exist if needed within sandbox
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func (s *Sandbox) DeleteFile(filename string) error {
	if s.ReadOnly {
		return errors.New("operation denied: tool is currently in read-only mode (/read-only)")
	}
	if s.DisableDelete {
		return errors.New("operation denied: file deletion is currently disabled (/disable-delete)")
	}
	path, err := s.sanitizePath(filename)
	if err != nil {
		return err
	}
	return os.Remove(path)
}
