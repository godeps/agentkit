package config

import (
	"bytes"
	"errors"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

const (
	claudeMDFileName = "CLAUDE.md"

	claudeMDMaxDepth     = 8
	claudeMDMaxFileBytes = 1 << 20 // 1 MiB
	claudeMDMaxTotal     = 4 << 20 // 4 MiB
)

// LoadClaudeMD loads ./CLAUDE.md and expands @include directives.
//
// Claude Code supports including additional files by placing "@path/to/file"
// lines inside CLAUDE.md. This loader replaces those lines with the referenced
// file content (recursively).
//
// Missing CLAUDE.md returns ("", nil).
func LoadClaudeMD(projectRoot string, filesystem *FS) (string, error) {
	root := strings.TrimSpace(projectRoot)
	if root == "" {
		root = "."
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}

	loader := claudeMDLoader{
		root:    root,
		fs:      filesystem,
		visited: map[string]struct{}{},
	}
	content, err := loader.load(filepath.Join(root, claudeMDFileName), 0)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(content), nil
}

type claudeMDLoader struct {
	root    string
	fs      *FS
	visited map[string]struct{}
	total   int64
}

func (l *claudeMDLoader) load(path string, depth int) (string, error) {
	if depth > claudeMDMaxDepth {
		return "", fmt.Errorf("claude.md: include depth exceeds %d", claudeMDMaxDepth)
	}

	absPath := strings.TrimSpace(path)
	if absPath == "" {
		return "", nil
	}
	if !filepath.IsAbs(absPath) && !isWindowsAbs(absPath) {
		absPath = filepath.Join(l.root, absPath)
	}
	absPath = filepath.Clean(absPath)
	if abs, err := filepath.Abs(absPath); err == nil {
		absPath = abs
	}

	if strings.TrimSpace(l.root) != "" {
		rel, err := filepath.Rel(l.root, absPath)
		if err != nil {
			return "", fmt.Errorf("claude.md: resolve include %q: %w", path, err)
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("claude.md: include path escapes project root: %s", path)
		}
	}

	visitKey := absPath
	if runtime.GOOS == "windows" {
		visitKey = strings.ToLower(visitKey)
	}
	if _, ok := l.visited[visitKey]; ok {
		return "", nil
	}
	l.visited[visitKey] = struct{}{}

	data, err := readFileLimited(l.fs, absPath, claudeMDMaxFileBytes)
	if err != nil {
		if errors.Is(err, iofs.ErrNotExist) && depth == 0 {
			return "", nil
		}
		return "", err
	}
	if int64(len(data)) > claudeMDMaxFileBytes {
		return "", fmt.Errorf("claude.md: %s exceeds %d bytes limit", absPath, claudeMDMaxFileBytes)
	}
	if l.total+int64(len(data)) > claudeMDMaxTotal {
		return "", fmt.Errorf("claude.md: total included content exceeds %d bytes limit", claudeMDMaxTotal)
	}
	l.total += int64(len(data))

	if bytes.IndexByte(data, 0) >= 0 {
		return "", fmt.Errorf("claude.md: %s appears to be binary", absPath)
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("claude.md: %s is not valid UTF-8", absPath)
	}

	dir := filepath.Dir(absPath)
	lines := strings.Split(string(data), "\n")

	var b strings.Builder
	inCodeBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}

		if !inCodeBlock && strings.HasPrefix(trimmed, "@") {
			target := strings.TrimSpace(strings.TrimPrefix(trimmed, "@"))
			if target == "" {
				continue
			}
			includePath := target
			if !filepath.IsAbs(includePath) && !isWindowsAbs(includePath) {
				includePath = filepath.Join(dir, includePath)
			}
			included, err := l.load(includePath, depth+1)
			if err != nil {
				return "", err
			}
			included = strings.TrimRight(included, "\n")
			if included != "" {
				b.WriteString(included)
				b.WriteByte('\n')
			}
			continue
		}

		b.WriteString(line)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func readFileLimited(filesystem *FS, path string, maxBytes int64) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("claude.md: path is empty")
	}
	if maxBytes <= 0 {
		maxBytes = claudeMDMaxFileBytes
	}

	stat := func() (iofs.FileInfo, error) { return os.Stat(path) }
	read := func() ([]byte, error) { return os.ReadFile(path) }
	if filesystem != nil {
		stat = func() (iofs.FileInfo, error) { return filesystem.Stat(path) }
		read = func() ([]byte, error) { return filesystem.ReadFile(path) }
	}

	info, err := stat()
	if err != nil {
		return nil, err
	}
	if info != nil && info.Size() > maxBytes {
		return nil, fmt.Errorf("claude.md: %s exceeds %d bytes limit", path, maxBytes)
	}
	data, err := read()
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("claude.md: %s exceeds %d bytes limit", path, maxBytes)
	}
	return data, nil
}
