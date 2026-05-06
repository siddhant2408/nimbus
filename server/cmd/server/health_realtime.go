package main

import (
	"encoding/json"
	"net/http"

	"github.com/siddhant2408/nimbus/internal/realtime"
)

// realtimeMetricsHandler returns the HTTP handler for /health/realtime.
//
// The endpoint exposes operational counters (per-event / per-scope sends),
// that should not be reachable by anonymous public
func realtimeMetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		snapshot := realtime.M.Snapshot()
		//snapshot["daemonws"] = daemonws.M.Snapshot()
		_ = json.NewEncoder(w).Encode(snapshot)
	}
}
