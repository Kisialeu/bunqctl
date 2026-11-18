package secret

import (
	"fmt"
	"sync"
)

// Memory is an in-process secret store for tests. Its zero value is ready
// to use.
type Memory struct {
	mu sync.Mutex
	m  map[string]string
}

// Get returns the secret name.
func (s *Memory) Get(name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[name]
	if !ok {
		return "", fmt.Errorf("%s: %w", name, ErrNotFound)
	}
	return v, nil
}

// Set stores the secret name.
func (s *Memory) Set(name, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string]string{}
	}
	s.m[name] = value
	return nil
}

// Delete removes the secret name.
func (s *Memory) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, name)
	return nil
}
