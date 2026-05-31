# slk bugs / feature requests

---

### `slk search` — full-text search via FTS5

SQLite supports FTS5 natively. Schema change + one virtual table index on `messages.text`. Would enable `slk search "deploy failed"` without any API calls. Low cost, high value.

---

### goreleaser — pre-built binaries on GitHub Releases

Add `.goreleaser.yaml` + GitHub Actions workflow to publish cross-platform binaries on each release. Makes `slk`/`slktui` installable without Go. Two binaries: `cmd/slk` and `cmd/slktui`. ~30 lines of config.

---

### `inbox` output too large for direct consumption

**Problem:** `slk inbox --json` with 333+ unread messages outputs ~200KB+ of JSON. Piping directly to Claude or reading in context hits limits.

**Note:** Likely a one-time backlog problem. With `--channel` and `--min-replies` for bulk triage, steady-state inbox should be manageable. `--limit` without paging just hides messages rather than solving anything — deprioritized unless the problem recurs.

**Suggestions:**
- `slk inbox --summary` — condensed JSON: ts + channel + author + reply_count + truncated text, no full body

---

### No way to group/sort inbox by channel or thread activity

**Problem:** Inbox is flat. With 333 unread in one channel, hard to triage by theme.

**Suggestions:**
- `slk inbox --by-channel` — group output by channel with a header per channel
- `slk inbox --sort replies` — sort by reply count descending (surfaces active threads)
- `slk inbox --channel-summary` — unread counts per channel without listing every message (similar to `slk channels` but scoped to inbox)

---

### `inbox` JSON includes full message text but no thread preview

**Problem:** To understand a thread, you need a separate `slk show <ts>` call. Full message body in inbox JSON is often not enough context — thread replies are the real signal.

**Suggestions:**
- Include `first_reply_preview` (first reply text, truncated) for messages with replies
- `--with-threads` flag to inline thread replies directly into inbox JSON

---

### No way to filter inbox by message type or author

**Problem:** Inbox mixes link shares, active discussions, announcements, and noise.

**Suggestions:**
- `slk inbox --author <name>` — filter to specific person
- `slk inbox --has-link` — messages containing URLs

---

### `slk read --before <ts>`

Mark everything before a timestamp as read. Useful for bulk-clearing old noise without touching recent messages.

---

---

### `slktui` capture action

Save current thread to notebook vault as a markdown file. Key `c` in card view. Needs `LIBRARY_PATH` env var (same as old slack-vac-tui). File should include author, timestamp, channel, full text, and replies.

---

### `slktui` defer action

~~Skip a thread for now without marking it read~~ — implemented as `n`/`p` in card view for next/prev navigation with no status change. Remaining question: should `d` explicitly bump a thread to the end of the list so it resurfaces later?

---

## Done

- **Slack deep links** — `slack_url` in `slk show` and `slk inbox --json`. `slk open` uses shared helper.
- **`--min-replies N`** — inbox filter to surface active threads, skip noise.
- **Bulk `read`** — `slk read <ts> <ts> ...` accepts multiple timestamps.
- **`read/unread --channel <name>`** — mark entire channel read or unread, pinned messages never touched.
