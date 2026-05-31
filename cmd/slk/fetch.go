package main

import (
	"fmt"
	"net/url"
	"os"
	"regexp"

	slkdb "github.com/stevejackson/slk/internal/db"
	slkingest "github.com/stevejackson/slk/internal/slack"
)

type FetchCmd struct {
	URL string `arg:"" help:"Slack message URL (e.g. https://workspace.slack.com/archives/C.../p...)"`
}

var slackArchiveRE = regexp.MustCompile(`/archives/([A-Z0-9]+)/p(\d+)`)

func parseSlackURL(raw string) (channelID, ts string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %w", err)
	}
	m := slackArchiveRE.FindStringSubmatch(u.Path)
	if m == nil {
		return "", "", fmt.Errorf("not a Slack message URL (expected /archives/CHANNEL/pTIMESTAMP)")
	}
	channelID = m[1]

	// If thread_ts query param present, it's the parent message — use it directly.
	if threadTS := u.Query().Get("thread_ts"); threadTS != "" {
		return channelID, threadTS, nil
	}

	// Reverse p-encoding: insert dot before last 6 digits.
	digits := m[2]
	if len(digits) <= 6 {
		return "", "", fmt.Errorf("timestamp too short in URL: %s", digits)
	}
	ts = digits[:len(digits)-6] + "." + digits[len(digits)-6:]
	return channelID, ts, nil
}

func (c *FetchCmd) Run() error {
	channelID, ts, err := parseSlackURL(c.URL)
	if err != nil {
		return err
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

	if err := ingester.SyncWorkspace(false); err != nil {
		return err
	}
	if err := ingester.SyncUsers(false); err != nil {
		return err
	}
	if err := ingester.FetchMessage(channelID, ts); err != nil {
		return err
	}
	fmt.Printf("fetched %s\n", ts)
	return nil
}
