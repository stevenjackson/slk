# slk

Two tools for managing Slack without the noise:

- **`slk`** — CLI for syncing, reading, and piping Slack messages
- **`slktui`** — Terminal UI for triaging your inbox one thread at a time

```
slk inbox | head -20
slk show 1712345678.123456 --json | claude "summarize this thread"
slktui   # keyboard-driven inbox triage
```

Both tools share the same SQLite database at `~/.slk/slk.db`.

## Install

**Prerequisites:** Go 1.21+

```sh
git clone https://github.com/stevejackson/slk
cd slk
make build        # builds both slk and slktui
make install      # copies both to /usr/local/bin
```

Or build individually:

```sh
go build -o slk ./cmd/slk
go build -o slktui ./cmd/slktui
```

Or install directly:

```sh
go install github.com/stevejackson/slk/cmd/slk@latest
go install github.com/stevejackson/slk/cmd/slktui@latest
```

## Setup

### 1. Get a Slack user token

You need a **user token** (`xoxp-...`), not a bot token. User tokens can read channels you're a member of.

Options:
- Create a Slack app at [api.slack.com/apps](https://api.slack.com/apps), add OAuth scopes, and install to your workspace
- Use an existing workspace app token if you have one

Required OAuth scopes:
| Scope | Purpose |
|-------|---------|
| `channels:history` | Read public channel messages |
| `channels:read` | List public channels |
| `groups:history` | Read private channel messages |
| `groups:read` | List private channels (optional) |
| `users:read` | Resolve user names |
| `im:history` | Read DMs (optional) |

### 2. Configure

```sh
cp .env.example .env
# edit .env and set SLACK_USER_TOKEN=xoxp-...
```

Or set the environment variable directly:

```sh
export SLACK_USER_TOKEN=xoxp-...
```

### 3. Track channels and sync

```sh
slk channels add   # search and select a channel to track
slk sync           # pull last 7 days of messages
```

## Usage

### Syncing

```sh
slk sync               # pull new messages (since last sync, up to 7 days)
slk sync --days 30     # pull last 30 days
```

Sync is incremental — subsequent runs only fetch messages newer than the last sync. Threads are fetched automatically when a message has replies.

### Reading messages

```sh
slk inbox                          # all unread, newest first
slk inbox --channel engineering    # filter by channel
slk inbox --all                    # include read and pinned messages
slk inbox --json                   # machine-readable output
```

Inbox output format:
```
1712345678.123456  #engineering          Alice Smith           Working on the deploy... [3 replies]
```

```sh
slk show 1712345678.123456             # message + full thread
slk show 1712345678.123456 --json      # full thread as JSON
```

### Managing messages

```sh
slk read <ts>      # mark as read (removes from inbox)
slk pin <ts>       # pin message (kept when archiving later)
slk unpin <ts>     # unpin (back to read)
slk open <ts>      # open in Slack (browser)
```

The `ts` value is the message timestamp shown in `slk inbox` output. It's also Slack's canonical message ID — use it with `slk open` to jump directly to the message in Slack.

### Managing channels

```sh
slk channels           # list tracked channels with unread counts
slk channels add       # interactive search to add a channel
slk channels rm name   # stop tracking (does not delete messages)
```

### Piping to other tools

```sh
# Summarize your inbox with Claude
slk inbox --json | claude "what needs my attention?"

# Get a specific thread as JSON
slk show 1712345678.123456 --json

# Count unreads per channel
slk inbox --all --json | jq 'group_by(.channel_name) | map({channel: .[0].channel_name, count: length})'

# Find messages mentioning a keyword
slk inbox --all --json | jq '.[] | select(.text | test("deploy"; "i"))'
```

## slktui

A full-screen terminal UI for working through your inbox. Run `slktui` after syncing.

**List view**
```
j / k       navigate
enter       open thread
r           mark read
q           quit
```

**Card view** — full thread text and all replies rendered as markdown
```
j / k       scroll
r           mark read, return to list
o           open in Slack
esc         back to list
q           quit
```

## Data

Messages are stored in `~/.slk/slk.db` (SQLite). Override with `SLK_DB=/path/to/db`.

Message statuses:
| Status | Meaning |
|--------|---------|
| `unread` | New from sync, appears in `slk inbox` |
| `read` | Seen, hidden from inbox by default |
| `pinned` | Flagged to keep — will be excluded from future archiving |

Messages are never deleted by sync — only status changes. If a message is updated or its thread grows, it returns to `unread`.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SLACK_USER_TOKEN` | — | Required. Your `xoxp-...` user token |
| `SLK_DB` | `~/.slk/slk.db` | Path to SQLite database |
