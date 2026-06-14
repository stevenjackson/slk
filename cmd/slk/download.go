package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/slack-go/slack"
)

type DownloadCmd struct {
	URL    string `arg:"" help:"Slack message URL"`
	Output string `short:"o" help:"output path (default: original filename in current dir)"`
}

func (c *DownloadCmd) Run() error {
	token := os.Getenv("SLACK_USER_TOKEN")
	if token == "" {
		return fmt.Errorf("SLACK_USER_TOKEN not set")
	}

	channelID, msgTS, threadTS, err := parseDownloadURL(c.URL)
	if err != nil {
		return err
	}

	slkClient := slack.New(token)
	files, err := fetchMessageFiles(slkClient, channelID, msgTS, threadTS)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found in message")
	}
	f := files[0]

	outPath := c.Output
	if outPath == "" {
		outPath = f.Name
	}

	return downloadToFile(slkClient, f.URLPrivateDownload, outPath)
}

func parseDownloadURL(raw string) (channelID, msgTS, threadTS string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid URL: %w", err)
	}
	m := slackArchiveRE.FindStringSubmatch(u.Path)
	if m == nil {
		return "", "", "", fmt.Errorf("not a Slack message URL (expected /archives/CHANNEL/pTIMESTAMP)")
	}
	channelID = m[1]
	digits := m[2]
	if len(digits) <= 6 {
		return "", "", "", fmt.Errorf("timestamp too short: %s", digits)
	}
	msgTS = digits[:len(digits)-6] + "." + digits[len(digits)-6:]
	threadTS = u.Query().Get("thread_ts")
	return
}

func fetchMessageFiles(client *slack.Client, channelID, msgTS, threadTS string) ([]slack.File, error) {
	if threadTS != "" {
		// Thread reply — scan the thread for the specific message
		params := &slack.GetConversationRepliesParameters{
			ChannelID: channelID,
			Timestamp: threadTS,
			Limit:     1000,
		}
		for {
			msgs, hasMore, cursor, err := client.GetConversationReplies(params)
			if err != nil {
				return nil, fmt.Errorf("conversations.replies: %w", err)
			}
			for _, m := range msgs {
				if m.Timestamp == msgTS {
					return m.Files, nil
				}
			}
			if !hasMore {
				break
			}
			params.Cursor = cursor
		}
		return nil, fmt.Errorf("message %s not found in thread", msgTS)
	}

	// Channel message
	params := &slack.GetConversationHistoryParameters{
		ChannelID: channelID,
		Latest:    msgTS,
		Oldest:    msgTS,
		Inclusive: true,
		Limit:     1,
	}
	resp, err := client.GetConversationHistory(params)
	if err != nil {
		return nil, fmt.Errorf("conversations.history: %w", err)
	}
	if len(resp.Messages) == 0 {
		return nil, fmt.Errorf("message not found: %s in %s", msgTS, channelID)
	}
	return resp.Messages[0].Files, nil
}

func downloadToFile(client *slack.Client, downloadURL, dest string) error {
	if dir := filepath.Dir(dest); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := client.GetFile(downloadURL, f); err != nil {
		os.Remove(dest)
		return fmt.Errorf("downloading: %w", err)
	}

	fmt.Fprintln(os.Stderr, "downloaded:", dest)
	fmt.Println(dest)
	return nil
}
