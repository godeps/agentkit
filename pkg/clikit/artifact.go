package clikit

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type artifactInfo struct {
	Path       string
	Dimensions string
	Format     string
	Size       string
}

var (
	imagePathPattern   = regexp.MustCompile(`(?:^|[\s"'` + "`" + `])(output/[^\s"'` + "`" + `]+\.(?:png|jpg|jpeg|webp))(?:$|[\s"'` + "`" + `])`)
	dimensionPattern   = regexp.MustCompile(`(\d{2,5})\s*[xX]\s*(\d{2,5})`)
	imageFormatPattern = regexp.MustCompile(`\b(PNG|JPG|JPEG|WEBP)\b`)
)

func detectArtifactInfo(v any) (artifactInfo, bool) {
	var info artifactInfo
	visitArtifactFields(v, &info)
	if info.Path == "" {
		return artifactInfo{}, false
	}
	return info, true
}

func visitArtifactFields(v any, info *artifactInfo) {
	if info == nil || v == nil {
		return
	}
	switch typed := v.(type) {
	case map[string]any:
		for k, val := range typed {
			key := strings.ToLower(strings.TrimSpace(k))
			if info.Path == "" && (strings.Contains(key, "path") || key == "saved") {
				if s, ok := val.(string); ok {
					tryExtractFromString(s, info)
				}
			}
			if info.Dimensions == "" && (key == "dimensions" || key == "size") {
				if s, ok := val.(string); ok {
					tryExtractFromString(s, info)
				}
			}
			if info.Format == "" && key == "format" {
				if s, ok := val.(string); ok {
					info.Format = strings.ToUpper(strings.TrimSpace(s))
				}
			}
			visitArtifactFields(val, info)
		}
	case []any:
		for _, it := range typed {
			visitArtifactFields(it, info)
		}
	case string:
		tryExtractFromString(typed, info)
		var nested map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(typed)), &nested); err == nil {
			visitArtifactFields(nested, info)
		}
	}
}

func tryExtractFromString(s string, info *artifactInfo) {
	if info == nil {
		return
	}
	txt := strings.TrimSpace(s)
	if txt == "" {
		return
	}
	if info.Path == "" {
		if matches := imagePathPattern.FindAllStringSubmatch(txt, -1); len(matches) > 0 {
			last := matches[len(matches)-1]
			if len(last) > 1 {
				info.Path = last[1]
			}
		}
	}
	if info.Dimensions == "" {
		if m := dimensionPattern.FindStringSubmatch(txt); len(m) > 2 {
			info.Dimensions = m[1] + " x " + m[2]
		}
	}
	if info.Format == "" {
		if m := imageFormatPattern.FindStringSubmatch(strings.ToUpper(txt)); len(m) > 1 {
			info.Format = m[1]
		}
	}
	if info.Size == "" {
		lower := strings.ToLower(txt)
		if strings.Contains(lower, "mb") || strings.Contains(lower, "kb") {
			info.Size = txt
		}
	}
}

func printArtifactCard(out io.Writer, ansi bool, info artifactInfo) {
	if out == nil || strings.TrimSpace(info.Path) == "" {
		return
	}
	printBlockHeader(out, "RESULT")
	title := "Generated File"
	if ansi {
		title = colorize(title, ansiBold+ansiGreen, true)
	}
	fmt.Fprintf(out, "%s\n", title)
	pathLabel := "path"
	pathValue := info.Path
	if ansi {
		pathLabel = colorize(pathLabel, ansiCyan, true)
		pathValue = colorize(pathValue, ansiBold+ansiCyan, true)
	}
	fmt.Fprintf(out, "%s: %s\n", pathLabel, pathValue)
	if strings.TrimSpace(info.Dimensions) != "" {
		label := "dimensions"
		if ansi {
			label = colorize(label, ansiDim, true)
		}
		fmt.Fprintf(out, "%s: %s\n", label, info.Dimensions)
	}
	if strings.TrimSpace(info.Format) != "" {
		label := "format"
		if ansi {
			label = colorize(label, ansiDim, true)
		}
		fmt.Fprintf(out, "%s: %s\n", label, info.Format)
	}
	if strings.TrimSpace(info.Size) != "" {
		label := "size"
		if ansi {
			label = colorize(label, ansiDim, true)
		}
		fmt.Fprintf(out, "%s: %s\n", label, info.Size)
	}
	printBlockFooter(out)
}
