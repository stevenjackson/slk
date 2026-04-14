package slack

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/slack-go/slack"
)

type Ingester struct {
	client *slack.Client
	db     *sql.DB
}

func NewIngester(token string, db *sql.DB) *Ingester {
	return &Ingester{
		client: slack.New(token),
		db:     db,
	}
}

// SyncUsers fetches and caches workspace users. Skips if already populated
// unless force=true.
func (g *Ingester) SyncUsers(force bool) error {
	if !force {
		var count int
		g.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
		if count > 0 {
			return nil
		}
	}
	fmt.Println("syncing users...")
	members, err := g.client.GetUsers(
		slack.GetUsersOptionLimit(500),
	)
	if err != nil {
		return fmt.Errorf("users.list: %w", err)
	}
	for _, m := range members {
		name := m.Name
		real := m.Profile.DisplayName
		if real == "" {
			real = m.RealName
		}
		if real == "" {
			real = name
		}
		_, err := g.db.Exec(
			`INSERT INTO users(id,name,real_name) VALUES(?,?,?)
			 ON CONFLICT(id) DO UPDATE SET name=excluded.name, real_name=excluded.real_name`,
			m.ID, name, real,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// SyncChannel pulls messages from oldest (unix timestamp string) to now.
func (g *Ingester) SyncChannel(channelID, channelName, oldest string) error {
	fmt.Printf("syncing #%s...\n", channelName)

	params := &slack.GetConversationHistoryParameters{
		ChannelID: channelID,
		Oldest:    oldest,
		Limit:     200,
	}

	var latestTS string
	var synced int

	for {
		resp, err := g.client.GetConversationHistory(params)
		if err != nil {
			return fmt.Errorf("conversations.history #%s: %w", channelName, err)
		}

		now := time.Now().UTC().Format(time.RFC3339)

		for _, msg := range resp.Messages {
			// Skip system subtypes (joins, leaves, etc.)
			if msg.SubType != "" && msg.SubType != "file_share" && msg.SubType != "thread_broadcast" {
				continue
			}

			ts := msg.Timestamp
			if latestTS == "" || tsFloat(ts) > tsFloat(latestTS) {
				latestTS = ts
			}

			userID := msg.User
			if userID == "" {
				userID = msg.BotID
			}

			text := extractText(msg.Msg)
			replyCount := msg.ReplyCount
			repliesJSON := "[]"

			if replyCount > 0 {
				replies, err := g.fetchThread(channelID, ts)
				if err != nil {
					fmt.Printf("  warn: thread %s: %v\n", ts, err)
				} else if len(replies) > 1 {
					// replies[0] is parent — skip it, store rest
					b, _ := json.Marshal(replies[1:])
					repliesJSON = string(b)
					// Use parent from thread response for authoritative text
					parent := replies[0]
					text = extractText(parent)
					if parent.User != "" {
						userID = parent.User
					}
				}
			}

			_, err := g.db.Exec(`
				INSERT INTO messages(ts, channel_id, user_id, text, reply_count, replies_json, status, synced_at)
				VALUES(?,?,?,?,?,?,'unread',?)
				ON CONFLICT(ts) DO UPDATE SET
					text         = excluded.text,
					user_id      = excluded.user_id,
					reply_count  = excluded.reply_count,
					replies_json = excluded.replies_json,
					synced_at    = excluded.synced_at,
					status = CASE
						WHEN text != excluded.text OR replies_json != excluded.replies_json THEN 'unread'
						ELSE status
					END`,
				ts, channelID, userID, text, replyCount, repliesJSON, now,
			)
			if err != nil {
				return err
			}
			synced++
		}

		if !resp.HasMore {
			break
		}
		params.Cursor = resp.ResponseMetaData.NextCursor
		time.Sleep(500 * time.Millisecond)
	}

	if latestTS != "" {
		g.db.Exec("UPDATE channels SET last_synced_ts=? WHERE id=?", latestTS, channelID)
	}
	fmt.Printf("  %d messages\n", synced)
	return nil
}

func (g *Ingester) fetchThread(channelID, threadTS string) ([]slack.Msg, error) {
	params := &slack.GetConversationRepliesParameters{
		ChannelID: channelID,
		Timestamp: threadTS,
		Limit:     1000,
	}
	var all []slack.Msg
	for {
		msgs, hasMore, cursor, err := g.client.GetConversationReplies(params)
		if err != nil {
			return nil, err
		}
		for _, m := range msgs {
			all = append(all, m.Msg)
		}
		if !hasMore {
			break
		}
		params.Cursor = cursor
		time.Sleep(200 * time.Millisecond)
	}
	return all, nil
}

// extractText pulls message text and appends file/attachment metadata.
func extractText(msg slack.Msg) string {
	text := msg.Text
	for _, f := range msg.Files {
		text += fmt.Sprintf("\n[file: %s (%s)]", f.Name, f.Mimetype)
	}
	for _, a := range msg.Attachments {
		title := a.Title
		if title == "" {
			title = a.Fallback
		}
		if title != "" {
			text += fmt.Sprintf("\n[attachment: %s]", title)
		}
	}
	return text
}

func tsFloat(ts string) float64 {
	f, _ := strconv.ParseFloat(ts, 64)
	return f
}
