package handler

import (
	"net/http"

	"github.com/hotspoon/railnow/platform"
)

// Handler is the Vercel Function entry point.
func Handler(w http.ResponseWriter, r *http.Request) {
	platform.VercelHandler(w, r)
}
