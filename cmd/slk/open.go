package main

import (
	"fmt"
	"os/exec"
	"runtime"

	slkdb "github.com/stevejackson/slk/internal/db"
)

func runOpen(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: slk open <ts>")
	}
	ts := args[0]

	db, err := slkdb.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	var channelID string
	err = db.QueryRow("SELECT channel_id FROM messages WHERE ts=?", ts).Scan(&channelID)
	if err != nil {
		return fmt.Errorf("message %s not found", ts)
	}

	url := fmt.Sprintf("https://slack.com/app_redirect?channel=%s&message_ts=%s", channelID, ts)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		fmt.Println(url)
		return nil
	}
	return cmd.Run()
}
