package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"logstream/server/auth"
	"logstream/server/config"
	"logstream/server/db"
	"logstream/server/dedup"
	"logstream/server/depstate"
	pb "logstream/server/pb"
	"logstream/server/store"
)

// API holds the dependencies required to serve the REST API.
type API struct {
	cfg      *config.Config
	db       *db.DB
	store    *store.Store
	dedup    *dedup.Deduplicator
	depState *depstate.Store
}

// New creates a new API instance.
func New(cfg *config.Config, database *db.DB, st *store.Store, dd *dedup.Deduplicator, ds *depstate.Store) *API {
	return &API{cfg: cfg, db: database, store: st, dedup: dd, depState: ds}
}

// RegisterRoutes mounts all REST routes on mux.
// All routes except POST /api/auth/login are protected by JWT auth middleware.
func (a *API) RegisterRoutes(mux *http.ServeMux, jwtSecret string) {
	// Public endpoint.
	mux.HandleFunc("/api/auth/login", auth.LoginHandler(a.cfg.AdminUser, a.cfg.AdminPass, jwtSecret))

	// Protected endpoints — wrap a sub-mux with the auth middleware.
	protected := http.NewServeMux()
	protected.HandleFunc("/api/services", a.handleServices)
	protected.HandleFunc("/api/services/", a.handleServiceByID) // /api/services/{id}/config
	protected.HandleFunc("/api/logs", a.handleLogs)
	protected.HandleFunc("/api/logs/tasks", a.handleListTasks) // GET /api/logs/tasks
	protected.HandleFunc("/api/logs/", a.handleLogsByService)  // DELETE /api/logs/{service_id}
	protected.HandleFunc("/api/stats", a.handleStats)
	protected.HandleFunc("/api/healthmon", a.handleHealthmon)

	mux.Handle("/api/services", auth.Middleware(jwtSecret, protected))
	mux.Handle("/api/services/", auth.Middleware(jwtSecret, protected))
	mux.Handle("/api/logs", auth.Middleware(jwtSecret, protected))
	mux.Handle("/api/logs/tasks", auth.Middleware(jwtSecret, protected))
	mux.Handle("/api/logs/", auth.Middleware(jwtSecret, protected))
	mux.Handle("/api/stats", auth.Middleware(jwtSecret, protected))
	mux.Handle("/api/healthmon", auth.Middleware(jwtSecret, protected))
}

// ---- handlers ----

// GET /api/services
func (a *API) handleServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	services, err := a.db.ListServices()
	if err != nil {
		jsonError(w, "failed to list services", http.StatusInternalServerError)
		return
	}
	jsonOK(w, services)
}

