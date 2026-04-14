package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load() // ignore error — env vars may already be set

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "sync":
		err = runSync(os.Args[2:])
	case "inbox":
		err = runInbox(os.Args[2:])
	case "show":
		err = runShow(os.Args[2:])
	case "read":
		err = runSetStatus(os.Args[2:], "read")
	case "unread":
		err = runSetStatus(os.Args[2:], "unread")
	case "pin":
		err = runSetStatus(os.Args[2:], "pinned")
	case "unpin":
		err = runSetStatus(os.Args[2:], "read")
	case "channels":
		err = runChannels(os.Args[2:])
	case "open":
		err = runOpen(os.Args[2:])
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "slk: unknown command %q\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "slk: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`slk — Slack message browser for the terminal

Usage:
  slk sync [--days N]               pull messages into local DB (default: 7 days)
  slk inbox [--channel name] [--min-replies N] [--json] [--all]
                                    show unread messages
  slk show <ts> [--json]            show message + thread
  slk read <ts> [<ts>...] [--channel name]   mark messages as read
  slk unread <ts> [<ts>...] [--channel name] mark messages as unread
  slk pin <ts> [<ts>...]                     pin messages (never archived)
  slk unpin <ts> [<ts>...]                   unpin messages (back to read)
  slk open <ts>                     open message in Slack (browser)
  slk channels                      list tracked channels
  slk channels add [--private]      add a channel to track (--private requires groups:read scope)
  slk channels rm <name>            stop tracking a channel

Environment:
  SLACK_USER_TOKEN    required (xoxp-...)
  SLK_DB              path to sqlite DB (default: ~/.slk/slk.db)
`)
}
