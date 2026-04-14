package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	slkdb "github.com/stevejackson/slk/internal/db"
)

type Message struct {
	TS          string `json:"ts"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	UserID      string `json:"user_id"`
	Author      string `json:"author"`
	Text        string `json:"text"`
	ReplyCount  int    `json:"reply_count"`
	Status      string `json:"status"`
	Time        string `json:"time"`
	SlackURL    string `json:"slack_url"`
}

func slackURL(channelID, ts string) string {
	return fmt.Sprintf("https://slack.com/app_redirect?channel=%s&message_ts=%s", channelID, ts)
}

type InboxCmd struct {
	Channel    string `help:"filter by channel name" short:"c"`
	MinReplies int    `help:"minimum reply count" name:"min-replies"`
	JSON       bool   `help:"output JSON" short:"j"`
	All        bool   `help:"include read and pinned messages"`
}

func (c *InboxCmd) Run() error {
	db, err := slkdb.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	userMap, err := loadUsers(db)
	if err != nil {
		return err
	}

	query := `
		SELECT m.ts, m.channel_id, c.name, m.user_id, m.text, m.reply_count, m.status
		FROM messages m
		JOIN channels c ON c.id = m.channel_id
		WHERE 1=1`
	var params []any

	if !c.All {
		query += " AND m.status = 'unread'"
	}
	if c.Channel != "" {
		query += " AND c.name = ?"
		params = append(params, c.Channel)
	}
	if c.MinReplies > 0 {
		query += " AND m.reply_count >= ?"
		params = append(params, c.MinReplies)
	}
	query += " ORDER BY m.ts ASC"

	rows, err := db.Query(query, params...)
	if err != nil {
		return err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.TS, &m.ChannelID, &m.ChannelName, &m.UserID, &m.Text, &m.ReplyCount, &m.Status); err != nil {
			return err
		}
		m.Author = resolveUser(m.UserID, userMap)
		m.Text = cleanMarkup(m.Text, userMap)
		m.Time = formatTS(m.TS)
		m.SlackURL = slackURL(m.ChannelID, m.TS)
		msgs = append(msgs, m)
	}

	if c.JSON {
		enc := json.NewEncoder(out())
		enc.SetIndent("", "  ")
		return enc.Encode(msgs)
	}

	if len(msgs) == 0 {
		fmt.Println("inbox empty")
		return nil
	}

	for _, m := range msgs {
		status := ""
		if m.Status == "pinned" {
			status = " [pinned]"
		}
		replies := ""
		if m.ReplyCount > 0 {
			replies = fmt.Sprintf(" [%d replies]", m.ReplyCount)
		}
		text := firstLine(m.Text, 80)
		fmt.Printf("%s  %-16s  #%-20s  %-20s  %s%s%s\n",
			m.TS, m.Time, m.ChannelName, m.Author, text, replies, status)
	}
	return nil
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

var (
	mentionRE   = regexp.MustCompile(`<@(U[A-Z0-9]+)>`)
	linkRE      = regexp.MustCompile(`<([^|>]+)(?:\|([^>]*))?>>?`)
	channelRE   = regexp.MustCompile(`<#[A-Z0-9]+\|([^>]+)>`)
	broadcastRE = regexp.MustCompile(`<!(\w+)>`)
)

// cleanMarkup converts Slack mrkdwn to plain text, then resolves user mentions.
func cleanMarkup(text string, userMap map[string]string) string {
	// <!here>, <!channel>, <!everyone> → @here etc.
	text = broadcastRE.ReplaceAllString(text, "@$1")

	// <#C123|channel-name> → #channel-name
	text = channelRE.ReplaceAllString(text, "#$1")

	// <@UID> → @Name
	text = mentionRE.ReplaceAllStringFunc(text, func(s string) string {
		uid := mentionRE.FindStringSubmatch(s)[1]
		if name, ok := userMap[uid]; ok {
			return "@" + name
		}
		return s
	})

	// <URL|display> → [display](URL)    <URL> → URL
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

// replaceMentions is kept for callers that only need mention resolution.
func replaceMentions(text string, userMap map[string]string) string {
	return mentionRE.ReplaceAllStringFunc(text, func(s string) string {
		uid := mentionRE.FindStringSubmatch(s)[1]
		if name, ok := userMap[uid]; ok {
			return "@" + name
		}
		return s
	})
}

func formatTS(ts string) string {
	f, err := strconv.ParseFloat(ts, 64)
	if err != nil {
		return ts
	}
	return time.Unix(int64(f), 0).Format("2006-01-02 15:04")
}

func firstLine(s string, max int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
