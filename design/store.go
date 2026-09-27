package design

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Store keeps every verdict given, keyed by task and expert.
type Store interface {
	Get(task, expert string) (Verdict, bool)
	Put(Verdict) error
}

// FileStore is a Store in an append-only JSON Lines file. A verdict put again
// for the same task and expert is appended too, and the later line wins, so
// the file is also the history of every change of mind.
type FileStore struct {
	path string

	mu       sync.Mutex
	verdicts map[storeKey]Verdict
}

type storeKey struct{ task, expert string }

// OpenStore reads the store at path; a file that does not exist yet is an
// empty store.
func OpenStore(path string) (*FileStore, error) {
	s := &FileStore{path: path, verdicts: map[storeKey]Verdict{}}
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	} else if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for n := 1; sc.Scan(); n++ {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var v Verdict
		if err := json.Unmarshal(sc.Bytes(), &v); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		s.verdicts[storeKey{v.Task, v.Expert}] = v
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}

func (s *FileStore) Get(task, expert string) (Verdict, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.verdicts[storeKey{task, expert}]
	return v, ok
}

// Put appends v to the file before remembering it, so a verdict the caller
// was told is stored survives the process being killed right after.
func (s *FileStore) Put(v Verdict) error {
	line, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	s.verdicts[storeKey{v.Task, v.Expert}] = v
	return nil
}

// Verdicts returns every current verdict on a task, one per expert, ordered
// by expert so that anything summed over them comes out the same every run.
func (s *FileStore) Verdicts(task string) []Verdict {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Verdict
	for k, v := range s.verdicts {
		if k.task == task {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Expert < out[j].Expert })
	return out
}
