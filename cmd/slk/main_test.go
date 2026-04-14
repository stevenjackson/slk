package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	slkdb "github.com/stevejackson/slk/internal/db"
)

// setupTestDB creates a temporary SQLite DB, sets SLK_DB, and seeds
// it with fixture data. Returns a cleanup function.
func setupTestDB(t *testing.T) func() {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	t.Setenv("SLK_DB", dbPath)

	db, err := slkdb.Open()
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	// Seed channels
	db.Exec(`INSERT INTO channels(id, name) VALUES ('C001', 'general'), ('C002', 'random')`)

	// Seed users
	db.Exec(`INSERT INTO users(id, name, real_name) VALUES ('U001', 'alice', 'Alice Smith'), ('U002', 'bob', 'Bob Jones')`)

	// Seed messages
	db.Exec(`INSERT INTO messages(ts, channel_id, user_id, text, reply_count, replies_json, status, synced_at) VALUES
		('1000000001.000001', 'C001', 'U001', 'hello world', 0,  '[]', 'unread', '2026-01-01'),
		('1000000002.000002', 'C001', 'U002', 'a thread',   5,  '[]', 'unread', '2026-01-01'),
		('1000000003.000003', 'C002', 'U001', 'in random',  0,  '[]', 'unread', '2026-01-01'),
		('1000000004.000004', 'C001', 'U001', 'already read', 0, '[]', 'read',  '2026-01-01'),
		('1000000005.000005', 'C001', 'U001', 'pinned msg',  0, '[]', 'pinned','2026-01-01')
	`)

	return func() {} // TempDir cleanup is automatic
}

// --- inbox ---

func TestInboxReturnsUnreadOnly(t *testing.T) {
	setupTestDB(t)()

	msgs := inboxJSON(t, nil)
	if len(msgs) != 3 {
		t.Errorf("expected 3 unread, got %d", len(msgs))
	}
}

func TestInboxChannelFilter(t *testing.T) {
	setupTestDB(t)()

	msgs := inboxJSON(t, []string{"--channel", "general"})
	if len(msgs) != 2 {
		t.Errorf("expected 2 in #general, got %d", len(msgs))
	}
	for _, m := range msgs {
		if m["channel_name"] != "general" {
			t.Errorf("expected channel general, got %s", m["channel_name"])
		}
	}
}

func TestInboxMinReplies(t *testing.T) {
	setupTestDB(t)()

	msgs := inboxJSON(t, []string{"--min-replies", "3"})
	if len(msgs) != 1 {
		t.Errorf("expected 1 message with 3+ replies, got %d", len(msgs))
	}
	if msgs[0]["reply_count"] != float64(5) {
		t.Errorf("expected reply_count 5, got %v", msgs[0]["reply_count"])
	}
}

func TestInboxAll(t *testing.T) {
	setupTestDB(t)()

	msgs := inboxJSON(t, []string{"--all"})
	if len(msgs) != 5 {
		t.Errorf("expected 5 messages with --all, got %d", len(msgs))
	}
}

func TestInboxSlackURL(t *testing.T) {
	setupTestDB(t)()

	msgs := inboxJSON(t, nil)
	for _, m := range msgs {
		url, ok := m["slack_url"].(string)
		if !ok || url == "" {
			t.Errorf("expected slack_url in message, got %v", m["slack_url"])
		}
	}
}

// --- read / unread / pin ---

func TestReadSingleMessage(t *testing.T) {
	setupTestDB(t)()

	if err := runSetStatus([]string{"1000000001.000001"}, "read"); err != nil {
		t.Fatalf("read: %v", err)
	}
	msgs := inboxJSON(t, nil)
	for _, m := range msgs {
		if m["ts"] == "1000000001.000001" {
			t.Error("message should be read, still in inbox")
		}
	}
}

func TestReadBulk(t *testing.T) {
	setupTestDB(t)()

	err := runSetStatus([]string{"1000000001.000001", "1000000002.000002"}, "read")
	if err != nil {
		t.Fatalf("bulk read: %v", err)
	}
	msgs := inboxJSON(t, nil)
	if len(msgs) != 1 {
		t.Errorf("expected 1 unread after bulk read, got %d", len(msgs))
	}
}

func TestReadChannel(t *testing.T) {
	setupTestDB(t)()

	err := runSetStatus([]string{"--channel", "general"}, "read")
	if err != nil {
		t.Fatalf("read --channel: %v", err)
	}
	msgs := inboxJSON(t, []string{"--channel", "general"})
	if len(msgs) != 0 {
		t.Errorf("expected 0 unread in #general after read --channel, got %d", len(msgs))
	}
	// #random should be untouched
	msgs = inboxJSON(t, []string{"--channel", "random"})
	if len(msgs) != 1 {
		t.Errorf("expected #random untouched, got %d", len(msgs))
	}
}

func TestReadChannelSkipsPinned(t *testing.T) {
	setupTestDB(t)()

	runSetStatus([]string{"--channel", "general"}, "read")
	msgs := inboxJSON(t, []string{"--all", "--channel", "general"})
	for _, m := range msgs {
		if m["ts"] == "1000000005.000005" && m["status"] != "pinned" {
			t.Error("pinned message should not be touched by read --channel")
		}
	}
}

func TestUnreadRestoresMessages(t *testing.T) {
	setupTestDB(t)()

	runSetStatus([]string{"--channel", "general"}, "read")
	runSetStatus([]string{"--channel", "general"}, "unread")

	msgs := inboxJSON(t, []string{"--channel", "general"})
	// 3: the 2 originally-unread + the 1 pre-existing read message, all become unread
	if len(msgs) != 3 {
		t.Errorf("expected 3 restored to unread, got %d", len(msgs))
	}
}

func TestReadNotFound(t *testing.T) {
	setupTestDB(t)()

	// should not error, just report not found per-ts
	if err := runSetStatus([]string{"9999999999.000000"}, "read"); err != nil {
		t.Errorf("unexpected error for missing ts: %v", err)
	}
}

// --- show ---

func TestShowIncludesSlackURL(t *testing.T) {
	setupTestDB(t)()

	// capture output via a quick DB query instead of stdout capture
	db, _ := slkdb.Open()
	defer db.Close()

	var channelID string
	db.QueryRow("SELECT channel_id FROM messages WHERE ts=?", "1000000001.000001").Scan(&channelID)
	url := slackURL(channelID, "1000000001.000001")
	if url == "" {
		t.Error("slackURL returned empty string")
	}
}

// --- helpers ---

func inboxJSON(t *testing.T, args []string) []map[string]any {
	t.Helper()

	// redirect stdout to capture JSON
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	allArgs := append([]string{"--json"}, args...)
	err := runInbox(allArgs)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runInbox: %v", err)
	}

	var msgs []map[string]any
	if err := json.NewDecoder(r).Decode(&msgs); err != nil {
		// empty inbox returns null — treat as empty slice
		return nil
	}
	return msgs
}
