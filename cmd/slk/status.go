package main

import (
	"fmt"

	slkdb "github.com/stevejackson/slk/internal/db"
)

func runSetStatus(args []string, status string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: slk %s <ts> [<ts>...] [--channel <name>]", status)
	}

	db, err := slkdb.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	// scan args for --channel anywhere in the list
	var channel string
	var timestamps []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--channel" {
			if i+1 >= len(args) {
				return fmt.Errorf("--channel requires a channel name")
			}
			channel = args[i+1]
			i++ // skip next arg
		} else {
			timestamps = append(timestamps, args[i])
		}
	}

	if channel != "" {
		// only transition messages that aren't already in the target status
		// never touch pinned messages
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
