package model

import "time"

// Webhook is an outbound event-subscription target configured by an
// admin. Events is a JSONB array of event-name patterns, e.g.
// ["node.*", "order.completed"]. AllowPrivate must be set to true to
// permit URLs targeting private/RFC1918 hosts (otherwise the SSRF
// guard in the delivery transport rejects them).
type Webhook struct {
	ID           int64       `gorm:"primaryKey"                                  json:"id"`
	Name         string      `gorm:"column:name;not null"                        json:"name"`
	URL          string      `gorm:"column:url;not null"                         json:"url"`
	Secret       string      `gorm:"column:secret;not null"                      json:"-"`
	Events       StringSlice `gorm:"column:events;type:jsonb;not null;default:'[]'::jsonb" json:"events"`
	Enabled      bool        `gorm:"column:enabled;not null"                     json:"enabled"`
	AllowPrivate bool        `gorm:"column:allow_private;not null;default:false" json:"allow_private"`

	// Method is the HTTP verb (GET / POST / PUT / DELETE / PATCH).
	// Default POST reproduces pre-template behavior. GET is useful
	// for status-page probes; body_template is sent as a query
	// string in that case.
	Method string `gorm:"column:method;not null;default:'POST'" json:"method"`

	// Headers is admin-defined custom request headers, merged on
	// top of the dashboard's signature/auth headers. Useful for
	// per-target API tokens (e.g. "Authorization: Bearer xxx").
	Headers StringMap `gorm:"column:headers;type:jsonb;not null;default:'{}'::jsonb" json:"headers"`

	// BodyTemplate is a Go text/template rendered with the event
	// Envelope as context. Empty string falls back to the standard
	// JSON Envelope serialization. Admins use this to fit the
	// receiver's expected schema (Slack blocks, n8n webhook
	// adaptors, etc.).
	//
	// Available template variables: .Version, .Event, .Timestamp,
	// .Data (a map[string]any decoded from the event payload, so
	// fields like {{.Data.user_id}} work).
	BodyTemplate string `gorm:"column:body_template;not null;default:''" json:"body_template"`

	// TemplateFormat tells the delivery layer what Content-Type to
	// send. Recognized values:
	//   "json" (default) — Content-Type: application/json; rendered
	//                       template should be a JSON document
	//   "form"            — Content-Type: application/x-www-form-urlencoded
	//   "text"            — Content-Type: text/plain
	//   "raw"             — no Content-Type forced; admin sets it
	//                        via Headers["Content-Type"]
	TemplateFormat string `gorm:"column:template_format;not null;default:'json'" json:"template_format"`

	CreatedAt time.Time `gorm:"column:created_at;not null;default:now()"  json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;default:now()"  json:"updated_at"`
}

func (Webhook) TableName() string { return "webhooks" }
