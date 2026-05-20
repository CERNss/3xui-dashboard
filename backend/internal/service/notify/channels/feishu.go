package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"text/template"
	"time"

	"github.com/cern/3xui-dashboard/internal/service/notify"
)

// Feishu delivers via a custom-bot webhook. Lark is the international
// brand of the same product and uses the same wire format.
//
// When `cardTemplate` is non-empty, it overrides the default
// interactive-card builder: the template is rendered with the
// notify.Message as context and the result is POSTed as the
// raw JSON body. Use this to fit a Feishu app's template_id
// requirement or to emit a simpler text card.
type Feishu struct {
	webhookURL   string
	cardTemplate *template.Template // nil → use default builder
	http         *http.Client
}

// NewFeishu builds the channel. Empty webhookURL → Enabled()=false.
// cardTemplate is optional; pass empty string to use the default
// interactive-card layout.
func NewFeishu(webhookURL, cardTemplate string) *Feishu {
	f := &Feishu{
		webhookURL: webhookURL,
		http:       &http.Client{Timeout: 10 * time.Second},
	}
	if cardTemplate != "" {
		// Parse at construction so misconfigured templates fail
		// loudly at boot instead of on first event. Template
		// errors are returned as ParseErrors but we panic here
		// because the dashboard would otherwise log the parse
		// failure on every event tick.
		tmpl, err := template.New("feishu").Option("missingkey=zero").Parse(cardTemplate)
		if err != nil {
			panic(fmt.Sprintf("feishu: FEISHU_CARD_TEMPLATE parse failed: %v", err))
		}
		f.cardTemplate = tmpl
	}
	return f
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

	// Template path: render the admin-supplied template and send
	// its bytes verbatim. The dashboard does NOT wrap the result
	// in {msg_type, card} — that's the template's responsibility,
	// since admins may want msg_type=text or a template_id form.
	//
	// Convert to a map[string]any so missing-key references in the
	// template render to empty rather than crashing the dispatch
	// loop — Go templates only honor `missingkey=zero` for map
	// access, not struct field access.
	if f.cardTemplate != nil {
		ctxMap := messageToMap(msg)
		var buf bytes.Buffer
		if err := f.cardTemplate.Execute(&buf, ctxMap); err != nil {
			return fmt.Errorf("feishu template render: %w", err)
		}
		_, respBody, err := PostJSONRaw(ctx, f.http, f.webhookURL, buf.Bytes(), PostJSONOptions{})
		if err != nil {
			return fmt.Errorf("feishu: %w", err)
		}
		var env feishuResponse
		if jerr := json.Unmarshal(respBody, &env); jerr == nil && env.Code != 0 {
			return fmt.Errorf("feishu api %d: %s", env.Code, env.Msg)
		}
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

// messageToMap converts a notify.Message to map[string]any so
// templates can reference fields without struct-vs-map mismatch.
// Custom: Level stringifies via its String() method; Fields stays
// a slice of {Key, Value} maps so {{range .Fields}} works.
func messageToMap(m notify.Message) map[string]any {
	fields := make([]map[string]any, 0, len(m.Fields))
	for _, f := range m.Fields {
		fields = append(fields, map[string]any{
			"Key":   f.Key,
			"Value": f.Value,
		})
	}
	return map[string]any{
		"Title":     m.Title,
		"Body":      m.Body,
		"Level":     m.Level.String(),
		"URL":       m.URL,
		"Recipient": m.Recipient,
		"Fields":    fields,
	}
}
