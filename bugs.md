# slk bugs / feature requests

## Observed via: AI-assisted inbox triage session

---

### `inbox` output too large for direct consumption

**Problem:** `slk inbox --json` with 333+ unread messages outputs ~200KB+ of JSON. Piping directly to Claude or reading in context hits limits — workarounds require `python3 -c` post-processing scripts.

**Suggestions:**
- `slk inbox --limit N` — cap results (e.g. top 50 by recency or reply count)
- ~~`slk inbox --min-replies N`~~ — **done**
- `slk inbox --summary` — condensed output: one line per message, ts + channel + author + reply count + truncated text (no full body). Current non-JSON format is close but still verbose at scale.

---

### No way to group/sort inbox by channel or thread activity

**Problem:** Inbox is flat. With 333 unread in one channel, it's hard to triage by theme without writing custom Python.

**Suggestions:**
- `slk inbox --by-channel` — group output by channel with a header per channel
- `slk inbox --sort replies` — sort by reply count descending (surfaces active threads)
- `slk channels` already shows unread counts — could `slk inbox --channel-summary` give that without listing every message?

---

### ~~No bulk `read` command~~ — **done**

~~**Problem:** Marking 10 messages read requires 10 separate `slk read <ts>` calls (or chaining with `&&`). Had to manually chain all timestamps.~~

Remaining suggestions:
- ~~`slk read --channel blogging`~~ — **done** (also works with `unread --channel`)
- `slk read --before <ts>` — mark everything before a timestamp as read

---

### `inbox` JSON includes full message text but no thread preview

**Problem:** To understand a thread, you need a separate `slk show <ts>` call. But the full message body is included in inbox JSON even though it's often not enough context (thread replies are the signal).

**Suggestions:**
- `slk inbox --json` could include `first_reply_preview` (first reply text, truncated) for messages with replies
- Or a `--with-threads` flag that inlines thread replies directly into inbox JSON

---

### ~~`show` and `inbox` don't output Slack deep links~~ — **done**

~~**Problem:** To get a shareable Slack URL for a message, you have to manually construct it from the channel ID and timestamp (`archives/{channel_id}/p{ts_without_dot}`). `slk show` and `slk inbox --json` don't include it.~~

`slack_url` now included in `slk show` output and `slk inbox --json`. `slk open` refactored to use shared helper.

---

### No way to filter inbox by message type

**Problem:** Inbox mixes link shares, active discussions, announcements, and noise. Separating "threads worth reading" from "passive link drops" requires reply-count heuristics.

**Suggestions:**
- `slk inbox --has-link` — messages containing URLs
- `slk inbox --min-replies N` (see above) — effectively separates discussions from noise
- `slk inbox --author <name>` — filter to specific person
