package handlers

import (
	"github.com/gin-gonic/gin"
	service "github.com/mediago-dev/mediago-drama/services/server/internal/service/settings"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnifiedModelsHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := Settings{service: service.NewSettings(nil)}
	router := gin.New()
	router.GET("/models", handler.HandleUnifiedModels)
	router.PUT("/models", handler.HandlePutUnifiedModel)
	for _, tc := range []struct {
		method, body string
		status       int
	}{
		{"GET", "", 200}, {"PUT", "invalid-json", 400}, {"PUT", `{"id":"custom-image","protocol":"unknown","enabled":true}`, 400}, {"PUT", `{"id":"custom-image","protocol":"images","enabled":true}`, 400},
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(tc.method, "/models", strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Errorf("%s %s => %d %s", tc.method, tc.body, response.Code, response.Body.String())
		}
		if tc.method == http.MethodGet && !strings.Contains(response.Body.String(), `"models":[]`) {
			t.Fatal("missing empty catalog")
		}
	}
}
