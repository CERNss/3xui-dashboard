package channels

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cern/3xui-dashboard/internal/service/notify"
)

// Discord delivers via a server webhook URL. The webhook is per-
// channel + tied to a single Discord guild → the URL alone is the
// auth, so leaking it is the operator's risk. Treat as a secret.
type Discord struct {
	webhookURL string
	http       *http.Client
}

// NewDiscord builds the channel. Empty webhookURL → Enabled()=false.
func NewDiscord(webhookURL string) *Discord {
	return &Discord{
		webhookURL: webhookURL,
		http:       &http.Client{Timeout: 10 * time.Second},
	}
}

func (d *Discord) Name() string  { return "discord" }
func (d *Discord) Enabled() bool { return d.webhookURL != "" }

// discordWebhook is the subset of the embed shape we use.
type discordWebhook struct {
	Content string         `json:"content,omitempty"`
	Embeds  []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title       string              `json:"title"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color"`
	Fields      []discordEmbedField `json:"fields,omitempty"`
	URL         string              `json:"url,omitempty"`
	Footer      *discordEmbedFooter `json:"footer,omitempty"`
}

type discordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type discordEmbedFooter struct {
	Text string `json:"text"`
}

func (d *Discord) Send(ctx context.Context, msg notify.Message) error {
	if !d.Enabled() {
		return nil
	}
	embed := discordEmbed{
		Title:       msg.Title,
		Description: msg.Body,
		Color:       msg.Level.DiscordColor(),
		URL:         msg.URL,
	}
	for _, f := range msg.Fields {
		embed.Fields = append(embed.Fields, discordEmbedField{
			Name:   f.Key,
			Value:  f.Value,
			Inline: len(f.Value) < 40, // short values render side-by-side
		})
	}
	if msg.EventType != "" {
		embed.Footer = &discordEmbedFooter{Text: msg.EventType}
	}

	status, body, err := PostJSON(ctx, d.http, d.webhookURL, discordWebhook{
		Embeds: []discordEmbed{embed},
	}, PostJSONOptions{})
	if err != nil {
		return fmt.Errorf("discord: %w", err)
	}
	// Discord returns 204 No Content on success — PostJSON's
	// 2xx-pass check covers that. 200 also valid (with a body
	// echoing the message). Anything else has already been
	// surfaced by PostJSON as an error.
	if status != http.StatusNoContent && status != http.StatusOK {
		return fmt.Errorf("discord: unexpected status %d: %s", status, body)
	}
	return nil
}
