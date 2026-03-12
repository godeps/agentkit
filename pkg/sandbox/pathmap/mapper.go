package pathmap

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
)

// Mapper translates between guest-visible and host-visible paths for one session.
type Mapper struct {
	mounts []sandboxenv.MountSpec
}

func New(mounts []sandboxenv.MountSpec) (*Mapper, error) {
	if err := validateMounts(mounts); err != nil {
		return nil, err
	}
	cloned := append([]sandboxenv.MountSpec(nil), mounts...)
	slices.SortFunc(cloned, func(a, b sandboxenv.MountSpec) int {
		return len(b.GuestPath) - len(a.GuestPath)
	})
	return &Mapper{mounts: cloned}, nil
}

func (m *Mapper) GuestToHost(guestPath string) (string, sandboxenv.MountSpec, error) {
	if m == nil {
		return "", sandboxenv.MountSpec{}, fmt.Errorf("pathmap: mapper is nil")
	}
	guest, err := normalizeGuestPath(guestPath)
	if err != nil {
		return "", sandboxenv.MountSpec{}, err
	}
	for _, mount := range m.mounts {
		root := filepath.Clean(mount.GuestPath)
		if !withinPath(guest, root) {
			continue
		}
		suffix := strings.TrimPrefix(guest, root)
		suffix = strings.TrimPrefix(suffix, string(filepath.Separator))
		host := filepath.Clean(mount.HostPath)
		if suffix != "" {
			host = filepath.Join(host, suffix)
		}
		return host, mount, nil
	}
	return "", sandboxenv.MountSpec{}, fmt.Errorf("pathmap: guest path not mounted: %s", guest)
}

func (m *Mapper) HostToGuest(hostPath string) (string, sandboxenv.MountSpec, error) {
	if m == nil {
		return "", sandboxenv.MountSpec{}, fmt.Errorf("pathmap: mapper is nil")
	}
	host, err := filepath.Abs(strings.TrimSpace(hostPath))
	if err != nil {
		return "", sandboxenv.MountSpec{}, fmt.Errorf("pathmap: abs host path: %w", err)
	}
	host = filepath.Clean(host)
	for _, mount := range m.mounts {
		root := filepath.Clean(mount.HostPath)
		if !withinPath(host, root) {
			continue
		}
		suffix := strings.TrimPrefix(host, root)
		suffix = strings.TrimPrefix(suffix, string(filepath.Separator))
		guest := filepath.Clean(mount.GuestPath)
		if suffix != "" {
			guest = filepath.Join(guest, suffix)
		}
		return guest, mount, nil
	}
	return "", sandboxenv.MountSpec{}, fmt.Errorf("pathmap: host path not mounted: %s", host)
}

func (m *Mapper) VisibleRoots() []string {
	if m == nil {
		return nil
	}
	out := make([]string, len(m.mounts))
	for i, mount := range m.mounts {
		out[i] = mount.GuestPath
	}
	return out
}

func validateMounts(mounts []sandboxenv.MountSpec) error {
	seen := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		if strings.TrimSpace(mount.HostPath) == "" {
			return fmt.Errorf("pathmap: host path is required")
		}
		guest, err := normalizeGuestPath(mount.GuestPath)
		if err != nil {
			return err
		}
		for _, existing := range seen {
			if withinPath(guest, existing) || withinPath(existing, guest) {
				return fmt.Errorf("pathmap: overlapping guest mount paths: %s and %s", guest, existing)
			}
		}
		seen = append(seen, guest)
	}
	return nil
}

func normalizeGuestPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("pathmap: guest path is required")
	}
	if !filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("pathmap: guest path must be absolute: %s", trimmed)
	}
	clean := filepath.Clean(trimmed)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("pathmap: invalid guest path: %s", path)
	}
	return clean, nil
}

func withinPath(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	root = strings.TrimSuffix(root, string(filepath.Separator))
	return strings.HasPrefix(path, root+string(filepath.Separator))
}
