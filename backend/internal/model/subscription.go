package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Subscription profile / ruleset constants.
const (
	// Ruleset source types.
	RulesetSourceRemote = "remote" // fetched from URL (and optionally cached)
	RulesetSourceInline = "inline" // matcher list stored in Content

	// Ruleset behaviors — mirror Clash rule-provider behavior values.
	RulesetBehaviorDomain    = "domain"
	RulesetBehaviorIPCIDR    = "ipcidr"
	RulesetBehaviorClassical = "classical"

	// Proxy-group types.
	GroupTypeSelect      = "select"
	GroupTypeURLTest     = "url-test"
	GroupTypeFallback    = "fallback"
	GroupTypeLoadBalance = "load-balance"
	GroupTypeRelay       = "relay"

	// Profile rule kinds.
	RuleKindRuleset = "ruleset" // pull matchers from a referenced ruleset
	RuleKindInline  = "inline"  // a single inline matcher,value pair

	// Ruleset delivery modes (per profile).
	RulesetModePassthrough = "passthrough" // rule-providers point at the upstream URL
	RulesetModeSelfHosted  = "self_hosted" // rule-providers point at this dashboard's /sub/ruleset endpoint
)

// ProxyGroup is one policy group in a profile. Membership is computed
// at render time: Filter/Exclude are regexes over node names, and
// Include lists other group names or specials (DIRECT, REJECT) to fold
// in ahead of the matched nodes.
type ProxyGroup struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`               // select | url-test | fallback | load-balance | relay
	Filter   string   `json:"filter,omitempty"`   // regex over node names; empty = all nodes
	Exclude  string   `json:"exclude,omitempty"`  // regex; matched node names dropped
	Include  []string `json:"include,omitempty"`  // other group names / DIRECT / REJECT, listed first
	TestURL  string   `json:"test_url,omitempty"` // for url-test / fallback
	Interval int      `json:"interval,omitempty"` // seconds; for url-test / fallback
}

// Rule is one ordered routing entry. Either a reference to a ruleset
// (Kind="ruleset", RulesetKey set) or a single inline matcher
// (Kind="inline", Matcher+Value set). Group is the destination
// proxy-group name (or DIRECT / REJECT).
type Rule struct {
	Kind       string `json:"kind"`                  // ruleset | inline
	RulesetKey string `json:"ruleset_key,omitempty"` // when Kind=ruleset
	Matcher    string `json:"matcher,omitempty"`     // when Kind=inline: DOMAIN-SUFFIX, GEOIP, IP-CIDR, ...
	Value      string `json:"value,omitempty"`       // when Kind=inline
	Group      string `json:"group"`                 // destination group / DIRECT / REJECT
	NoResolve  bool   `json:"no_resolve,omitempty"`  // append no-resolve (IP rules)
}

// RenameRule rewrites node display names (regex Pattern -> Replace).
type RenameRule struct {
	Pattern string `json:"pattern"`
	Replace string `json:"replace"`
}

// EmojiRule prefixes an emoji onto node names whose name matches Pattern.
type EmojiRule struct {
	Pattern string `json:"pattern"`
	Emoji   string `json:"emoji"`
}

// NodeFilter keeps/drops nodes by regex over their names before grouping.
type NodeFilter struct {
	Include string `json:"include,omitempty"`
	Exclude string `json:"exclude,omitempty"`
}

// Transforms describes node-name post-processing applied before grouping.
type Transforms struct {
	Rename []RenameRule `json:"rename,omitempty"`
	Emoji  []EmojiRule  `json:"emoji,omitempty"`
	Filter NodeFilter   `json:"filter,omitempty"`
	Sort   string       `json:"sort,omitempty"` // "" | name | name-desc
}

// ---- JSONB column types --------------------------------------------------

// ProxyGroups is the proxy_groups JSONB column.
type ProxyGroups []ProxyGroup

func (s ProxyGroups) Value() (driver.Value, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]ProxyGroup(s))
}
func (s *ProxyGroups) Scan(value any) error { return jsonbScan(value, s) }

// Rules is the rules JSONB column (ordered).
type Rules []Rule

func (s Rules) Value() (driver.Value, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]Rule(s))
}
func (s *Rules) Scan(value any) error { return jsonbScan(value, s) }

// BaseOverrides maps a target name ("clash","surge",...) to a raw base
// config snippet that replaces the renderer's built-in skeleton.
type BaseOverrides map[string]string

func (m BaseOverrides) Value() (driver.Value, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(map[string]string(m))
}
func (m *BaseOverrides) Scan(value any) error { return jsonbScan(value, m) }

// Value/Scan for Transforms (a JSONB object column).
func (t Transforms) Value() (driver.Value, error) { return json.Marshal(t) }
func (t *Transforms) Scan(value any) error        { return jsonbScan(value, t) }

// ---- Rows ----------------------------------------------------------------

// SubscriptionProfile is the target-agnostic routing policy applied when
// rendering a user's aggregated nodes for a ruled target (Clash, Surge,
// Quantumult X, Loon, sing-box). Exactly one profile is the global
// default; others are reachable via ?profile=<key>.
type SubscriptionProfile struct {
	ID            int64         `gorm:"primaryKey"                                       json:"id"`
	Key           string        `gorm:"column:key;not null"                              json:"key"`
	Name          string        `gorm:"column:name;not null;default:''"                  json:"name"`
	Description   string        `gorm:"column:description;not null;default:''"           json:"description"`
	IsDefault     bool          `gorm:"column:is_default;not null;default:false"         json:"is_default"`
	Enabled       bool          `gorm:"column:enabled;not null;default:true"             json:"enabled"`
	RulesetMode   string        `gorm:"column:ruleset_mode;not null;default:'passthrough'" json:"ruleset_mode"`
	ProxyGroups   ProxyGroups   `gorm:"column:proxy_groups;type:jsonb;not null;default:'[]'::jsonb"   json:"proxy_groups"`
	Rules         Rules         `gorm:"column:rules;type:jsonb;not null;default:'[]'::jsonb"          json:"rules"`
	Transforms    Transforms    `gorm:"column:transforms;type:jsonb;not null;default:'{}'::jsonb"     json:"transforms"`
	BaseOverrides BaseOverrides `gorm:"column:base_overrides;type:jsonb;not null;default:'{}'::jsonb" json:"base_overrides"`
	CreatedAt     time.Time     `gorm:"column:created_at;not null;default:now()"         json:"created_at"`
	UpdatedAt     time.Time     `gorm:"column:updated_at;not null;default:now()"         json:"updated_at"`
}

func (SubscriptionProfile) TableName() string { return "subscription_profiles" }

// SubscriptionRuleset is a reusable rule list referenced by a profile's
// rules. Remote rulesets are fetched (and cached server-side for
// preview / expansion / non-fetching targets); inline rulesets carry
// their matchers in Content.
type SubscriptionRuleset struct {
	ID            int64      `gorm:"primaryKey"                                   json:"id"`
	Key           string     `gorm:"column:key;not null"                          json:"key"`
	Name          string     `gorm:"column:name;not null;default:''"              json:"name"`
	SourceType    string     `gorm:"column:source_type;not null;default:'remote'" json:"source_type"`
	URL           string     `gorm:"column:url;not null;default:''"               json:"url"`
	Content       string     `gorm:"column:content;not null;default:''"           json:"content"`
	Behavior      string     `gorm:"column:behavior;not null;default:'classical'" json:"behavior"`
	Format        string     `gorm:"column:format;not null;default:'yaml'"        json:"format"`
	TTLSeconds    int        `gorm:"column:ttl_seconds;not null;default:86400"    json:"ttl_seconds"`
	Enabled       bool       `gorm:"column:enabled;not null;default:true"         json:"enabled"`
	LastFetchedAt *time.Time `gorm:"column:last_fetched_at"                       json:"last_fetched_at,omitempty"`
	LastStatus    string     `gorm:"column:last_status;not null;default:''"       json:"last_status"`
	CachedContent string     `gorm:"column:cached_content;not null;default:''"    json:"-"`
	CachedETag    string     `gorm:"column:cached_etag;not null;default:''"       json:"-"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null;default:now()"     json:"created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;not null;default:now()"     json:"updated_at"`
}

func (SubscriptionRuleset) TableName() string { return "subscription_rulesets" }
