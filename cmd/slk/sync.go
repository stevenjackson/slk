package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	slkdb "github.com/stevejackson/slk/internal/db"
	slkingest "github.com/stevejackson/slk/internal/slack"
)

func runSync(args []string) error {
	days := 7
	for i, a := range args {
		if a == "--days" && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("--days: %w", err)
			}
			days = n
		}
	}

	token := os.Getenv("SLACK_USER_TOKEN")
	if token == "" {
		return fmt.Errorf("SLACK_USER_TOKEN not set")
	}

	db, err := slkdb.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	ingester := slkingest.NewIngester(token, db)

	if err := ingester.SyncUsers(false); err != nil {
		return err
	}

	rows, err := db.Query("SELECT id, name, last_synced_ts FROM channels")
	if err != nil {
		return err
	}
	defer rows.Close()

	type channel struct {
		id, name    string
		lastSynced  *string
	}
	var channels []channel
	for rows.Next() {
		var ch channel
		if err := rows.Scan(&ch.id, &ch.name, &ch.lastSynced); err != nil {
			return err
		}
		channels = append(channels, ch)
	}
	rows.Close()

	if len(channels) == 0 {
		return fmt.Errorf("no channels tracked — run: slk channels add")
	}

	oldest := fmt.Sprintf("%.6f", float64(time.Now().Unix()-int64(days)*86400))

	for _, ch := range channels {
		o := oldest
		// If we've synced before and it's more recent than the window, start there
		if ch.lastSynced != nil && *ch.lastSynced > o {
			o = *ch.lastSynced
		}
		if err := ingester.SyncChannel(ch.id, ch.name, o); err != nil {
			fmt.Fprintf(os.Stderr, "warn: #%s: %v\n", ch.name, err)
		}
	}
	return nil
}
