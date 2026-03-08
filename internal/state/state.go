package state

import (
	"bufio"
	"os"
	"path/filepath"
	"sync"
)

type State struct {
	mu        sync.RWMutex
	filePath  string
	processed map[string]bool
}

func Load(filePath string) (*State, error) {
	s := &State{
		filePath:  filePath,
		processed: make(map[string]bool),
	}

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			s.processed[line] = true
		}
	}

	return s, scanner.Err()
}

func (s *State) HasProcessed(path, label string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := path + "|" + label
	return s.processed[key]
}

func (s *State) MarkProcessed(path, label string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := path + "|" + label

	if s.processed[key] {
		return nil
	}
	s.processed[key] = true

	if err := os.MkdirAll(filepath.Dir(s.filePath), 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(s.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(key + "\n")
	return err
}
