package handlers

import (
	"net/http"
	"strings"

	"servify/apps/server/internal/config"

	"github.com/gin-gonic/gin"
)

type PortalConfigHandler struct {
	cfg *config.Config
}

func NewPortalConfigHandler(cfg *config.Config) *PortalConfigHandler {
	return &PortalConfigHandler{cfg: cfg}
}

type PortalConfigResponse struct {
	BrandName      string   `json:"brand_name"`
	LogoURL        string   `json:"logo_url,omitempty"`
	PrimaryColor   string   `json:"primary_color,omitempty"`
	SecondaryColor string   `json:"secondary_color,omitempty"`
	DefaultLocale  string   `json:"default_locale"`
	Locales        []string `json:"locales"`
	SupportEmail   string   `json:"support_email,omitempty"`
}

func (h *PortalConfigHandler) Get(c *gin.Context) {
	var p config.PortalConfig
	if h.cfg != nil {
		p = h.cfg.Portal
	}
	if strings.TrimSpace(p.BrandName) == "" {
		p.BrandName = "Servify"
	}
	if strings.TrimSpace(p.PrimaryColor) == "" {
		p.PrimaryColor = "#4299e1"
	}
	if strings.TrimSpace(p.SecondaryColor) == "" {
		p.SecondaryColor = "#764ba2"
	}
	if strings.TrimSpace(p.DefaultLocale) == "" {
		p.DefaultLocale = "zh-CN"
	}
	if len(p.Locales) == 0 {
		p.Locales = []string{"zh-CN", "en-US"}
	}
	c.JSON(http.StatusOK, PortalConfigResponse{
		BrandName:      p.BrandName,
		LogoURL:        p.LogoURL,
		PrimaryColor:   p.PrimaryColor,
		SecondaryColor: p.SecondaryColor,
		DefaultLocale:  p.DefaultLocale,
		Locales:        p.Locales,
		SupportEmail:   p.SupportEmail,
	})
}
