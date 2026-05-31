package admin

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
)

// SubscriptionHandler serves /api/admin/subscription/{profiles,rulesets}
// — the CRUD surface for the /sub converter's routing profiles + rule
// lists.
type SubscriptionHandler struct {
	profiles *repository.SubscriptionProfileRepo
	rulesets *repository.SubscriptionRulesetRepo
}

func NewSubscriptionHandler(profiles *repository.SubscriptionProfileRepo, rulesets *repository.SubscriptionRulesetRepo) *SubscriptionHandler {
	return &SubscriptionHandler{profiles: profiles, rulesets: rulesets}
}

func (h *SubscriptionHandler) RegisterRoutes(rg *gin.RouterGroup) {
	s := rg.Group("/subscription")
	s.GET("/profiles", h.ListProfiles)
	s.POST("/profiles", h.CreateProfile)
	s.GET("/profiles/:id", h.GetProfile)
	s.PUT("/profiles/:id", h.UpdateProfile)
	s.DELETE("/profiles/:id", h.DeleteProfile)
	s.POST("/profiles/:id/default", h.SetDefaultProfile)

	s.GET("/rulesets", h.ListRulesets)
	s.POST("/rulesets", h.CreateRuleset)
	s.PUT("/rulesets/:id", h.UpdateRuleset)
	s.DELETE("/rulesets/:id", h.DeleteRuleset)
}

// ---- profiles -------------------------------------------------------------

func (h *SubscriptionHandler) ListProfiles(c *gin.Context) {
	rows, err := h.profiles.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"profiles": rows})
}

