package main

import (
	"fmt"
	"os/exec"
	"runtime"

	slkdb "github.com/stevejackson/slk/internal/db"
)

type OpenCmd struct {
	Ts string `arg:"" help:"message timestamp"`
}

func (c *OpenCmd) Run() error {
	db, err := slkdb.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	var channelID string
	err = db.QueryRow("SELECT channel_id FROM messages WHERE ts=?", c.Ts).Scan(&channelID)
	if err != nil {
		return fmt.Errorf("message %s not found", c.Ts)
	}

	url := slackURL(channelID, c.Ts)

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
