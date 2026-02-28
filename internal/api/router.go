package api

import (
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"kraken/internal/db"
	"kraken/internal/queue"
)

//go:embed web/*
var webAssets embed.FS

type Handler struct {
	store         *db.Store
	queue         *queue.RedisQueue
	fixScriptsDir string
}

func NewHandler(store *db.Store, q *queue.RedisQueue, fixScriptsDir string) *Handler {
	return &Handler{
		store:         store,
		queue:         q,
		fixScriptsDir: fixScriptsDir,
	}
}

func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/v1", func(v1 chi.Router) {
		v1.Get("/projects", h.listProjects)
		v1.Post("/projects", h.createProject)
		v1.Delete("/projects/{projectID}", h.deleteProject)
		v1.Patch("/projects/{projectID}/autofix", h.patchProjectAutofix)
		v1.Get("/projects/{projectID}/checks", h.listProjectChecks)
		v1.Post("/projects/{projectID}/checks", h.createProjectCheck)
		v1.Post("/projects/{projectID}/run-now", h.runProjectNow)
		v1.Get("/projects/{projectID}/logs", h.listProjectLogs)
		v1.Get("/projects/{projectID}/incidents", h.listProjectIncidents)
		v1.Get("/projects/{projectID}/check-runs", h.listProjectCheckRuns)
		v1.Get("/projects/{projectID}/fixes", h.listProjectFixes)
		v1.Post("/projects/{projectID}/fixes", h.createProjectFix)
		v1.Post("/projects/{projectID}/fixes/upload", h.uploadProjectFix)
		v1.Post("/projects/{projectID}/fixes/{fixID}/run", h.runProjectFix)
		v1.Post("/smtp_profiles", h.createSMTPProfile)
	})

	h.mountWebUI(r)
	return r
}

func (h *Handler) mountWebUI(r chi.Router) {
	sub, err := fs.Sub(webAssets, "web")
	if err != nil {
		return
	}
	fileServer := http.FileServer(http.FS(sub))
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		http.ServeFileFS(w, req, sub, "index.html")
	})
	r.Get("/app.js", func(w http.ResponseWriter, req *http.Request) {
		http.ServeFileFS(w, req, sub, "app.js")
	})
	r.Get("/styles.css", func(w http.ResponseWriter, req *http.Request) {
		http.ServeFileFS(w, req, sub, "styles.css")
	})
	r.Handle("/web/*", http.StripPrefix("/web/", fileServer))
}

func (h *Handler) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.store.ListProjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func (h *Handler) createProject(w http.ResponseWriter, r *http.Request) {
	var req db.CreateProjectParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Domain) == "" || req.CheckIntervalSec <= 0 {
		writeError(w, http.StatusBadRequest, errors.New("name, domain and check_interval_sec are required"))
		return
	}
	project, err := h.store.CreateProject(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

func (h *Handler) deleteProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseIDParam(r, "projectID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.store.DeleteProject(r.Context(), projectID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deleted":    true,
		"project_id": projectID,
	})
}

type patchAutofixRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *Handler) patchProjectAutofix(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseIDParam(r, "projectID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req patchAutofixRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.store.SetProjectAutofix(r.Context(), projectID, req.Enabled); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project_id": projectID, "autofix_enabled": req.Enabled})
}

