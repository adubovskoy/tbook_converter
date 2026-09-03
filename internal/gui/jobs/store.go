package jobs

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Store persists the job list as one JSON array. Callers serialize access
// (the Manager holds its own lock); Store only guarantees atomic writes and
// never fails the app on a corrupt file.
type Store struct {
	path string
}

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) Load() ([]*Job, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var jobs []*Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		// A corrupt history must never brick the app: keep the evidence, start empty.
		_ = os.Rename(s.path, s.path+".bak")
		return nil, nil
	}
	return jobs, nil
}

func (s *Store) Save(jobs []*Job) error {
	if jobs == nil {
		jobs = []*Job{}
	}
	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
