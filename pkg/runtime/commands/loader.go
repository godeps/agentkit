package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/godeps/agentkit/pkg/config"
	"gopkg.in/yaml.v3"
)

type fileOps struct {
	readFile func(string) ([]byte, error)
	openFile func(string) (fs.File, error)
	statFile func(string) (fs.FileInfo, error)
}

type walkDirFunc func(string, fs.WalkDirFunc) error

var (
	fileOpOverridesMu sync.RWMutex
	fileOpOverrides   = struct {
		read func(string) ([]byte, error)
		open func(string) (*os.File, error)
		stat func(string) (fs.FileInfo, error)
	}{}
)

// LoaderOptions controls how commands are discovered from the filesystem.
type LoaderOptions struct {
	ProjectRoot string
	// Deprecated: user-level scanning has been removed; this field is ignored.
	UserHome string
	// Deprecated: user-level scanning has been removed; this flag is ignored.
	EnableUser bool
	FS         *config.FS
}

func resolveFileOps(opts LoaderOptions) fileOps {
	if opts.FS != nil {
		return fileOps{
			readFile: opts.FS.ReadFile,
			openFile: opts.FS.Open,
			statFile: opts.FS.Stat,
		}
	}
	return fileOps{
		readFile: readFileOverrideOrOS,
		openFile: openFileOverrideOrOS,
		statFile: statFileOverrideOrOS,
	}
}

func readFileOverrideOrOS(path string) ([]byte, error) {
	fileOpOverridesMu.RLock()
	override := fileOpOverrides.read
	fileOpOverridesMu.RUnlock()
	if override != nil {
		return override(path)
	}
	return os.ReadFile(path)
}

func openFileOverrideOrOS(path string) (fs.File, error) {
	fileOpOverridesMu.RLock()
	override := fileOpOverrides.open
	fileOpOverridesMu.RUnlock()
	if override != nil {
		return override(path)
	}
	return os.Open(path)
}

func statFileOverrideOrOS(path string) (fs.FileInfo, error) {
	fileOpOverridesMu.RLock()
	override := fileOpOverrides.stat
	fileOpOverridesMu.RUnlock()
	if override != nil {
		return override(path)
	}
	return os.Stat(path)
}

func resolveWalkDirFunc(opts LoaderOptions) walkDirFunc {
	if opts.FS != nil {
		return opts.FS.WalkDir
	}
	return filepath.WalkDir
}

// CommandFile captures an on-disk command definition.
type CommandFile struct {
	Name     string
	Path     string
	Metadata CommandMetadata
}

// CommandMetadata describes optional YAML frontmatter fields.
type CommandMetadata struct {
	Name                   string `yaml:"name"`
	Description            string `yaml:"description"`
	AllowedTools           string `yaml:"allowed-tools"`
	ArgumentHint           string `yaml:"argument-hint"`
	Model                  string `yaml:"model"`
	DisableModelInvocation bool   `yaml:"disable-model-invocation"`
}

// CommandRegistration wires a definition to its handler.
type CommandRegistration struct {
	Definition Definition
	Handler    Handler
}

// LoadFromFS loads slash commands from the filesystem. It never returns a nil
// registrations slice. Errors are aggregated so a single bad file doesn't
// prevent others from loading.
func LoadFromFS(opts LoaderOptions) ([]CommandRegistration, []error) {
	var (
		registrations []CommandRegistration
		merged        = map[string]CommandFile{}
		errs          []error
	)

	ops := resolveFileOps(opts)
	walk := resolveWalkDirFunc(opts)

	projectDir := filepath.Join(opts.ProjectRoot, ".claude", "commands")
	files, loadErrs := loadCommandDir(projectDir, ops, walk)
	errs = append(errs, loadErrs...)
	for name, file := range files {
		merged[name] = file
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
		reg := CommandRegistration{
			Definition: Definition{
				Name:        file.Name,
				Description: strings.TrimSpace(file.Metadata.Description),
			},
			Handler: buildHandler(file, ops),
		}
		registrations = append(registrations, reg)
	}

	return registrations, errs
}

func loadCommandDir(root string, ops fileOps, walk walkDirFunc) (map[string]CommandFile, []error) {
	results := map[string]CommandFile{}
	var errs []error

	info, err := ops.statFile(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return results, nil
		}
		return results, []error{fmt.Errorf("commands: stat %s: %w", root, err)}
	}
	if !info.IsDir() {
		return results, []error{fmt.Errorf("commands: path %s is not a directory", root)}
	}

	walkErr := walk(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			errs = append(errs, fmt.Errorf("commands: walk %s: %w", path, walkErr))
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(d.Name())) != ".md" {
			return nil
		}

		fallback := strings.ToLower(strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())))
		file, parseErr := parseCommandFile(path, fallback, ops)
		if parseErr != nil {
			errs = append(errs, parseErr)
			return nil
		}
		if _, exists := results[file.Name]; exists {
			errs = append(errs, fmt.Errorf("commands: duplicate command %q in %s", file.Name, root))
			return nil
		}
		results[file.Name] = file
		return nil
	})
	if walkErr != nil {
		errs = append(errs, walkErr)
	}
	return results, errs
}

