package checkpoint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DiskStore struct {
	dir string
}

func NewDiskStore(dir string) *DiskStore {
	return &DiskStore{dir: strings.TrimSpace(dir)}
}

func (s *DiskStore) Save(_ context.Context, rec Record) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	data, err := EncodeRecord(rec)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, sanitizeID(rec.ID)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	path := s.filePath(rec.ID)
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(path)
		if retry := os.Rename(tmpPath, path); retry != nil {
			return retry
		}
	}
	return nil
}

func (s *DiskStore) Load(_ context.Context, id string) (Record, error) {
	data, err := os.ReadFile(s.filePath(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Record{}, ErrNotFound
		}
		return Record{}, err
	}
	return DecodeRecord(data)
}

func (s *DiskStore) Delete(_ context.Context, id string) error {
	if err := os.Remove(s.filePath(id)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *DiskStore) filePath(id string) string {
	return filepath.Join(s.dir, sanitizeID(id)+".json")
}

func sanitizeID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "checkpoint"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", "..", "_")
	return replacer.Replace(id)
}

func DiskStoreDir(projectRoot, configRoot string) string {
	projectRoot = strings.TrimSpace(projectRoot)
	configRoot = strings.TrimSpace(configRoot)
	base := configRoot
	if base == "" {
		base = filepath.Join(projectRoot, ".claude")
	} else if !filepath.IsAbs(base) && projectRoot != "" {
		base = filepath.Join(projectRoot, base)
	}
	if strings.TrimSpace(base) == "" {
		return ""
	}
	return filepath.Join(base, "checkpoints")
}

func (s *DiskStore) String() string {
	return fmt.Sprintf("DiskStore(%s)", s.dir)
}
