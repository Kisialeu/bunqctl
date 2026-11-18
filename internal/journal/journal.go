// Package journal remembers recent money writes so that an accidental second
// run (a retried script, a double cron entry) is refused.
package journal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/Kisialeu/bunqctl/internal/atomicfile"
)

// keep bounds how long entries stay in the file.
const keep = 24 * time.Hour

type entry struct {
	Key  string    `json:"key"`
	Time time.Time `json:"time"`
}

// Journal is a file of recent request fingerprints.
type Journal struct {
	path string
	now  func() time.Time
}

// New returns the journal stored at path.
func New(path string) *Journal { return &Journal{path: path, now: time.Now} }

// Key returns a fingerprint of an operation and its request body.
func Key(op string, body any) (string, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("journal key: %w", err)
	}
	sum := sha256.Sum256(append([]byte(op+"\n"), b...))
	return hex.EncodeToString(sum[:]), nil
}

// Seen reports when key was recorded within the last window, if it was.
func (j *Journal) Seen(key string, window time.Duration) (time.Time, bool, error) {
	entries, err := j.load()
	if err != nil {
		return time.Time{}, false, err
	}
	cutoff := j.now().Add(-window)
	for _, e := range entries {
		if e.Key == key && e.Time.After(cutoff) {
			return e.Time, true, nil
		}
	}
	return time.Time{}, false, nil
}

// Add records key now and drops entries older than a day.
func (j *Journal) Add(key string) error {
	entries, err := j.load()
	if err != nil {
		return err
	}
	now := j.now()
	kept := []entry{{Key: key, Time: now}}
	for _, e := range entries {
		if now.Sub(e.Time) < keep {
			kept = append(kept, e)
		}
	}
	b, err := json.MarshalIndent(kept, "", "  ")
	if err != nil {
		return fmt.Errorf("encode journal: %w", err)
	}
	return atomicfile.Write(j.path, append(b, '\n'))
}

func (j *Journal) load() ([]entry, error) {
	b, err := os.ReadFile(j.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read journal: %w", err)
	}
	var entries []entry
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil, fmt.Errorf("decode journal %s: %w", j.path, err)
	}
	return entries, nil
}