// /api/services/{id}/config
func (a *API) handleServiceByID(w http.ResponseWriter, r *http.Request) {
	// Strip prefix "/api/services/"
	path := strings.TrimPrefix(r.URL.Path, "/api/services/")
	// Expect: {id}/config
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 || parts[1] != "config" {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	serviceID := parts[0]
	if serviceID == "" {
		jsonError(w, "service id required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		a.getServiceConfig(w, r, serviceID)
	case http.MethodPut:
		a.updateServiceConfig(w, r, serviceID)
	default:
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) getServiceConfig(w http.ResponseWriter, r *http.Request, serviceID string) {
	cfg, err := a.db.GetConfig(serviceID)
	if err != nil {
		jsonError(w, "failed to get config", http.StatusInternalServerError)
		return
	}
	jsonOK(w, cfg)
}

func (a *API) updateServiceConfig(w http.ResponseWriter, r *http.Request, serviceID string) {
	// Decode request body as a partial ServiceConfig DTO.
	var body struct {
		TtlDays   *int32  `json:"ttl_days"`
		MinLevel  *string `json:"min_level"`
		BatchSize *int32  `json:"batch_size"`
		FlushMs   *int32  `json:"flush_ms"`
		Enabled   *bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Load existing config so we do a partial update.
	existing, err := a.db.GetConfig(serviceID)
	if err != nil {
		jsonError(w, "failed to load config", http.StatusInternalServerError)
		return
	}
	existing.ServiceId = serviceID

	if body.TtlDays != nil {
		existing.TtlDays = *body.TtlDays
	}
	if body.MinLevel != nil {
		existing.MinLevel = pb.LogLevelFromString(*body.MinLevel)
	}
	if body.BatchSize != nil {
		existing.BatchSize = *body.BatchSize
	}
	if body.FlushMs != nil {
		existing.FlushMs = *body.FlushMs
	}
	if body.Enabled != nil {
		existing.Enabled = *body.Enabled
	}

	if err := a.db.UpsertConfig(existing); err != nil {
		jsonError(w, "failed to update config", http.StatusInternalServerError)
		return
	}

	jsonOK(w, existing)
}

// GET  /api/logs  — query historical logs
// Params: service_id, level (repeatable), task_id, documento, module, search,
//
//	from (unix ms), to (unix ms), limit, offset
func (a *API) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	serviceIDs := q["service_id"] // may be empty → query all services

	// If no service_id provided, collect all known service IDs.
	if len(serviceIDs) == 0 {
		services, err := a.db.ListServices()
		if err != nil {
			jsonError(w, "failed to list services", http.StatusInternalServerError)
			return
		}
		for _, svc := range services {
			serviceIDs = append(serviceIDs, svc.ID)
		}
	}

	baseReq := &pb.QueryRequest{
		Levels:    q["level"],
		TaskId:    q.Get("task_id"),
		Documento: q.Get("documento"),
		Module:    q.Get("module"),
		Search:    q.Get("search"),
		FromTs:    parseInt64(q.Get("from"), 0),
		ToTs:      parseInt64(q.Get("to"), 0),
		Limit:     int32(parseInt64(q.Get("limit"), 100)),
		Offset:    int32(parseInt64(q.Get("offset"), 0)),
	}

	// Merge results across all requested services.
	var allEntries []*pb.LogEntry
	var grandTotal int32
	for _, svcID := range serviceIDs {
		req := *baseReq
		req.ServiceId = svcID
		entries, total, err := a.store.Query(&req)
		if err != nil {
			continue
		}
		allEntries = append(allEntries, entries...)
		grandTotal += int32(total)
	}

	// Ensure entries is never nil so JSON encodes as [] instead of null.
	if allEntries == nil {
		allEntries = []*pb.LogEntry{}
	}

	// Respect limit across merged results.
	limit := baseReq.Limit
	if limit <= 0 {
		limit = 100
	}
	if int32(len(allEntries)) > limit {
		allEntries = allEntries[:limit]
	}

	hasMore := int32(baseReq.Offset)+limit < grandTotal

	jsonOK(w, map[string]interface{}{
		"entries":  allEntries,
		"total":    grandTotal,
		"limit":    limit,
		"offset":   baseReq.Offset,
		"has_more": hasMore,
	})
}

// DELETE /api/logs/{service_id}?days=N
// Deletes log files for service_id older than N days.
// If ?days is omitted the service's configured TTL is used.
func (a *API) handleLogsByService(w http.ResponseWriter, r *http.Request) {
	serviceID := strings.TrimPrefix(r.URL.Path, "/api/logs/")
	if serviceID == "" {
		jsonError(w, "service_id required", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodDelete {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Resolve TTL: prefer explicit ?days param, fall back to service config.
	var days int32
	if d := r.URL.Query().Get("days"); d != "" {
		days = int32(parseInt64(d, 0))
	}
	if days <= 0 {
		cfg, err := a.db.GetConfig(serviceID)
		if err == nil && cfg != nil {
			days = cfg.TtlDays
		}
	}
	if days <= 0 {
		days = int32(a.cfg.DefaultTTLDays)
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -int(days))
	deleted, err := a.store.DeleteOlderThan(serviceID, cutoff)
	if err != nil {
		jsonError(w, "delete failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, map[string]interface{}{
		"service_id":    serviceID,
		"deleted_files": deleted,
		"cutoff_date":   cutoff.Format("2006-01-02"),
	})
}

// GET /api/logs/tasks — list available task_ids with metadata
// Params: service_id (repeatable), from (unix ms), to (unix ms)
func (a *API) handleListTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	serviceIDs := q["service_id"]

	// If no service_id provided, use all known services.
	if len(serviceIDs) == 0 {
		services, err := a.db.ListServices()
		if err != nil {
			jsonError(w, "failed to list services", http.StatusInternalServerError)
			return
		}
		for _, svc := range services {
			serviceIDs = append(serviceIDs, svc.ID)
		}
	}

	fromTs := parseInt64(q.Get("from"), 0)
	toTs := parseInt64(q.Get("to"), 0)

	tasks, err := a.store.ListTasks(serviceIDs, fromTs, toTs)
	if err != nil {
		jsonError(w, "failed to list tasks", http.StatusInternalServerError)
		return
	}
	if tasks == nil {
		tasks = []store.TaskInfo{}
	}

	jsonOK(w, map[string]interface{}{
		"tasks": tasks,
		"total": len(tasks),
	})
}

// GET /api/stats — aggregate dashboard statistics
func (a *API) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	services, err := a.db.ListServices()
	if err != nil {
		jsonError(w, "failed to fetch services", http.StatusInternalServerError)
		return
	}

	online := 0
	for _, svc := range services {
		if svc.Status == "online" {
			online++
		}
	}

	// Count today's total log lines across all services.
	today := time.Now().UTC().Format("2006-01-02")
	totalToday := 0
	for _, svc := range services {
		dates, _ := a.store.ListDates(svc.ID)
		for _, d := range dates {
			if d == today {
				totalToday++ // file exists for today; a full count would be expensive
			}
		}
	}

	// Collect dedup drop counts per service.
	dedupDrops := make(map[string]int64)
	var totalDropped int64
	if a.dedup != nil {
		ctx := r.Context()
		for _, svc := range services {
			count := a.dedup.GetDropCount(ctx, svc.ID)
			if count > 0 {
				dedupDrops[svc.ID] = count
				totalDropped += count
			}
		}
	}

	jsonOK(w, map[string]interface{}{
		"services_total":         len(services),
		"services_online":        online,
		"log_files_today":        totalToday,
		"server_time":            time.Now().UnixMilli(),
		"duplicates_dropped_today": totalDropped,
		"dedup_drops_by_service": dedupDrops,
	})
}

// GET /api/healthmon — returns dependency (circuit breaker) status for all services
func (a *API) handleHealthmon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if a.depState == nil {
		jsonOK(w, map[string]interface{}{"services": []interface{}{}})
		return
	}

	// Optional filter by service_id
	svcFilter := r.URL.Query().Get("service_id")
	if svcFilter != "" {
		dep := a.depState.Get(svcFilter)
		if dep == nil {
			jsonOK(w, map[string]interface{}{"services": []interface{}{}})
			return
		}
		jsonOK(w, map[string]interface{}{"services": []*depstate.ServiceDeps{dep}})
		return
	}

	all := a.depState.All()
	if all == nil {
		all = []*depstate.ServiceDeps{}
	}
	jsonOK(w, map[string]interface{}{"services": all})
}

// ---- helpers ----

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func parseInt64(s string, def int64) int64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return def
	}
	return v
}
