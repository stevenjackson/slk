package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/slack-go/slack"
	slkdb "github.com/stevejackson/slk/internal/db"
)

type ChannelsCmd struct {
	List ChannelsListCmd `cmd:"" default:"1" help:"list tracked channels"`
	Add  ChannelsAddCmd  `cmd:"" help:"add a channel to track"`
	Rm   ChannelsRmCmd   `cmd:"" help:"stop tracking a channel"`
}

type ChannelsListCmd struct{}

func (c *ChannelsListCmd) Run() error {
	db, err := slkdb.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT c.name, c.last_synced_ts,
		       COUNT(CASE WHEN m.status='unread' THEN 1 END) as unread
		FROM channels c
		LEFT JOIN messages m ON m.channel_id = c.id
		GROUP BY c.id
		ORDER BY unread DESC, c.name ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var lastSync *string
		var unread int
		rows.Scan(&name, &lastSync, &unread)
		sync := "never"
		if lastSync != nil {
			sync = formatTS(*lastSync)
		}
		fmt.Printf("%-30s  %4d unread  synced: %s\n", "#"+name, unread, sync)
	}
	return nil
}

type ChannelsAddCmd struct {
	Private bool `help:"include private channels (requires groups:read scope)"`
}

func (c *ChannelsAddCmd) Run() error {
	token := os.Getenv("SLACK_USER_TOKEN")
	if token == "" {
		return fmt.Errorf("SLACK_USER_TOKEN not set")
	}

	client := slack.New(token)
	db, err := slkdb.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	fmt.Println("fetching channels...")

	types := []string{"public_channel"}
	if c.Private {
		types = append(types, "private_channel")
	}

	var allChannels []slack.Channel
	params := &slack.GetConversationsParameters{
		Types: types,
		Limit: 1000,
	}
	for {
		channels, cursor, err := client.GetConversations(params)
		if err != nil {
			return fmt.Errorf("conversations.list: %w", err)
		}
		allChannels = append(allChannels, channels...)
		if cursor == "" {
			break
		}
		params.Cursor = cursor
	}

	fmt.Print("filter (enter to show all): ")
	var filter string
	fmt.Scanln(&filter)
	filter = strings.ToLower(filter)

	var matches []slack.Channel
	for _, ch := range allChannels {
		if filter == "" || strings.Contains(strings.ToLower(ch.Name), filter) {
			matches = append(matches, ch)
		}
	}

	if len(matches) == 0 {
		fmt.Println("no channels match")
		return nil
	}

	for i, ch := range matches {
		fmt.Printf("%3d  #%s\n", i+1, ch.Name)
		if i >= 49 {
			fmt.Printf("  ... %d more\n", len(matches)-50)
			break
		}
	}

	fmt.Print("select number: ")
	var n int
	fmt.Scan(&n)
	if n < 1 || n > len(matches) {
		return fmt.Errorf("invalid selection")
	}
	ch := matches[n-1]

	_, err = db.Exec("INSERT OR IGNORE INTO channels(id, name) VALUES(?,?)", ch.ID, ch.Name)
	if err != nil {
		return err
	}
	fmt.Printf("tracking #%s\n", ch.Name)
	return nil
}

type ChannelsRmCmd struct {
	Name string `arg:"" help:"channel name to stop tracking"`
}

func (c *ChannelsRmCmd) Run() error {
	name := strings.TrimPrefix(c.Name, "#")
	db, err := slkdb.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	res, err := db.Exec("DELETE FROM channels WHERE name=?", name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("channel #%s not tracked", name)
	}
	fmt.Printf("stopped tracking #%s\n", name)
	return nil
}
