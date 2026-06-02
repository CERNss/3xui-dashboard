package public

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/model"
)

type fakeSettings struct {
	values map[string]string
}

func (f fakeSettings) Get(_ context.Context, key string) (string, bool, error) {
	if f.values == nil {
		return "", false, nil
	}
	v, ok := f.values[key]
	return v, ok, nil
}

func TestSubscriptionPublicBaseURLPreference(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name        string
		settings    fakeSettings
		fallback    string
		reqHost     string
		forwarded   string
		expectedURL string
	}{
		{
			name: "setting wins and trims",
			settings: fakeSettings{values: map[string]string{
				model.SettingSubscriptionPublicBaseURL: "https://sub.example.com/panel/",
			}},
			fallback:    "https://env.example.com",
			reqHost:     "request.example.com",
			expectedURL: "https://sub.example.com/panel",
		},
		{
			name:        "env fallback wins over request origin",
			fallback:    "https://env.example.com/base/",
			reqHost:     "request.example.com",
			expectedURL: "https://env.example.com/base",
		},
		{
			name:        "request origin is final fallback",
			reqHost:     "request.example.com",
			forwarded:   "https",
			expectedURL: "https://request.example.com",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewSubHandler(nil, nil, "", tc.fallback, nil)
			h.settings = tc.settings
			c := requestContext(tc.reqHost, tc.forwarded)

			if got := h.subscriptionPublicBaseURL(c); got != tc.expectedURL {
				t.Errorf("subscriptionPublicBaseURL = %q, want %q", got, tc.expectedURL)
			}
		})
	}
}

func requestContext(host, forwardedProto string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "http://"+host+"/sub/id", nil)
	if forwardedProto != "" {
		req.Header.Set("X-Forwarded-Proto", forwardedProto)
	}
	c.Request = req
	return c
}
