package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cern/3xui-dashboard/internal/service/notify"
)

// Feishu delivers via a custom-bot webhook. Lark is the international
// brand of the same product and uses the same wire format.
type Feishu struct {
	webhookURL string
	http       *http.Client
}

// NewFeishu builds the channel. Empty webhookURL → Enabled()=false.
func NewFeishu(webhookURL string) *Feishu {
	return &Feishu{
		webhookURL: webhookURL,
		http:       &http.Client{Timeout: 10 * time.Second},
	}
}

func (f *Feishu) Name() string  { return "feishu" }
func (f *Feishu) Enabled() bool { return f.webhookURL != "" }

// feishuMessage uses the interactive card shape so we get level-
// colored headers + structured fields. The text-only variant
// (msg_type=text) is simpler but renders everything in a single
// paragraph with no visual hierarchy.
type feishuMessage struct {
	MsgType string      `json:"msg_type"`
	Card    feishuCard  `json:"card"`
}

type feishuCard struct {
	Config   feishuConfig    `json:"config"`
	Header   feishuHeader    `json:"header"`
	Elements []feishuElement `json:"elements"`
}

type feishuConfig struct {
	WideScreenMode bool `json:"wide_screen_mode"`
}

type feishuHeader struct {
	Title    feishuText `json:"title"`
	Template string     `json:"template"`
}

type feishuText struct {
	Tag     string `json:"tag"`
	Content string `json:"content"`
}

// feishuElement is one of: div (text), hr, action (buttons).
// We only build div + hr + action here so the type is loose;
// json.Marshal omits the zero fields.
type feishuElement struct {
	Tag     string         `json:"tag"`
	Text    *feishuText    `json:"text,omitempty"`
	Fields  []feishuField  `json:"fields,omitempty"`
	Actions []feishuAction `json:"actions,omitempty"`
}

type feishuField struct {
	IsShort bool       `json:"is_short"`
	Text    feishuText `json:"text"`
}

type feishuAction struct {
	Tag   string     `json:"tag"`
	Text  feishuText `json:"text"`
	Type  string     `json:"type"`
	URL   string     `json:"url"`
}

// feishuResponse is the standard envelope. Success → code=0.
type feishuResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (f *Feishu) Send(ctx context.Context, msg notify.Message) error {
	if !f.Enabled() {
		return nil
	}
	card := feishuCard{
		Config: feishuConfig{WideScreenMode: true},
		Header: feishuHeader{
			Title:    feishuText{Tag: "plain_text", Content: msg.Title},
			Template: msg.Level.FeishuTemplate(),
		},
	}
	if msg.Body != "" {
		card.Elements = append(card.Elements, feishuElement{
			Tag:  "div",
			Text: &feishuText{Tag: "lark_md", Content: msg.Body},
		})
	}
	if len(msg.Fields) > 0 {
		card.Elements = append(card.Elements, feishuElement{Tag: "hr"})
		fields := make([]feishuField, 0, len(msg.Fields))
		for _, fld := range msg.Fields {
			fields = append(fields, feishuField{
				IsShort: len(fld.Value) < 40,
				Text:    feishuText{Tag: "lark_md", Content: fmt.Sprintf("**%s**\n%s", fld.Key, fld.Value)},
			})
		}
		card.Elements = append(card.Elements, feishuElement{
			Tag:    "div",
			Fields: fields,
		})
	}
	if msg.URL != "" {
		card.Elements = append(card.Elements, feishuElement{
			Tag: "action",
			Actions: []feishuAction{{
				Tag:  "button",
				Text: feishuText{Tag: "plain_text", Content: "查看详情"},
				Type: "primary",
				URL:  msg.URL,
			}},
		})
	}

	_, respBody, err := PostJSON(ctx, f.http, f.webhookURL, feishuMessage{
		MsgType: "interactive",
		Card:    card,
	}, PostJSONOptions{})
	if err != nil {
		return fmt.Errorf("feishu: %w", err)
	}
	var env feishuResponse
	if jerr := json.Unmarshal(respBody, &env); jerr == nil && env.Code != 0 {
		return fmt.Errorf("feishu api %d: %s", env.Code, env.Msg)
	}
	return nil
}