func (h *SubscriptionHandler) GetProfile(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	p, err := h.profiles.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if p == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *SubscriptionHandler) CreateProfile(c *gin.Context) {
	var p model.SubscriptionProfile
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	normalizeProfile(&p)
	if err := validateProfile(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// New profiles are never the default; use the /default endpoint to
	// keep the single-default invariant.
	p.ID, p.IsDefault = 0, false
	if err := h.profiles.Create(c.Request.Context(), &p); err != nil {
		h.writeMutErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (h *SubscriptionHandler) UpdateProfile(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	var p model.SubscriptionProfile
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	normalizeProfile(&p)
	if err := validateProfile(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p.ID = 0 // Save targets by path id; is_default is left untouched.
	if err := h.profiles.Save(c.Request.Context(), id, &p); err != nil {
		h.writeMutErr(c, err)
		return
	}
	updated, _ := h.profiles.Get(c.Request.Context(), id)
	c.JSON(http.StatusOK, updated)
}

func (h *SubscriptionHandler) DeleteProfile(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	existing, err := h.profiles.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return
	}
	if existing.IsDefault {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete the default profile; set another default first"})
		return
	}
	if err := h.profiles.Delete(c.Request.Context(), id); err != nil {
		h.writeMutErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *SubscriptionHandler) SetDefaultProfile(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	if err := h.profiles.SetDefault(c.Request.Context(), id); err != nil {
		h.writeMutErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ---- rulesets -------------------------------------------------------------

func (h *SubscriptionHandler) ListRulesets(c *gin.Context) {
	rows, err := h.rulesets.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rulesets": rows})
}

func (h *SubscriptionHandler) CreateRuleset(c *gin.Context) {
	var rs model.SubscriptionRuleset
	if err := c.ShouldBindJSON(&rs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	normalizeRuleset(&rs)
	if err := validateRuleset(&rs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rs.ID = 0
	if err := h.rulesets.Create(c.Request.Context(), &rs); err != nil {
		h.writeMutErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, rs)
}

func (h *SubscriptionHandler) UpdateRuleset(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	var rs model.SubscriptionRuleset
	if err := c.ShouldBindJSON(&rs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	normalizeRuleset(&rs)
	if err := validateRuleset(&rs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rs.ID = 0
	if err := h.rulesets.Save(c.Request.Context(), id, &rs); err != nil {
		h.writeMutErr(c, err)
		return
	}
	updated, _ := h.rulesets.Get(c.Request.Context(), id)
	c.JSON(http.StatusOK, updated)
}

func (h *SubscriptionHandler) DeleteRuleset(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	if err := h.rulesets.Delete(c.Request.Context(), id); err != nil {
		h.writeMutErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ---- validation -----------------------------------------------------------

func normalizeProfile(p *model.SubscriptionProfile) {
	p.Key = strings.TrimSpace(p.Key)
	if p.RulesetMode == "" {
		p.RulesetMode = model.RulesetModePassthrough
	}
}

func validateProfile(p *model.SubscriptionProfile) error {
	if p.Key == "" {
		return errors.New("key is required")
	}
	switch p.RulesetMode {
	case model.RulesetModePassthrough, model.RulesetModeSelfHosted:
	default:
		return fmt.Errorf("ruleset_mode must be %q or %q", model.RulesetModePassthrough, model.RulesetModeSelfHosted)
	}
	for i, g := range p.ProxyGroups {
		if strings.TrimSpace(g.Name) == "" {
			return fmt.Errorf("proxy_groups[%d]: name is required", i)
		}
		if !validGroupType(g.Type) {
			return fmt.Errorf("proxy_groups[%d]: invalid type %q", i, g.Type)
		}
	}
	for i, r := range p.Rules {
		switch r.Kind {
		case model.RuleKindRuleset:
			if strings.TrimSpace(r.RulesetKey) == "" {
				return fmt.Errorf("rules[%d]: ruleset_key is required for a ruleset rule", i)
			}
		case model.RuleKindInline:
			if strings.TrimSpace(r.Matcher) == "" {
				return fmt.Errorf("rules[%d]: matcher is required for an inline rule", i)
			}
		default:
			return fmt.Errorf("rules[%d]: invalid kind %q", i, r.Kind)
		}
		if strings.TrimSpace(r.Group) == "" {
			return fmt.Errorf("rules[%d]: group is required", i)
		}
	}
	return nil
}

func validGroupType(t string) bool {
	switch t {
	case model.GroupTypeSelect, model.GroupTypeURLTest, model.GroupTypeFallback,
		model.GroupTypeLoadBalance, model.GroupTypeRelay:
		return true
	default:
		return false
	}
}

func normalizeRuleset(rs *model.SubscriptionRuleset) {
	rs.Key = strings.TrimSpace(rs.Key)
	rs.URL = strings.TrimSpace(rs.URL)
	if rs.SourceType == "" {
		rs.SourceType = model.RulesetSourceRemote
	}
	if rs.Behavior == "" {
		rs.Behavior = model.RulesetBehaviorClassical
	}
	if rs.TTLSeconds <= 0 {
		rs.TTLSeconds = 86400
	}
}

func validateRuleset(rs *model.SubscriptionRuleset) error {
	if rs.Key == "" {
		return errors.New("key is required")
	}
	switch rs.SourceType {
	case model.RulesetSourceRemote:
		if rs.URL == "" {
			return errors.New("url is required for a remote ruleset")
		}
	case model.RulesetSourceInline:
		if strings.TrimSpace(rs.Content) == "" {
			return errors.New("content is required for an inline ruleset")
		}
	default:
		return fmt.Errorf("source_type must be %q or %q", model.RulesetSourceRemote, model.RulesetSourceInline)
	}
	switch rs.Behavior {
	case model.RulesetBehaviorDomain, model.RulesetBehaviorIPCIDR, model.RulesetBehaviorClassical:
	default:
		return fmt.Errorf("invalid behavior %q", rs.Behavior)
	}
	return nil
}

// writeMutErr maps repo errors to status codes (duplicate key → 409,
// not-found → 404, else 500).
func (h *SubscriptionHandler) writeMutErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case strings.Contains(strings.ToLower(err.Error()), "unique"),
		strings.Contains(strings.ToLower(err.Error()), "duplicate"):
		c.JSON(http.StatusConflict, gin.H{"error": "key already exists"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
