package main

import (
	"fmt"

	slkdb "github.com/stevejackson/slk/internal/db"
)

type ReadCmd struct {
	Timestamps []string `arg:"" optional:"" help:"message timestamps"`
	Channel    string   `help:"mark entire channel as read" short:"c"`
}

func (c *ReadCmd) Run() error { return setStatus(c.Timestamps, c.Channel, "read") }

type UnreadCmd struct {
	Timestamps []string `arg:"" optional:"" help:"message timestamps"`
	Channel    string   `help:"mark entire channel as unread" short:"c"`
}

func (c *UnreadCmd) Run() error { return setStatus(c.Timestamps, c.Channel, "unread") }

type PinCmd struct {
	Timestamps []string `arg:"" optional:"" help:"message timestamps"`
}

func (c *PinCmd) Run() error { return setStatus(c.Timestamps, "", "pinned") }

type UnpinCmd struct {
	Timestamps []string `arg:"" optional:"" help:"message timestamps"`
}

func (c *UnpinCmd) Run() error { return setStatus(c.Timestamps, "", "read") }

func setStatus(timestamps []string, channel, status string) error {
	if channel == "" && len(timestamps) == 0 {
		return fmt.Errorf("usage: slk %s <ts> [<ts>...] [--channel <name>]", status)
	}

	db, err := slkdb.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	if channel != "" {
		res, err := db.Exec(`
			UPDATE messages SET status=?
			WHERE status != ? AND status != 'pinned'
			AND channel_id = (SELECT id FROM channels WHERE name=?)`, status, status, channel)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		fmt.Printf("#%s → %s (%d messages)\n", channel, status, n)
		return nil
	}

	for _, ts := range timestamps {
		res, err := db.Exec("UPDATE messages SET status=? WHERE ts=?", status, ts)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			fmt.Printf("%s → not found\n", ts)
			continue
		}
		fmt.Printf("%s → %s\n", ts, status)
	}
	return nil
}
