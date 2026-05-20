package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cern/3xui-dashboard/internal/service/notify"
)

// Telegram delivers notifications via the Bot API. The bot must
// already be added to the target chat; chat_id is obtained once
// via /getUpdates and pasted into env.
type Telegram struct {
	botToken string
	chatID   string
	apiBase  string
	http     *http.Client
}

// NewTelegram builds the channel. Empty botToken / chatID → the
// channel reports Enabled()=false.
func NewTelegram(botToken, chatID string) *Telegram {
	return &Telegram{
		botToken: botToken,
		chatID:   chatID,
		apiBase:  "https://api.telegram.org",
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

// SetAPIBase replaces the API host. Test-only — production never
// calls this.
func (t *Telegram) SetAPIBase(u string) { t.apiBase = u }

func (t *Telegram) Name() string  { return "telegram" }
func (t *Telegram) Enabled() bool { return t.botToken != "" && t.chatID != "" }

// telegramRequest mirrors the small slice of sendMessage params we use.
type telegramRequest struct {
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview"`
}

// telegramResponse parses Telegram's envelope. We don't need the
// `result` object — only ok + description for error surfacing.
type telegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func (t *Telegram) Send(ctx context.Context, msg notify.Message) error {
	if !t.Enabled() {
		return nil
	}
	url := fmt.Sprintf("%s/bot%s/sendMessage", t.apiBase, t.botToken)
	body := t.renderHTML(msg)

	_, respBody, err := PostJSON(ctx, t.http, url, telegramRequest{
		ChatID:                t.chatID,
		Text:                  body,
		ParseMode:             "HTML",
		DisableWebPagePreview: true,
	}, PostJSONOptions{})
	if err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	// Telegram returns 200 with `{ok:false, description:"..."}` for
	// API errors (bad chat ID, expired token, etc.). Surface those.
	var env telegramResponse
	if jerr := json.Unmarshal(respBody, &env); jerr == nil && !env.OK && env.Description != "" {
		return fmt.Errorf("telegram api: %s", env.Description)
	}
	return nil
}

// renderHTML builds the Telegram-safe HTML body. Telegram supports a
// tiny tag whitelist (<b>, <i>, <u>, <code>, <pre>, <a>) — we stick
// to <b> and <a> + a leading emoji for severity.
func (t *Telegram) renderHTML(msg notify.Message) string {
	var b strings.Builder
	b.WriteString(msg.Level.Emoji())
	b.WriteString(" <b>")
	b.WriteString(escapeHTML(msg.Title))
	b.WriteString("</b>\n\n")
	if msg.Body != "" {
		b.WriteString(escapeHTML(msg.Body))
	}
	if len(msg.Fields) > 0 {
		b.WriteString("\n\n<pre>")
		for i, f := range msg.Fields {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(escapeHTML(f.Key))
			b.WriteString(": ")
			b.WriteString(escapeHTML(f.Value))
		}
		b.WriteString("</pre>")
	}
	if msg.URL != "" {
		b.WriteString(`<a href="`)
		b.WriteString(escapeHTMLAttr(msg.URL))
		b.WriteString(`">查看详情</a>`)
	}
	return b.String()
}

// escapeHTML escapes the characters Telegram cares about in text
// content (NOT in HTML attributes — see escapeHTMLAttr).
func escapeHTML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return r.Replace(s)
}

func escapeHTMLAttr(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		`"`, "&quot;",
		"<", "&lt;",
		">", "&gt;",
	).Replace(s)
}
