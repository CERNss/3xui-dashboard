package notify

import (
	"context"
	"fmt"
)

// Channel is the contract every notification destination (email,
// telegram, discord, feishu, …) implements. The notify service
// fans events out to a routed list of channels — each channel
// renders the same Message into its own native wire shape.
type Channel interface {
	// Name returns the wire identifier ("email", "telegram", …).
	// Used by the Router for matching and the dedup log for the
	// channel-specific kind suffix.
	Name() string

	// Enabled reports whether the channel is fully configured. A
	// channel whose required env vars are empty SHALL return false
	// and the dispatch loop SHALL skip it without error.
	Enabled() bool

	// Send delivers the message. Per-channel retry is the channel's
	// responsibility (Channel.Send may block while retrying).
	// Returning an error logs at warn level; other channels still
	// run.
	Send(ctx context.Context, msg Message) error
}

// Level controls severity-derived styling: emoji prefix for
// Telegram, embed color for Discord, header template for Feishu.
type Level int

const (
	LevelInfo Level = iota
	LevelWarn
	LevelError
)

// String returns the lowercase token used in log fields + channel
// rendering helpers.
func (l Level) String() string {
	switch l {
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return fmt.Sprintf("level(%d)", int(l))
	}
}

// Emoji returns a single-character indicator for the level. Used by
// Telegram (HTML doesn't support inline colored text without
// shenanigans, so emoji are the universal "severity at a glance").
func (l Level) Emoji() string {
	switch l {
	case LevelError:
		return "🔴"
	case LevelWarn:
		return "🟡"
	default:
		return "🟢"
	}
}

// DiscordColor returns the RGB int Discord uses for embed colors.
// Picked to match common dashboard conventions: green for info,
// amber for warn, red for error.
func (l Level) DiscordColor() int {
	switch l {
	case LevelError:
		return 0xE74C3C // red
	case LevelWarn:
		return 0xF1C40F // amber
	default:
		return 0x2ECC71 // green
	}
}

// FeishuTemplate returns the card-header color name Feishu / Lark
// expects in interactive messages. Per their docs: blue, wathet,
// turquoise, green, yellow, orange, red, carmine, violet, purple,
// indigo, grey. We use a 3-level subset.
func (l Level) FeishuTemplate() string {
	switch l {
	case LevelError:
		return "red"
	case LevelWarn:
		return "yellow"
	default:
		return "green"
	}
}

// Field is one structured key/value pair attached to a Message. Each
// channel renders Fields differently (Telegram: <code> block; Discord:
// embed.fields; Feishu: 2-col card row; email: appended as plain text
// "key: value\n" lines).
type Field struct {
	Key   string
	Value string
}

// Message is the channel-agnostic payload the notify service builds
// from an event. Channels translate this into their wire format.
type Message struct {
	// Level controls severity styling per channel.
	Level Level
	// Title is the short subject line. <80 chars recommended — some
	// channels use it as the embed title, email subject, etc.
	Title string
	// Body is the longer plain-text body. Channels apply their own
	// escaping; do NOT pre-escape HTML / markdown.
	Body string
	// Fields render as a structured table where supported.
	Fields []Field
	// URL is an optional "view in panel" link surfaced as a click
	// target on channels that have first-class link support.
	URL string
	// EventType is the bus type that triggered this message. Used
	// by the dedup log key + structured logs. Not user-visible.
	EventType string
	// OwnershipID is the dedup boundary when the event is per-client.
	// 0 means "no per-ownership dedup" (e.g. node.offline alerts —
	// we dedup on node ID instead via DedupKey).
	OwnershipID int64
	// DedupKey is the override for the dedup log key when OwnershipID
	// isn't the right boundary. Empty → use OwnershipID.
	DedupKey string
	// Recipient is a channel-specific routing hint, used by per-user
	// channels (email) where the dispatch loop needs to tell the
	// channel WHO to send to. Webhook-style channels (telegram,
	// discord, feishu) ignore this field — their target is baked
	// into the channel config. Empty means "use the channel's
	// configured default target" (e.g. ops alerts go to
	// MAIL_OPS_ADDRESS).
	Recipient string
}
