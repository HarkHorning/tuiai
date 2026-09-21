package main

import (
	"os"
	"path/filepath"
	"strings"
)

type WorkspaceManager struct {
	Root string
}

func NewWorkspaceManager() (*WorkspaceManager, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &WorkspaceManager{Root: cwd}, nil
}

// SanitizePath ensures paths stay strictly within the workspace and end with .md
func (w *WorkspaceManager) SanitizePath(relPath string) (string, bool) {
	// Clean the path relative to root
	cleaned := filepath.Clean(relPath)
	fullPath := filepath.Join(w.Root, cleaned)

	// Ensure it starts with root (prevents directory traversal)
	rel, err := filepath.Rel(w.Root, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}

	// Must be a markdown file
	if !strings.HasSuffix(strings.ToLower(fullPath), ".md") {
		return "", false
	}

	return fullPath, true
}

// ListMarkdownFiles returns all .md files in the workspace recursively
func (w *WorkspaceManager) ListMarkdownFiles() ([]string, error) {
	var files []string
	err := filepath.WalkDir(w.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip hidden directories like .git
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != w.Root {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			rel, err := filepath.Rel(w.Root, path)
			if err == nil {
				files = append(files, rel)
			}
		}
		return nil
	})
	return files, err
}

func (w *WorkspaceManager) ReadMarkdown(relPath string) (string, error) {
	fullPath, ok := w.SanitizePath(relPath)
	if !ok {
		return "", os.ErrPermission
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (w *WorkspaceManager) WriteMarkdown(relPath, content string) error {
	fullPath, ok := w.SanitizePath(relPath)
	if !ok {
		return os.ErrPermission
	}
	return os.WriteFile(fullPath, []byte(content), 0644)
}
