package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"kraken/internal/db"
	"kraken/internal/queue"
)

type Handler struct {
	store *db.Store
	queue *queue.RedisQueue
}

func NewHandler(store *db.Store, q *queue.RedisQueue) *Handler {
	return &Handler{store: store, queue: q}
}

func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/v1", func(v1 chi.Router) {
		v1.Get("/projects", h.listProjects)
		v1.Post("/projects", h.createProject)
		v1.Patch("/projects/{projectID}/autofix", h.patchProjectAutofix)
		v1.Get("/projects/{projectID}/checks", h.listProjectChecks)
		v1.Post("/projects/{projectID}/checks", h.createProjectCheck)
		v1.Post("/projects/{projectID}/run-now", h.runProjectNow)
		v1.Post("/smtp_profiles", h.createSMTPProfile)
	})

	return r
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

func parseIDParam(r *http.Request, key string) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, key), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
