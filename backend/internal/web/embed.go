package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// dist holds the exported Expo web build (index.html + _expo/… + assets/…).
// The `all:` prefix is required so dot/underscore files (expo emits `_expo/…`)
// are included. A committed stub index.html keeps this package compiling when
// no real export is present (local builds, lint/test CI); the Docker build
// overwrites the directory with the real export before `go build`.
//
//go:embed all:dist
var dist embed.FS

// RegisterSPA serves the embedded expo-web build and falls back to index.html
// for client-side routes. It MUST be called AFTER all /api and /health routes
// are registered so it only catches unmatched paths (gin NoRoute). It returns
// an error if the embed is misconfigured so main can fail loudly at boot
// rather than silently 404.
func RegisterSPA(r *gin.Engine) error {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return err
	}
	fileServer := http.FileServer(http.FS(sub))
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		return err // embed misconfigured — fail loudly, never silently 404
	}

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		// NEVER shadow the API or health surface. Unmatched /api/... (e.g. a
		// typo'd endpoint) must 404 as JSON, not return the SPA shell.
		if strings.HasPrefix(p, "/api/") || p == "/health" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		// Serve a real asset if it exists; otherwise hand back index.html so
		// deep links like /groups/123 / /invite?token=… boot the SPA.
		if _, statErr := fs.Stat(sub, strings.TrimPrefix(p, "/")); statErr == nil && p != "/" {
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})
	return nil
}
