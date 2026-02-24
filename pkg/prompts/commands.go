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

	"github.com/godeps/agentkit/pkg/runtime/commands"
	"gopkg.in/yaml.v3"
)

var commandNameRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// commandMetadata describes optional YAML frontmatter fields.
type commandMetadata struct {
	Name                   string `yaml:"name"`
	Description            string `yaml:"description"`
	AllowedTools           string `yaml:"allowed-tools"`
	ArgumentHint           string `yaml:"argument-hint"`
	Model                  string `yaml:"model"`
	DisableModelInvocation bool   `yaml:"disable-model-invocation"`
}

// parseCommands parses all commands from the given directory in the filesystem.
func parseCommands(fsys fs.FS, dir string, validate bool) ([]CommandRegistration, []error) {
	var (
		regs   []CommandRegistration
		errs   []error
		merged = map[string]commandFile{}
	)

	info, err := fs.Stat(fsys, dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, []error{fmt.Errorf("prompts: stat commands dir %s: %w", dir, err)}
	}
	if !info.IsDir() {
		return nil, []error{fmt.Errorf("prompts: commands path %s is not a directory", dir)}
	}

	walkErr := fs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			errs = append(errs, fmt.Errorf("prompts: walk commands %s: %w", path, walkErr))
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(d.Name())) != ".md" {
			return nil
		}

		fallback := strings.ToLower(strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())))
		file, parseErr := parseCommandFile(fsys, path, fallback, validate)
		if parseErr != nil {
			errs = append(errs, parseErr)
			return nil
		}
		if _, exists := merged[file.name]; exists {
			errs = append(errs, fmt.Errorf("prompts: duplicate command %q in %s", file.name, dir))
			return nil
		}
		merged[file.name] = file
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
		def := commands.Definition{
			Name:        file.name,
			Description: strings.TrimSpace(file.meta.Description),
		}

		body := file.body
		meta := file.meta
		path := file.path
		handler := commands.HandlerFunc(func(_ context.Context, inv commands.Invocation) (commands.Result, error) {
			rendered := applyCommandArguments(body, inv.Args)
			res := commands.Result{Output: rendered}
			metadata := buildCommandMetadataMap(meta, path)
			if len(metadata) > 0 {
				res.Metadata = metadata
			}
			return res, nil
		})

		regs = append(regs, CommandRegistration{
			Definition: def,
			Handler:    handler,
		})
	}

	return regs, errs
}

type commandFile struct {
	name string
	path string
	meta commandMetadata
	body string
}

func parseCommandFile(fsys fs.FS, path, fallback string, validate bool) (commandFile, error) {
	content, err := fs.ReadFile(fsys, path)
	if err != nil {
		return commandFile{}, fmt.Errorf("prompts: read command %s: %w", path, err)
	}

	meta, body, err := parseCommandFrontMatter(string(content))
	if err != nil {
		return commandFile{}, fmt.Errorf("prompts: parse command %s: %w", path, err)
	}

	name := strings.ToLower(strings.TrimSpace(meta.Name))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(fallback))
	}

	if validate && !commandNameRegexp.MatchString(name) {
		return commandFile{}, fmt.Errorf("prompts: invalid command name %q from file %s", name, path)
	}

	meta.Name = name
	return commandFile{
		name: name,
		path: path,
		meta: meta,
		body: body,
	}, nil
}

func parseCommandFrontMatter(content string) (commandMetadata, string, error) {
	trimmed := strings.TrimPrefix(content, "\uFEFF")
	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return commandMetadata{}, trimmed, nil
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return commandMetadata{}, "", errors.New("prompts: missing closing frontmatter separator")
	}

	metaText := strings.Join(lines[1:end], "\n")
	var meta commandMetadata
	if err := yaml.Unmarshal([]byte(metaText), &meta); err != nil {
		return commandMetadata{}, "", err
	}

	body := strings.Join(lines[end+1:], "\n")
	body = strings.TrimPrefix(body, "\n")
	return meta, body, nil
}

func buildCommandMetadataMap(meta commandMetadata, path string) map[string]any {
	out := map[string]any{}
	if meta.AllowedTools != "" {
		out["allowed-tools"] = meta.AllowedTools
	}
	if meta.ArgumentHint != "" {
		out["argument-hint"] = meta.ArgumentHint
	}
	if meta.Model != "" {
		out["model"] = meta.Model
	}
	if meta.DisableModelInvocation {
		out["disable-model-invocation"] = true
	}
	if path != "" {
		out["source"] = path
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func applyCommandArguments(body string, args []string) string {
	if len(args) == 0 {
		return body
	}
	return strings.ReplaceAll(body, "$ARGUMENTS", strings.Join(args, " "))
}
