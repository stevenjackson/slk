package main

import (
	"github.com/alecthomas/kong"
	slkconfig "github.com/stevejackson/slk/internal/config"
)

var cli struct {
	Sync     SyncCmd     `cmd:"" help:"pull messages into local DB"`
	Fetch    FetchCmd    `cmd:"" help:"fetch a specific message by Slack URL"`
	Download DownloadCmd `cmd:"" help:"download a file from a Slack message"`
	Inbox    InboxCmd    `cmd:"" help:"show unread messages"`
	Show     ShowCmd     `cmd:"" help:"show message + thread"`
	Read     ReadCmd     `cmd:"" help:"mark messages as read"`
	Unread   UnreadCmd   `cmd:"" help:"mark messages as unread"`
	Pin      PinCmd      `cmd:"" help:"pin messages (never archived)"`
	Unpin    UnpinCmd    `cmd:"" help:"unpin messages (back to read)"`
	Open     OpenCmd     `cmd:"" help:"open message in Slack (browser)"`
	Channels ChannelsCmd `cmd:"" help:"manage tracked channels"`
}

func main() {
	slkconfig.Load()
	ctx := kong.Parse(&cli,
		kong.Name("slk"),
		kong.Description("Slack message browser for the terminal.\n\nEnvironment:\n  SLACK_USER_TOKEN    required (xoxp-...)\n  SLK_DB              path to sqlite DB (default: ~/.slk/slk.db)"),
		kong.UsageOnError(),
	)
	ctx.FatalIfErrorf(ctx.Run())
}
