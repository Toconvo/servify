package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"servify/apps/server/internal/config"
)

func TestPortalConfigHandler_Get_WithDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Portal: config.PortalConfig{
			BrandName: "",
		},
	}
	handler := NewPortalConfigHandler(cfg)

	router := gin.New()
	router.GET("/portal/config", handler.Get)

	req := httptest.NewRequest("GET", "/portal/config", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check default values
	assert.Contains(t, w.Body.String(), "Servify")
	assert.Contains(t, w.Body.String(), "#4299e1")
	assert.Contains(t, w.Body.String(), "zh-CN")
}

func TestPortalConfigHandler_Get_WithCustomConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Portal: config.PortalConfig{
			BrandName:      "Test Brand",
			LogoURL:        "https://example.com/logo.png",
			PrimaryColor:   "#FF0000",
			SecondaryColor: "#00FF00",
			DefaultLocale:  "en-US",
			Locales:        []string{"en-US", "fr-FR"},
			SupportEmail:   "support@example.com",
		},
	}
	handler := NewPortalConfigHandler(cfg)

	router := gin.New()
	router.GET("/portal/config", handler.Get)

	req := httptest.NewRequest("GET", "/portal/config", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check custom values
	assert.Contains(t, w.Body.String(), "Test Brand")
	assert.Contains(t, w.Body.String(), "https://example.com/logo.png")
	assert.Contains(t, w.Body.String(), "#FF0000")
	assert.Contains(t, w.Body.String(), "#00FF00")
	assert.Contains(t, w.Body.String(), "en-US")
	assert.Contains(t, w.Body.String(), "fr-FR")
	assert.Contains(t, w.Body.String(), "support@example.com")
}

func TestPortalConfigHandler_Get_WithNilConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewPortalConfigHandler(nil)

	router := gin.New()
	router.GET("/portal/config", handler.Get)

	req := httptest.NewRequest("GET", "/portal/config", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check defaults are applied
	assert.Contains(t, w.Body.String(), "Servify")
}

func TestPortalConfigHandler_Get_WithPartialConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Portal: config.PortalConfig{
			BrandName: "Partial Brand",
			// Leave other fields empty to test defaults
		},
	}
	handler := NewPortalConfigHandler(cfg)

	router := gin.New()
	router.GET("/portal/config", handler.Get)

	req := httptest.NewRequest("GET", "/portal/config", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Custom brand name
	assert.Contains(t, w.Body.String(), "Partial Brand")
	// Defaults for other fields
	assert.Contains(t, w.Body.String(), "#4299e1")
	assert.Contains(t, w.Body.String(), "zh-CN")
}

func TestNewPortalConfigHandler(t *testing.T) {
	cfg := &config.Config{
		Portal: config.PortalConfig{
			BrandName: "Test",
		},
	}
	handler := NewPortalConfigHandler(cfg)

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.cfg)
}

func TestPortalConfigResponse_Structure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Portal: config.PortalConfig{
			BrandName:      "Test Brand",
			LogoURL:        "logo.png",
			PrimaryColor:   "#123456",
			SecondaryColor: "#654321",
			DefaultLocale:  "de-DE",
			Locales:        []string{"de-DE", "en-GB"},
			SupportEmail:   "test@test.com",
		},
	}
	handler := NewPortalConfigHandler(cfg)

	router := gin.New()
	router.GET("/portal/config", handler.Get)

	req := httptest.NewRequest("GET", "/portal/config", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "brand_name")
	assert.Contains(t, w.Body.String(), "primary_color")
	assert.Contains(t, w.Body.String(), "secondary_color")
	assert.Contains(t, w.Body.String(), "default_locale")
	assert.Contains(t, w.Body.String(), "locales")
}
