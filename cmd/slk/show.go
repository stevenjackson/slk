package main

import (
	"encoding/json"
	"fmt"

	slkdb "github.com/stevejackson/slk/internal/db"
)

type ThreadMessage struct {
	UserID string `json:"user"`
	Text   string `json:"text"`
}

type ShowResult struct {
	Message
	Replies []ThreadMessage `json:"replies"`
}

func runShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: slk show <ts> [--json]")
	}
	ts := args[0]
	asJSON := false
	for _, a := range args[1:] {
		if a == "--json" {
			asJSON = true
		}
	}

	db, err := slkdb.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	userMap, err := loadUsers(db)
	if err != nil {
		return err
	}

	var m Message
	var repliesJSON string
	err = db.QueryRow(`
		SELECT m.ts, m.channel_id, c.name, m.user_id, m.text, m.reply_count, m.status, m.replies_json
		FROM messages m
		JOIN channels c ON c.id = m.channel_id
		WHERE m.ts = ?`, ts).
		Scan(&m.TS, &m.ChannelID, &m.ChannelName, &m.UserID, &m.Text, &m.ReplyCount, &m.Status, &repliesJSON)
	if err != nil {
		return fmt.Errorf("message %s not found", ts)
	}

	m.Author = resolveUser(m.UserID, userMap)
	m.Text = cleanMarkup(m.Text, userMap)
	m.Time = formatTS(m.TS)
	m.SlackURL = slackURL(m.ChannelID, m.TS)

	var rawReplies []json.RawMessage
	json.Unmarshal([]byte(repliesJSON), &rawReplies)

	var replies []ThreadMessage
	for _, r := range rawReplies {
		var raw struct {
			User string `json:"user"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(r, &raw); err == nil {
			replies = append(replies, ThreadMessage{
				UserID: resolveUser(raw.User, userMap),
				Text:   cleanMarkup(raw.Text, userMap),
			})
		}
	}

	if asJSON {
		result := ShowResult{Message: m, Replies: replies}
		enc := json.NewEncoder(out())
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("ts:      %s\n", m.TS)
	fmt.Printf("channel: #%s\n", m.ChannelName)
	fmt.Printf("author:  %s\n", m.Author)
	fmt.Printf("time:    %s\n", m.Time)
	fmt.Printf("status:  %s\n", m.Status)
	fmt.Printf("url:     %s\n", m.SlackURL)
	fmt.Println()
	fmt.Println(m.Text)

	if len(replies) > 0 {
		fmt.Printf("\n--- %d replies ---\n", len(replies))
		for _, r := range replies {
			fmt.Printf("\n%s: %s\n", r.UserID, r.Text)
		}
	}
	return nil
}
