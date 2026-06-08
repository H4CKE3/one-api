package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetDashboardDays(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		query    string
		expected int
	}{
		{name: "default", query: "", expected: 7},
		{name: "seven days", query: "?days=7", expected: 7},
		{name: "thirty days", query: "?days=30", expected: 30},
		{name: "unsupported days", query: "?days=90", expected: 7},
		{name: "invalid days", query: "?days=abc", expected: 7},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/user/dashboard"+test.query, nil)

			if actual := getDashboardDays(c); actual != test.expected {
				t.Fatalf("expected %d days, got %d", test.expected, actual)
			}
		})
	}
}
