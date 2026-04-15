package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Reply struct {
	Author string
	Text   string
}

type Thread struct {
	TS          string
	ChannelID   string
	ChannelName string
	Author      string
	Text        string
	ReplyCount  int
	Status      string
	SlackURL    string
	Replies     []Reply
}

func LoadInbox(db *sql.DB) ([]Thread, error) {
	workspaceURL := getWorkspaceURL(db)
	userMap, err := loadUsers(db)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT m.ts, m.channel_id, c.name, m.user_id, m.text, m.reply_count, m.status, m.replies_json
		FROM messages m
		JOIN channels c ON c.id = m.channel_id
		WHERE m.status = 'unread'
		ORDER BY m.ts ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []Thread
	for rows.Next() {
		var t Thread
		var repliesJSON string
		if err := rows.Scan(&t.TS, &t.ChannelID, &t.ChannelName, &t.Author, &t.Text, &t.ReplyCount, &t.Status, &repliesJSON); err != nil {
			return nil, err
		}
		t.Author = resolveUser(t.Author, userMap)
		t.Text = cleanMarkup(t.Text, userMap)
		t.SlackURL = slackURL(workspaceURL, t.ChannelID, t.TS)
		t.Replies = parseReplies(repliesJSON, userMap)
		threads = append(threads, t)
	}
	return threads, nil
}

func LoadThread(db *sql.DB, ts string) (Thread, error) {
	workspaceURL := getWorkspaceURL(db)
	userMap, err := loadUsers(db)
	if err != nil {
		return Thread{}, err
	}

	var t Thread
	var repliesJSON string
	err = db.QueryRow(`
		SELECT m.ts, m.channel_id, c.name, m.user_id, m.text, m.reply_count, m.status, m.replies_json
		FROM messages m
		JOIN channels c ON c.id = m.channel_id
		WHERE m.ts = ?`, ts).
		Scan(&t.TS, &t.ChannelID, &t.ChannelName, &t.Author, &t.Text, &t.ReplyCount, &t.Status, &repliesJSON)
	if err != nil {
		return Thread{}, fmt.Errorf("thread %s not found: %w", ts, err)
	}
	t.Author = resolveUser(t.Author, userMap)
	t.Text = cleanMarkup(t.Text, userMap)
	t.SlackURL = slackURL(workspaceURL, t.ChannelID, t.TS)
	t.Replies = parseReplies(repliesJSON, userMap)
	return t, nil
}

func SetStatus(db *sql.DB, ts, status string) error {
	_, err := db.Exec("UPDATE messages SET status=? WHERE ts=?", status, ts)
	return err
}

func FormatTime(ts string) string {
	f, err := strconv.ParseFloat(ts, 64)
	if err != nil {
		return ts
	}
	return time.Unix(int64(f), 0).Format("2006-01-02 15:04")
}

// --- helpers ---

func getWorkspaceURL(db *sql.DB) string {
	var url string
	db.QueryRow("SELECT value FROM config WHERE key='workspace_url'").Scan(&url)
	return url
}

func loadUsers(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query("SELECT id, real_name FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var id, name string
		rows.Scan(&id, &name)
		m[id] = name
	}
	return m, nil
}

func resolveUser(id string, userMap map[string]string) string {
	if name, ok := userMap[id]; ok && name != "" {
		return name
	}
	return id
}

func slackURL(workspaceURL, channelID, ts string) string {
	p := "p" + strings.ReplaceAll(ts, ".", "")
	return fmt.Sprintf("%sarchives/%s/%s", workspaceURL, channelID, p)
}

func parseReplies(repliesJSON string, userMap map[string]string) []Reply {
	if repliesJSON == "" || repliesJSON == "[]" {
		return nil
	}
	var raw []struct {
		User string `json:"user"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(repliesJSON), &raw); err != nil {
		return nil
	}
	var replies []Reply
	for _, r := range raw {
		replies = append(replies, Reply{
			Author: resolveUser(r.User, userMap),
			Text:   cleanMarkup(r.Text, userMap),
		})
	}
	return replies
}

var (
	mentionRE   = regexp.MustCompile(`<@(U[A-Z0-9]+)>`)
	linkRE      = regexp.MustCompile(`<([^|>]+)(?:\|([^>]*))?>>?`)
	channelRE   = regexp.MustCompile(`<#[A-Z0-9]+\|([^>]+)>`)
	broadcastRE = regexp.MustCompile(`<!(\w+)>`)
)

func cleanMarkup(text string, userMap map[string]string) string {
	text = broadcastRE.ReplaceAllString(text, "@$1")
	text = channelRE.ReplaceAllString(text, "#$1")
	text = mentionRE.ReplaceAllStringFunc(text, func(s string) string {
		uid := mentionRE.FindStringSubmatch(s)[1]
		if name, ok := userMap[uid]; ok {
			return "@" + name
		}
		return s
	})
	text = linkRE.ReplaceAllStringFunc(text, func(s string) string {
		m := linkRE.FindStringSubmatch(s)
		url, display := m[1], m[2]
		if display != "" && display != url {
			return "[" + display + "](" + url + ")"
		}
		return url
	})
	return text
}
