package healthmon

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// CheckResponse is the JSON body returned by GET /health/{service_id}.
type CheckResponse struct {
	ServiceID string     `json:"service_id"`
	Available bool       `json:"available"`
	Status    string     `json:"status"`
	Fallback  *string    `json:"fallback"`
	Essential bool       `json:"essential"`
	Message   string     `json:"message"`
	OpenedAt  *time.Time `json:"opened_at,omitempty"`
}

// ReportRequest is the JSON body sent by POST /report/{service_id}.
type ReportRequest struct {
	Success    bool  `json:"success"`
	StatusCode int   `json:"status_code"`
	LatencyMs  int64 `json:"latency_ms"`
}

type apiHandler struct {
	mon *Monitor
}

func newAPIHandler(mon *Monitor) http.Handler {
	h := &apiHandler{mon: mon}
	mux := http.NewServeMux()
	mux.HandleFunc("/health/", h.handleCheck)
	mux.HandleFunc("/report/", h.handleReport)
	mux.HandleFunc("/status", h.handleStatus)
	mux.HandleFunc("/metrics", h.handlePrometheus)
	return mux
}

// GET /health/{service_id}
func (h *apiHandler) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/health/")
	if id == "" {
		http.Error(w, "service_id required", http.StatusBadRequest)
		return
	}

	resp := h.mon.Check(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// POST /report/{service_id}
func (h *apiHandler) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/report/")
	if id == "" {
		http.Error(w, "service_id required", http.StatusBadRequest)
		return
	}

	var req ReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	h.mon.Report(id, req.Success)
	w.WriteHeader(http.StatusNoContent)
}

// GET /status
func (h *apiHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	states := h.mon.reg.allStates()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"services": states,
	})
}

// GET /metrics  (Prometheus text format)
func (h *apiHandler) handlePrometheus(w http.ResponseWriter, r *http.Request) {
	states := h.mon.reg.allStates()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	for _, s := range states {
		open := 0
		if s.Status == "OPEN" {
			open = 1
		}
		half := 0
		if s.Status == "HALF_OPEN" {
			half = 1
		}
		w.Write([]byte("# HELP healthmon_circuit_open 1 if circuit is OPEN\n"))
		w.Write([]byte("# TYPE healthmon_circuit_open gauge\n"))
		w.Write([]byte(strings.Join([]string{
			"healthmon_circuit_open{service=\"", s.ServiceID, "\"} ", itoa(open), "\n",
			"healthmon_circuit_half_open{service=\"", s.ServiceID, "\"} ", itoa(half), "\n",
			"healthmon_error_rate{service=\"", s.ServiceID, "\"} ", fmtFloat(s.ErrorRate), "\n",
			"healthmon_total_requests{service=\"", s.ServiceID, "\"} ", itoa64(s.TotalRequests), "\n",
		}, "")))
	}
}

func serveHealthmon(addr string, mon *Monitor) {
	handler := newAPIHandler(mon)
	log.Printf("[healthmon] HTTP API listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Printf("[healthmon] server error: %v", err)
	}
}

func itoa(n int) string        { return strconv.Itoa(n) }
func itoa64(n int64) string    { return strconv.FormatInt(n, 10) }
func fmtFloat(f float64) string { return fmt.Sprintf("%.4f", f) }
