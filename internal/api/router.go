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
	uiDir         string
}

func NewHandler(store *db.Store, q *queue.RedisQueue, fixScriptsDir, uiDir string) *Handler {
	return &Handler{
		store:         store,
		queue:         q,
		fixScriptsDir: fixScriptsDir,
		uiDir:         uiDir,
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
		v1.Get("/projects/{projectID}/settings", h.getProjectSettings)
		v1.Put("/projects/{projectID}/settings", h.updateProjectSettings)
		v1.Get("/projects/{projectID}/checks", h.listProjectChecks)
		v1.Post("/projects/{projectID}/checks", h.createProjectCheck)
		v1.Get("/projects/{projectID}/checks/{checkID}/runs", h.listCheckRunsByCheck)
		v1.Post("/projects/{projectID}/run-now", h.runProjectNow)
		v1.Get("/projects/{projectID}/logs", h.listProjectLogs)
		v1.Get("/projects/{projectID}/incidents", h.listProjectIncidents)
		v1.Get("/projects/{projectID}/check-runs", h.listProjectCheckRuns)
		v1.Get("/projects/{projectID}/paths/health", h.listPathHealth)
		v1.Get("/projects/{projectID}/uptime", h.getProjectUptime)
		v1.Get("/projects/{projectID}/fixes", h.listProjectFixes)
		v1.Post("/projects/{projectID}/fixes", h.createProjectFix)
		v1.Post("/projects/{projectID}/fixes/upload", h.uploadProjectFix)
		v1.Put("/projects/{projectID}/fixes/{fixID}", h.updateProjectFix)
		v1.Delete("/projects/{projectID}/fixes/{fixID}", h.deleteProjectFix)
		v1.Post("/projects/{projectID}/fixes/{fixID}/run", h.runProjectFix)
		v1.Get("/smtp_profiles", h.listSMTPProfiles)
		v1.Post("/smtp_profiles", h.createSMTPProfile)
	})

	h.mountWebUI(r)
	return r
}

func (h *Handler) mountWebUI(r chi.Router) {
	uiFS := h.resolveUIFS()
	if uiFS == nil {
		return
	}
	fileServer := http.FileServer(http.FS(uiFS))
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFileFS(w, req, uiFS, "index.html")
	})
	r.Get("/app.js", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFileFS(w, req, uiFS, "app.js")
	})
	r.Get("/styles.css", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFileFS(w, req, uiFS, "styles.css")
	})
	r.Handle("/web/*", http.StripPrefix("/web/", fileServer))
}

