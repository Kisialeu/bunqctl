package journal

import (
	"path/filepath"
	"testing"
	"time"
)

func Test_Journal_seenWithinWindow(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	j := New(filepath.Join(t.TempDir(), "j.json"))
	j.now = func() time.Time { return now }

	key, err := Key("move", map[string]string{"amount": "10.00"})
	if err != nil {
		t.Fatal(err)
	}
	if _, seen, err := j.Seen(key, 10*time.Minute); err != nil || seen {
		t.Fatalf("empty journal: seen=%v err=%v", seen, err)
	}
	if err := j.Add(key); err != nil {
		t.Fatal(err)
	}

	now = now.Add(5 * time.Minute)
	if _, seen, _ := j.Seen(key, 10*time.Minute); !seen {
		t.Error("not seen 5 minutes later")
	}
	other, _ := Key("move", map[string]string{"amount": "10.01"})
	if _, seen, _ := j.Seen(other, 10*time.Minute); seen {
		t.Error("a different body was reported as seen")
	}

	now = now.Add(10 * time.Minute)
	if _, seen, _ := j.Seen(key, 10*time.Minute); seen {
		t.Error("still seen after the window")
	}
}

func Test_Journal_prunesOldEntries(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	j := New(filepath.Join(t.TempDir(), "j.json"))
	j.now = func() time.Time { return now }

	if err := j.Add("old"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(25 * time.Hour)
	if err := j.Add("new"); err != nil {
		t.Fatal(err)
	}
	entries, err := j.load()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Key != "new" {
		t.Errorf("entries = %+v, want only the new one", entries)
	}
}
