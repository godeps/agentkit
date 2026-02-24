package prompts

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/godeps/agentkit/pkg/runtime/subagents"
	"gopkg.in/yaml.v3"
)

var (
	subagentNameRegexp = regexp.MustCompile(`^[a-z0-9-]+$`)
	allowedModels      = map[string]struct{}{"sonnet": {}, "opus": {}, "haiku": {}, "inherit": {}}
	allowedPermission  = map[string]struct{}{
		"default":           {},
		"acceptedits":       {},
		"bypasspermissions": {},
		"plan":              {},
		"ignore":            {},
	}
)

// subagentMetadata mirrors the YAML frontmatter fields.
type subagentMetadata struct {
	Name           string `yaml:"name"`
	Description    string `yaml:"description"`
	Tools          string `yaml:"tools"`
	Model          string `yaml:"model"`
	PermissionMode string `yaml:"permissionMode"`
	Skills         string `yaml:"skills"`
}

// parseSubagents parses all subagents from the given directory in the filesystem.
func parseSubagents(fsys fs.FS, dir string, validate bool) ([]SubagentRegistration, []error) {
	var (
		regs   []SubagentRegistration
		errs   []error
		merged = map[string]subagentFile{}
	)

	info, err := fs.Stat(fsys, dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, []error{fmt.Errorf("prompts: stat subagents dir %s: %w", dir, err)}
	}
	if !info.IsDir() {
		return nil, []error{fmt.Errorf("prompts: subagents path %s is not a directory", dir)}
	}

	walkErr := fs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			errs = append(errs, fmt.Errorf("prompts: walk subagents %s: %w", path, walkErr))
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(d.Name())) != ".md" {
			return nil
		}

		fallback := strings.ToLower(strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())))
		file, parseErr := parseSubagentFile(fsys, path, fallback, validate)
		if parseErr != nil {
			errs = append(errs, parseErr)
			return nil
		}
		if _, exists := merged[file.meta.Name]; exists {
			errs = append(errs, fmt.Errorf("prompts: duplicate subagent %q in %s", file.meta.Name, dir))
			return nil
		}
		merged[file.meta.Name] = file
		return nil
	})
	if walkErr != nil {
		errs = append(errs, walkErr)
	}

	if len(merged) == 0 {
		return nil, errs
	}

	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		file := merged[name]
		model, modelErr := normalizeSubagentModel(file.meta.Model)
		if modelErr != nil {
			errs = append(errs, fmt.Errorf("prompts: subagent %s: %w", file.path, modelErr))
			continue
		}
		whitelist := parseSubagentList(file.meta.Tools)
		skillsList := parseSubagentList(file.meta.Skills)
		meta := buildSubagentMetadataMap(file, whitelist, skillsList, model)

		def := subagents.Definition{
			Name:         file.meta.Name,
			Description:  file.meta.Description,
			BaseContext:  subagents.Context{ToolWhitelist: whitelist, Model: model, Metadata: meta},
			DefaultModel: model,
		}

		body := file.body
		handler := subagents.HandlerFunc(func(context.Context, subagents.Context, subagents.Request) (subagents.Result, error) {
			res := subagents.Result{Output: body}
			if len(meta) > 0 {
				res.Metadata = meta
			}
			return res, nil
		})

		regs = append(regs, SubagentRegistration{
			Definition: def,
			Handler:    handler,
		})
	}

	return regs, errs
}

type subagentFile struct {
	name string
	path string
	meta subagentMetadata
	body string
}

func parseSubagentFile(fsys fs.FS, path, fallback string, validate bool) (subagentFile, error) {
	content, err := fs.ReadFile(fsys, path)
	if err != nil {
		return subagentFile{}, fmt.Errorf("prompts: read subagent %s: %w", path, err)
	}

	meta, body, err := parseSubagentFrontMatter(string(content))
	if err != nil {
		return subagentFile{}, fmt.Errorf("prompts: parse subagent %s: %w", path, err)
	}

	meta.Name = strings.ToLower(strings.TrimSpace(meta.Name))
	meta.Description = strings.TrimSpace(meta.Description)
	meta.Tools = strings.TrimSpace(meta.Tools)
	meta.Model = strings.ToLower(strings.TrimSpace(meta.Model))
	meta.PermissionMode = strings.TrimSpace(meta.PermissionMode)
	meta.Skills = strings.TrimSpace(meta.Skills)

	if meta.Name == "" {
		meta.Name = strings.ToLower(strings.TrimSpace(fallback))
	}

	if validate {
		if err := validateSubagentMetadata(meta); err != nil {
			return subagentFile{}, fmt.Errorf("prompts: validate subagent %s: %w", path, err)
		}
	}

	return subagentFile{
		name: meta.Name,
		path: path,
		meta: meta,
		body: body,
	}, nil
}

func parseSubagentFrontMatter(content string) (subagentMetadata, string, error) {
	trimmed := strings.TrimPrefix(content, "\uFEFF")
	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return subagentMetadata{}, "", errors.New("missing YAML frontmatter")
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return subagentMetadata{}, "", errors.New("missing closing frontmatter separator")
	}

	metaText := strings.Join(lines[1:end], "\n")
	var meta subagentMetadata
	if err := yaml.Unmarshal([]byte(metaText), &meta); err != nil {
		return subagentMetadata{}, "", fmt.Errorf("decode YAML: %w", err)
	}

	body := strings.Join(lines[end+1:], "\n")
	body = strings.TrimPrefix(body, "\n")
	return meta, body, nil
}

func validateSubagentMetadata(meta subagentMetadata) error {
	if meta.Name == "" {
		return errors.New("name is required")
	}
	if !subagentNameRegexp.MatchString(meta.Name) {
		return fmt.Errorf("invalid name %q", meta.Name)
	}
	desc := strings.TrimSpace(meta.Description)
	if desc == "" {
		return errors.New("description is required")
	}
	if meta.Model != "" {
		if _, ok := allowedModels[meta.Model]; !ok {
			return fmt.Errorf("invalid model %q", meta.Model)
		}
	}
	if pm := strings.ToLower(meta.PermissionMode); pm != "" {
		if _, ok := allowedPermission[pm]; !ok {
			return fmt.Errorf("invalid permissionMode %q", meta.PermissionMode)
		}
	}
	return nil
}

func normalizeSubagentModel(model string) (string, error) {
	if model == "" || model == "inherit" {
		return "", nil
	}
	if _, ok := allowedModels[model]; ok {
		return model, nil
	}
	return "", fmt.Errorf("invalid model %q", model)
}

func parseSubagentList(raw string) []string {
	parts := strings.Split(raw, ",")
	seen := map[string]struct{}{}
	var out []string
	for _, part := range parts {
		val := strings.ToLower(strings.TrimSpace(part))
		if val == "" {
			continue
		}
		if _, ok := seen[val]; ok {
			continue
		}
		seen[val] = struct{}{}
		out = append(out, val)
	}
	sort.Strings(out)
	return out
}

func buildSubagentMetadataMap(file subagentFile, tools, skills []string, model string) map[string]any {
	meta := map[string]any{}
	if len(tools) > 0 {
		meta["tools"] = tools
	}
	if model != "" {
		meta["model"] = model
	}
	if pm := strings.ToLower(strings.TrimSpace(file.meta.PermissionMode)); pm != "" {
		meta["permission-mode"] = pm
	}
	if len(skills) > 0 {
		meta["skills"] = skills
	}
	if file.path != "" {
		meta["source"] = file.path
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}