func (h *Handler) resolveUIFS() fs.FS {
	candidates := make([]string, 0, 2)
	if strings.TrimSpace(h.uiDir) != "" {
		candidates = append(candidates, h.uiDir)
	}
	candidates = append(candidates, "internal/api/web")

	for _, dir := range candidates {
		indexPath := filepath.Join(dir, "index.html")
		if info, err := os.Stat(indexPath); err == nil && !info.IsDir() {
			return os.DirFS(dir)
		}
	}

	sub, err := fs.Sub(webAssets, "web")
	if err != nil {
		return nil
	}
	return sub
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

type projectSettingsResponse struct {
	Project      db.Project              `json:"project"`
	Checks       []db.Check              `json:"checks"`
	SMTPProfiles []db.SMTPProfileSummary `json:"smtp_profiles"`
}

type updateProjectSettingsRequest struct {
	Name                     string                  `json:"name"`
	Domain                   string                  `json:"domain"`
	CheckIntervalSec         int                     `json:"check_interval_sec"`
	FailureThreshold         int                     `json:"failure_threshold"`
	AutofixEnabled           bool                    `json:"autofix_enabled"`
	MaxAutofixRetries        int                     `json:"max_autofix_retries"`
	SMTPProfileID            *int64                  `json:"smtp_profile_id"`
	AlertEmails              []string                `json:"alert_emails"`
	EmailSubjectOpened       string                  `json:"email_subject_opened"`
	EmailBodyOpened          string                  `json:"email_body_opened"`
	EmailSubjectResolved     string                  `json:"email_subject_resolved"`
	EmailBodyResolved        string                  `json:"email_body_resolved"`
	EmailSubjectRepeated     string                  `json:"email_subject_repeated"`
	EmailBodyRepeated        string                  `json:"email_body_repeated"`
	EmailSubjectAutofixLimit string                  `json:"email_subject_autofix_limit"`
	EmailBodyAutofixLimit    string                  `json:"email_body_autofix_limit"`
	Checks                   []db.ReplaceCheckParams `json:"checks"`
}

func (h *Handler) getProjectSettings(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseIDParam(r, "projectID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	project, err := h.store.GetProjectByID(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if project == nil {
		writeError(w, http.StatusNotFound, errors.New("project not found"))
		return
	}

	checks, err := h.store.ListChecksByProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	profiles, err := h.store.ListSMTPProfiles(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, projectSettingsResponse{
		Project:      *project,
		Checks:       checks,
		SMTPProfiles: profiles,
	})
}

func (h *Handler) updateProjectSettings(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseIDParam(r, "projectID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req updateProjectSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Domain = strings.TrimSpace(req.Domain)
	if req.Name == "" || req.Domain == "" {
		writeError(w, http.StatusBadRequest, errors.New("name and domain are required"))
		return
	}
	if req.CheckIntervalSec <= 0 {
		writeError(w, http.StatusBadRequest, errors.New("check_interval_sec must be greater than 0"))
		return
	}
	if req.FailureThreshold <= 0 {
		writeError(w, http.StatusBadRequest, errors.New("failure_threshold must be greater than 0"))
		return
	}

	if req.SMTPProfileID != nil {
		if *req.SMTPProfileID <= 0 {
			req.SMTPProfileID = nil
		} else {
			profile, err := h.store.GetSMTPProfile(r.Context(), *req.SMTPProfileID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			if profile == nil {
				writeError(w, http.StatusBadRequest, errors.New("smtp_profile_id not found"))
				return
			}
		}
	}

	checkInputs := req.Checks
	if checkInputs == nil {
		existingChecks, err := h.store.ListChecksByProject(r.Context(), projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		checkInputs = make([]db.ReplaceCheckParams, 0, len(existingChecks))
		for _, c := range existingChecks {
			id := c.ID
			checkInputs = append(checkInputs, db.ReplaceCheckParams{
				ID:             &id,
				Type:           c.Type,
				Target:         c.Target,
				TimeoutMs:      c.TimeoutMs,
				ExpectedStatus: c.ExpectedStatus,
			})
		}
	}

	updatedProject, err := h.store.UpdateProject(r.Context(), projectID, db.UpdateProjectParams{
		Name:                     req.Name,
		Domain:                   req.Domain,
		CheckIntervalSec:         req.CheckIntervalSec,
		FailureThreshold:         req.FailureThreshold,
		AutofixEnabled:           req.AutofixEnabled,
		MaxAutofixRetries:        req.MaxAutofixRetries,
		SMTPProfileID:            req.SMTPProfileID,
		AlertEmails:              normalizeEmails(req.AlertEmails),
		EmailSubjectOpened:       req.EmailSubjectOpened,
		EmailBodyOpened:          req.EmailBodyOpened,
		EmailSubjectResolved:     req.EmailSubjectResolved,
		EmailBodyResolved:        req.EmailBodyResolved,
		EmailSubjectRepeated:     req.EmailSubjectRepeated,
		EmailBodyRepeated:        req.EmailBodyRepeated,
		EmailSubjectAutofixLimit: req.EmailSubjectAutofixLimit,
		EmailBodyAutofixLimit:    req.EmailBodyAutofixLimit,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	updatedChecks, err := h.store.ReplaceProjectChecks(r.Context(), projectID, checkInputs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	profiles, err := h.store.ListSMTPProfiles(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, projectSettingsResponse{
		Project:      updatedProject,
		Checks:       updatedChecks,
		SMTPProfiles: profiles,
	})
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

func (h *Handler) listCheckRunsByCheck(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseIDParam(r, "projectID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	checkID, err := parseIDParam(r, "checkID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	runs, err := h.store.ListCheckRunsByCheck(r.Context(), projectID, checkID, parseLimit(r, 120))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (h *Handler) listPathHealth(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseIDParam(r, "projectID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	paths, err := h.store.ListPathHealthByProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, paths)
}

type uptimeResponse struct {
	Window      string           `json:"window"`
	BucketSec   int              `json:"bucket_sec"`
	Start       time.Time        `json:"start"`
	End         time.Time        `json:"end"`
	UptimeRatio float64          `json:"uptime_ratio"`
	Points      []db.UptimePoint `json:"points"`
}

func (h *Handler) getProjectUptime(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseIDParam(r, "projectID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	windowName := strings.TrimSpace(r.URL.Query().Get("window"))
	if windowName == "" {
		windowName = "1h"
	}

	windowDur, bucketDur, err := uptimeWindowConfig(windowName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	now := time.Now().UTC()
	end := alignToBucket(now, bucketDur)
	if !end.After(now) {
		end = end.Add(bucketDur)
	}
	start := end.Add(-windowDur)

	points, err := h.store.GetUptimeSeries(r.Context(), projectID, start, end, bucketDur)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	totalUp := 0
	totalKnown := 0
	for _, p := range points {
		totalUp += p.UpSeconds
		totalKnown += p.UpSeconds + p.DownSeconds
	}
	ratio := 0.0
	if totalKnown > 0 {
		ratio = float64(totalUp) / float64(totalKnown)
	}

	writeJSON(w, http.StatusOK, uptimeResponse{
		Window:      windowName,
		BucketSec:   int(bucketDur.Seconds()),
		Start:       start,
		End:         end,
		UptimeRatio: ratio,
		Points:      points,
	})
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

func (h *Handler) updateProjectFix(w http.ResponseWriter, r *http.Request) {
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

	existing, err := h.store.GetProjectFix(r.Context(), projectID, fixID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, errors.New("fix not attached to project"))
		return
	}

	var req db.UpdateFixParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.ScriptPath) == "" || strings.TrimSpace(req.SupportedErrorPattern) == "" {
		writeError(w, http.StatusBadRequest, errors.New("name, script_path and supported_error_pattern are required"))
		return
	}

	updated, err := h.store.UpdateFix(r.Context(), fixID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) deleteProjectFix(w http.ResponseWriter, r *http.Request) {
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

	existing, err := h.store.GetProjectFix(r.Context(), projectID, fixID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, errors.New("fix not attached to project"))
		return
	}

	if err := h.store.DetachFixFromProject(r.Context(), projectID, fixID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := h.store.DeleteFix(r.Context(), fixID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"deleted": true,
		"fix_id":  fixID,
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
	writeJSON(w, http.StatusCreated, db.SMTPProfileSummary{
		ID:        profile.ID,
		Host:      profile.Host,
		Port:      profile.Port,
		Username:  profile.Username,
		FromEmail: profile.FromEmail,
	})
}

func (h *Handler) listSMTPProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.store.ListSMTPProfiles(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, profiles)
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

func normalizeEmails(raw []string) []string {
	if raw == nil {
		return []string{}
	}
	res := make([]string, 0, len(raw))
	for _, item := range raw {
		trimmed := strings.TrimSpace(strings.ToLower(item))
		if trimmed == "" {
			continue
		}
		res = append(res, trimmed)
	}
	return res
}

func uptimeWindowConfig(window string) (time.Duration, time.Duration, error) {
	switch window {
	case "1h":
		return 1 * time.Hour, 1 * time.Minute, nil
	case "12h":
		return 12 * time.Hour, 5 * time.Minute, nil
	case "1d":
		return 24 * time.Hour, 15 * time.Minute, nil
	case "7d":
		return 7 * 24 * time.Hour, 1 * time.Hour, nil
	case "30d":
		return 30 * 24 * time.Hour, 6 * time.Hour, nil
	default:
		return 0, 0, errors.New("window must be one of: 1h, 12h, 1d, 7d, 30d")
	}
}

func alignToBucket(ts time.Time, bucket time.Duration) time.Time {
	if bucket <= 0 {
		return ts
	}
	return ts.Truncate(bucket)
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