func parseCommandFile(path, fallback string, ops fileOps) (CommandFile, error) {
	meta, err := readFrontMatterMetadata(path, ops)
	if err != nil {
		return CommandFile{}, fmt.Errorf("commands: parse %s: %w", path, err)
	}
	name := strings.ToLower(strings.TrimSpace(meta.Name))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(fallback))
	}
	if !validName(name) {
		return CommandFile{}, fmt.Errorf("commands: invalid command name %q from file %s", name, path)
	}
	meta.Name = name
	return CommandFile{
		Name:     name,
		Path:     path,
		Metadata: meta,
	}, nil
}

func readFrontMatterMetadata(path string, ops fileOps) (CommandMetadata, error) {
	file, err := ops.openFile(path)
	if err != nil {
		return CommandMetadata{}, fmt.Errorf("commands: read %s: %w", path, err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	first, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return CommandMetadata{}, err
	}

	first = strings.TrimPrefix(first, "\uFEFF")
	if strings.TrimSpace(first) != "---" {
		return CommandMetadata{}, nil
	}

	var lines []string
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return CommandMetadata{}, readErr
		}
		if strings.TrimSpace(line) == "---" {
			metaText := strings.Join(lines, "")
			var meta CommandMetadata
			if err := yaml.Unmarshal([]byte(metaText), &meta); err != nil {
				return CommandMetadata{}, fmt.Errorf("decode YAML: %w", err)
			}
			return meta, nil
		}
		if line != "" {
			lines = append(lines, line)
		}
		if errors.Is(readErr, io.EOF) {
			return CommandMetadata{}, errors.New("commands: missing closing frontmatter separator")
		}
	}
}

func parseFrontMatter(content string) (CommandMetadata, string, error) {
	trimmed := strings.TrimPrefix(content, "\uFEFF") // drop BOM if present
	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return CommandMetadata{}, trimmed, nil
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return CommandMetadata{}, "", errors.New("commands: missing closing frontmatter separator")
	}

	metaText := strings.Join(lines[1:end], "\n")
	var meta CommandMetadata
	if err := yaml.Unmarshal([]byte(metaText), &meta); err != nil {
		return CommandMetadata{}, "", err
	}

	body := strings.Join(lines[end+1:], "\n")
	body = strings.TrimPrefix(body, "\n")
	return meta, body, nil
}

type lazyCommandBody struct {
	path     string
	metadata CommandMetadata
	ops      fileOps

	mu         sync.Mutex
	body       string
	loadedMeta CommandMetadata
	loaded     bool
	modTime    time.Time
}

func (l *lazyCommandBody) load() (string, CommandMetadata, error) {
	info, err := l.ops.statFile(l.path)
	if err != nil {
		return "", CommandMetadata{}, fmt.Errorf("commands: stat %s: %w", l.path, err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.loaded && !info.ModTime().After(l.modTime) {
		return l.body, l.loadedMetaOrFallback(), nil
	}

	data, err := l.ops.readFile(l.path)
	if err != nil {
		return "", CommandMetadata{}, fmt.Errorf("commands: read %s: %w", l.path, err)
	}

	meta, body, err := parseFrontMatter(string(data))
	if err != nil {
		return "", CommandMetadata{}, fmt.Errorf("commands: parse %s: %w", l.path, err)
	}

	l.body = body
	l.loadedMeta = meta
	l.loaded = true
	l.modTime = info.ModTime()

	return l.body, l.loadedMetaOrFallback(), nil
}

func (l *lazyCommandBody) loadedMetaOrFallback() CommandMetadata {
	if l.loadedMeta != (CommandMetadata{}) {
		return l.loadedMeta
	}
	return l.metadata
}

func buildHandler(file CommandFile, ops fileOps) Handler {
	loader := &lazyCommandBody{
		path:     file.Path,
		metadata: file.Metadata,
		ops:      ops,
	}

	return HandlerFunc(func(_ context.Context, inv Invocation) (Result, error) {
		body, meta, err := loader.load()
		if err != nil {
			return Result{}, err
		}
		rendered := applyArguments(body, inv.Args)
		res := Result{Output: rendered}
		metadata := buildMetadataMap(meta, file.Path)
		if len(metadata) > 0 {
			res.Metadata = metadata
		}
		return res, nil
	})
}

func buildMetadataMap(meta CommandMetadata, path string) map[string]any {
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

func applyArguments(body string, args []string) string {
	if len(args) == 0 && !strings.Contains(body, "$") {
		return body
	}
	rendered := strings.ReplaceAll(body, "$ARGUMENTS", strings.Join(args, " "))
	if !strings.Contains(rendered, "$") {
		return rendered
	}
	re := regexp.MustCompile(`\$(\d+)`)
	return re.ReplaceAllStringFunc(rendered, func(match string) string {
		idx, err := strconv.Atoi(match[1:])
		if err != nil || idx <= 0 || idx > len(args) {
			return ""
		}
		return args[idx-1]
	})
}

// SetCommandFileOpsForTest swaps filesystem helpers; intended for white-box tests only.
func SetCommandFileOpsForTest(
	read func(string) ([]byte, error),
	open func(string) (*os.File, error),
	stat func(string) (fs.FileInfo, error),
) (restore func()) {
	fileOpOverridesMu.Lock()
	prev := fileOpOverrides
	if read != nil {
		fileOpOverrides.read = read
	}
	if open != nil {
		fileOpOverrides.open = open
	}
	if stat != nil {
		fileOpOverrides.stat = stat
	}
	fileOpOverridesMu.Unlock()
	return func() {
		fileOpOverridesMu.Lock()
		fileOpOverrides = prev
		fileOpOverridesMu.Unlock()
	}
}
