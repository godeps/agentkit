package clikit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type outputValidationResult struct {
	Path        string
	Resolved    string
	Exists      bool
	IsDir       bool
	Fresh       bool
	JSONChecked bool
	JSONValid   bool
	Err         string
}

var outputPathPattern = regexp.MustCompile(`(?:^|[\s"'` + "`" + `])((?:\./)?output/[^\s"'` + "`" + `]+)`)

func detectOutputPathsFromText(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	matches := outputPathPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	uniq := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		p := strings.TrimSpace(m[1])
		p = strings.TrimRight(p, ".,;:)]}\"'")
		p = strings.TrimSuffix(p, `\n`)
		p = strings.TrimSuffix(p, `\r`)
		p = strings.TrimSuffix(p, "\n")
		p = strings.TrimSuffix(p, "\r")
		if strings.HasPrefix(p, "./") {
			p = strings.TrimPrefix(p, "./")
		}
		p = filepath.Clean(p)
		if strings.TrimSpace(p) == "" {
			continue
		}
		uniq[p] = struct{}{}
	}
	if len(uniq) == 0 {
		return nil
	}
	out := make([]string, 0, len(uniq))
	for p := range uniq {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func validateGeneratedOutputsDetailed(baseDir string, paths []string, runStartedAt time.Time) ([]outputValidationResult, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	baseDir = filepath.Clean(strings.TrimSpace(baseDir))
	var errs []string
	results := make([]outputValidationResult, 0, len(paths))
	for _, p := range paths {
		p = filepath.Clean(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		resolved := p
		if !filepath.IsAbs(resolved) && baseDir != "" {
			resolved = filepath.Clean(filepath.Join(baseDir, resolved))
		}
		result := outputValidationResult{
			Path:     p,
			Resolved: resolved,
			Fresh:    true,
		}
		info, err := os.Stat(resolved)
		if err != nil {
			result.Err = "not found"
			errs = append(errs, fmt.Sprintf("%s: %s", p, result.Err))
			results = append(results, result)
			continue
		}
		result.Exists = true
		if info.IsDir() {
			result.IsDir = true
			results = append(results, result)
			continue
		}
		if !runStartedAt.IsZero() && info.ModTime().Before(runStartedAt.Add(-1*time.Second)) {
			result.Fresh = false
			result.Err = "not generated in current run"
			errs = append(errs, fmt.Sprintf("%s: %s", p, result.Err))
			results = append(results, result)
			continue
		}
		if strings.EqualFold(filepath.Ext(p), ".json") {
			result.JSONChecked = true
			b, err := os.ReadFile(resolved)
			if err != nil {
				result.Err = "read failed"
				errs = append(errs, fmt.Sprintf("%s: %s", p, result.Err))
				results = append(results, result)
				continue
			}
			result.JSONValid = json.Valid(b)
			if !result.JSONValid {
				result.Err = "invalid json"
				errs = append(errs, fmt.Sprintf("%s: %s", p, result.Err))
			}
		}
		results = append(results, result)
	}
	if len(errs) > 0 {
		return results, fmt.Errorf("generated output verification failed: %s", strings.Join(errs, "; "))
	}
	return results, nil
}

func validateGeneratedOutputs(baseDir string, paths []string, runStartedAt time.Time) error {
	_, err := validateGeneratedOutputsDetailed(baseDir, paths, runStartedAt)
	return err
}