func (h *Handler) listProjectChecks(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseIDParam(r, "projectID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	checks, err := h.store.ListChecksByProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, checks)
}

func (h *Handler) createProjectCheck(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseIDParam(r, "projectID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req db.CreateCheckParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.ProjectID = projectID
	if strings.TrimSpace(req.Target) == "" || strings.TrimSpace(req.Type) == "" {
		writeError(w, http.StatusBadRequest, errors.New("type and target are required"))
		return
	}
	check, err := h.store.CreateCheck(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, check)
}

func (h *Handler) runProjectNow(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseIDParam(r, "projectID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	checks, err := h.store.ListChecksByProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for _, check := range checks {
		if err := h.queue.EnqueueCheck(r.Context(), queue.CheckJob{CheckID: check.ID, Reason: "manual"}); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"queued": len(checks), "project_id": projectID})
}

func (h *Handler) listProjectLogs(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseIDParam(r, "projectID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	logs, err := h.store.ListLogsByProject(r.Context(), projectID, parseLimit(r, 100))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (h *Handler) listProjectIncidents(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseIDParam(r, "projectID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	incidents, err := h.store.ListIncidentsByProject(r.Context(), projectID, parseLimit(r, 50))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, incidents)
}

func (h *Handler) listProjectCheckRuns(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseIDParam(r, "projectID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	runs, err := h.store.ListCheckRunsByProject(r.Context(), projectID, parseLimit(r, 100))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (h *Handler) listProjectFixes(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseIDParam(r, "projectID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	fixes, err := h.store.ListProjectFixes(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, fixes)
}

func (h *Handler) createProjectFix(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseIDParam(r, "projectID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req db.CreateFixParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.ScriptPath) == "" || strings.TrimSpace(req.SupportedErrorPattern) == "" {
		writeError(w, http.StatusBadRequest, errors.New("name, script_path and supported_error_pattern are required"))
		return
	}

	fix, err := h.store.CreateFix(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.store.AttachFixToProject(r.Context(), projectID, fix.ID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, fix)
}

func (h *Handler) uploadProjectFix(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseIDParam(r, "projectID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := r.ParseMultipartForm(2 << 20); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid multipart form"))
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	fixType := strings.TrimSpace(r.FormValue("type"))
	pattern := strings.TrimSpace(r.FormValue("supported_error_pattern"))
	timeoutSec, err := strconv.Atoi(strings.TrimSpace(r.FormValue("timeout_sec")))
	if err != nil || timeoutSec <= 0 {
		timeoutSec = 60
	}
	if name == "" || pattern == "" {
		writeError(w, http.StatusBadRequest, errors.New("name and supported_error_pattern are required"))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("file is required"))
		return
	}
	defer file.Close()

	if header.Size <= 0 || header.Size > (2<<20) {
		writeError(w, http.StatusBadRequest, errors.New("file must be between 1 byte and 2MB"))
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".sh" {
		writeError(w, http.StatusBadRequest, errors.New("only .sh files are allowed"))
		return
	}

	safeName := sanitizeFilename(strings.TrimSuffix(header.Filename, ext))
	if safeName == "" {
		safeName = "fix"
	}
	storedFileName := "uploaded-" + strconv.FormatInt(time.Now().UTC().Unix(), 10) + "-" + safeName + ".sh"

	if err := os.MkdirAll(h.fixScriptsDir, 0o750); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	absPath := filepath.Join(h.fixScriptsDir, storedFileName)
	out, err := os.OpenFile(absPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o750)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, io.LimitReader(file, 2<<20)); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	fix, err := h.store.CreateFix(r.Context(), db.CreateFixParams{
		Name:                  name,
		Type:                  fixType,
		ScriptPath:            storedFileName,
		SupportedErrorPattern: pattern,
		TimeoutSec:            timeoutSec,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.store.AttachFixToProject(r.Context(), projectID, fix.ID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"fix":         fix,
		"uploaded_as": storedFileName,
	})
}

type runFixRequest struct {
	RequestedBy string `json:"requested_by"`
}

func (h *Handler) runProjectFix(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseIDParam(r, "projectID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	fixID, err := parseIDParam(r, "fixID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	fix, err := h.store.GetProjectFix(r.Context(), projectID, fixID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if fix == nil {
		writeError(w, http.StatusNotFound, errors.New("fix not attached to project"))
		return
	}

	var req runFixRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if strings.TrimSpace(req.RequestedBy) == "" {
		req.RequestedBy = "api"
	}

	if err := h.queue.EnqueueFix(r.Context(), queue.FixJob{
		ProjectID:   projectID,
		FixID:       fixID,
		RequestedBy: req.RequestedBy,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"project_id":   projectID,
		"fix_id":       fixID,
		"queued":       true,
		"requested_by": req.RequestedBy,
	})
}

type createSMTPProfileRequest struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	FromEmail string `json:"from_email"`
}

func (h *Handler) createSMTPProfile(w http.ResponseWriter, r *http.Request) {
	var req createSMTPProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Host == "" || req.Port <= 0 || req.Username == "" || req.Password == "" || req.FromEmail == "" {
		writeError(w, http.StatusBadRequest, errors.New("host, port, username, password, from_email are required"))
		return
	}
	profile, err := h.store.CreateSMTPProfile(r.Context(), req.Host, req.Port, req.Username, req.Password, req.FromEmail)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, profile)
}

var filenameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = filenameSanitizer.ReplaceAllString(name, "-")
	name = strings.Trim(name, ".-")
	if len(name) > 64 {
		name = name[:64]
	}
	return name
}

func parseIDParam(r *http.Request, key string) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, key), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func parseLimit(r *http.Request, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	if parsed > 500 {
		return 500
	}
	return parsed
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
