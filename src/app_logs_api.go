// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const requestLogViewKey = "request_logs"

type logColumnDef struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	Category        string `json:"category"`
	Type            string `json:"type"`
	DefaultVisible  bool   `json:"default_visible"`
	Width           int    `json:"width"`
	RegexpFilter    bool   `json:"regexp_filter"`
	Exportable      bool   `json:"exportable"`
	TabulatorSorter string `json:"sorter"`
}

type logTypeDef struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

type requestLogAPIQuery struct {
	Page      int
	PageSize  int
	Start     time.Time
	End       time.Time
	Filters   map[string]*regexp.Regexp
	Columns   []string
	LogTypes  map[string]bool
	AgentID   string
	ExportAll bool
}

var activityLogTypes = []logTypeDef{
	{ID: "Request", Label: "Requests", Description: "Agent and proxy requests to Google Workspace or helper endpoints, including outcome, policy permission, client metadata, and target object details."},
	{ID: "Policy audit", Label: "Policy audits", Description: "Policy changes such as creating, updating, deleting, changing defaults, or applying a policy to a Workspace account."},
	{ID: "Workspace audit", Label: "Workspace audits", Description: "Workspace account changes such as connecting, disconnecting, renaming, reordering, or changing which proxy policy applies."},
	{ID: "Drive folder audit", Label: "Drive folder audits", Description: "Allowed Drive folder changes such as adding, updating, refreshing the cached tree, or deleting approved folders."},
	{ID: "User audit", Label: "User audits", Description: "Account-level activity such as signing in, signing out, accountability changes, timezone changes, and Activity logs view preferences."},
}

var requestLogColumns = []logColumnDef{
	{ID: "timestamp_local", Label: "Timestamp", Category: "Event", Type: "datetime", DefaultVisible: true, Width: 180, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "log_type", Label: "Log type", Category: "Event", Type: "string", DefaultVisible: true, Width: 150, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "service", Label: "Area", Category: "Request", Type: "string", DefaultVisible: true, Width: 115, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "action", Label: "Operation", Category: "Event", Type: "string", DefaultVisible: true, Width: 220, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "method", Label: "Method", Category: "Request", Type: "string", Width: 95, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "path", Label: "Path", Category: "Request", Type: "string", Width: 360, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "query", Label: "Query", Category: "Request", Type: "string", Width: 360, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "request_id", Label: "Request ID", Category: "Request", Type: "string", Width: 175, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "log_id", Label: "Log ID", Category: "Event", Type: "string", Width: 175, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "target_object_type", Label: "Target type", Category: "Target", Type: "string", DefaultVisible: true, Width: 180, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "target_name", Label: "Target name", Category: "Target", Type: "string", DefaultVisible: true, Width: 240, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "target_object_id", Label: "Target ID", Category: "Target", Type: "string", Width: 240, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "outcome", Label: "Outcome", Category: "Outcome", Type: "string", DefaultVisible: true, Width: 110, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "http_status", Label: "HTTP status", Category: "Outcome", Type: "number", Width: 115, RegexpFilter: true, Exportable: true, TabulatorSorter: "number"},
	{ID: "upstream_status", Label: "Google status", Category: "Outcome", Type: "number", Width: 120, RegexpFilter: true, Exportable: true, TabulatorSorter: "number"},
	{ID: "error_message", Label: "Error message", Category: "Outcome", Type: "string", Width: 300, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "policy_capability_title", Label: "Policy permission", Category: "Policy", Type: "string", Width: 240, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "policy_name", Label: "Policy name", Category: "Policy", Type: "string", Width: 180, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "policy_rule_name", Label: "Policy rule", Category: "Policy", Type: "string", Width: 220, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "policy_id", Label: "Policy ID", Category: "Policy", Type: "string", Width: 160, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "policy_capability_key", Label: "Policy permission key", Category: "Policy", Type: "string", Width: 220, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "workspace_email", Label: "Workspace account", Category: "Workspace", Type: "string", DefaultVisible: true, Width: 220, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "user_email", Label: "Account owner", Category: "Workspace", Type: "string", Width: 220, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "agent_name", Label: "Agent name", Category: "Agent", Type: "string", DefaultVisible: true, Width: 180, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "agent_location", Label: "Agent location", Category: "Agent", Type: "string", Width: 220, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "agent_id", Label: "Agent ID", Category: "Agent", Type: "string", Width: 175, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "agent_motive", Label: "Agent motive", Category: "Agent", Type: "string", Width: 360, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "human_approval", Label: "Human approval", Category: "Agent", Type: "string", Width: 360, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "remote_addr", Label: "Connection IP", Category: "Client", Type: "string", Width: 180, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "cf_connecting_ip", Label: "CF-Connecting-IP", Category: "Client", Type: "string", Width: 180, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "x_forwarded_for", Label: "X-Forwarded-For", Category: "Client", Type: "string", Width: 220, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "x_real_ip", Label: "X-Real-IP", Category: "Client", Type: "string", Width: 160, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "forwarded", Label: "Forwarded", Category: "Client", Type: "string", Width: 220, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "user_agent", Label: "User agent", Category: "Client", Type: "string", Width: 360, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
	{ID: "details_json", Label: "Details", Category: "Event", Type: "string", Width: 420, RegexpFilter: true, Exportable: true, TabulatorSorter: "string"},
}

var defaultRequestLogColumnIDs = []string{
	"timestamp_local",
	"log_type",
	"service",
	"action",
	"target_object_type",
	"target_name",
	"outcome",
	"workspace_email",
	"agent_name",
}

func requestLogColumnByID() map[string]logColumnDef {
	out := make(map[string]logColumnDef, len(requestLogColumns))
	for _, col := range requestLogColumns {
		out[col.ID] = col
	}
	return out
}

func defaultRequestLogColumns() []string {
	return normalizeRequestLogColumns(defaultRequestLogColumnIDs)
}

func allRequestLogColumnIDs() []string {
	out := make([]string, 0, len(requestLogColumns))
	for _, col := range requestLogColumns {
		out = append(out, col.ID)
	}
	return out
}

func activityLogTypeByID() map[string]logTypeDef {
	out := make(map[string]logTypeDef, len(activityLogTypes))
	for _, logType := range activityLogTypes {
		out[logType.ID] = logType
	}
	return out
}

func (a *App) handleRequestLogColumnsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	if a.requireSessionAPIUser(w, r) == nil {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"columns":         requestLogColumns,
		"default_columns": defaultRequestLogColumns(),
		"log_types":       activityLogTypes,
		"page_sizes":      []int{50, 100, 150, 200, 250},
	})
}

func (a *App) handleRequestLogViewSettingsAPI(w http.ResponseWriter, r *http.Request) {
	user := a.requireSessionAPIUser(w, r)
	if user == nil {
		return
	}
	switch r.Method {
	case http.MethodGet:
		settings, err := a.requestLogViewSettings(user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "view_settings_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"visible_columns": settings.VisibleColumns,
			"column_widths":   settings.ColumnWidths,
			"page_size":       settings.PageSize,
		})
	case http.MethodPost:
		if !a.requireSessionCSRF(w, r) {
			return
		}
		if err := r.ParseForm(); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_form", err.Error())
			return
		}
		previous, _ := a.requestLogViewSettings(user.ID)
		columns := normalizeRequestLogColumns(strings.Split(r.FormValue("visible_columns"), ","))
		if len(columns) == 0 {
			writeError(w, http.StatusBadRequest, "columns_required", "select at least one column")
			return
		}
		widths, err := parseRequestLogColumnWidths(r.FormValue("column_widths"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_column_widths", err.Error())
			return
		}
		pageSize := parsePageSize(r.FormValue("page_size"))
		if err := a.store.SaveUserLogViewSettings(&UserLogViewSettings{UserID: user.ID, ViewKey: requestLogViewKey, VisibleColumns: columns, ColumnWidths: widths, PageSize: pageSize}); err != nil {
			writeError(w, http.StatusBadRequest, "view_settings_error", err.Error())
			return
		}
		details := map[string]any{
			"field":               "activity_log_view",
			"new_visible_columns": columns,
			"new_page_size":       pageSize,
			"column_width_count":  len(widths),
			"source":              "api",
		}
		if previous != nil {
			details["previous_visible_columns"] = previous.VisibleColumns
			details["previous_page_size"] = previous.PageSize
		}
		a.logUserAudit(r, user, "user_settings_updated", requestLogViewKey, "Activity log view settings", details)
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET or POST required")
	}
}

func (a *App) handleRequestLogsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	user := a.requireSessionAPIUser(w, r)
	if user == nil {
		return
	}
	settings, err := a.store.GetUserSettings(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settings_error", err.Error())
		return
	}
	loggingSettings, err := a.store.GetUserLoggingSettings(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "logging_settings_error", err.Error())
		return
	}
	query, err := a.parseRequestLogAPIQuery(r.URL.Query(), user, settings, loggingSettings, false)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_log_query", err.Error())
		return
	}
	rows, total, err := a.filteredRequestLogRows(user.ID, settings.Timezone, query)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_log_query", err.Error())
		return
	}
	lastPage := 1
	if total > 0 {
		lastPage = (total + query.PageSize - 1) / query.PageSize
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":      rows,
		"total":     total,
		"page":      query.Page,
		"page_size": query.PageSize,
		"last_page": lastPage,
	})
}

func (a *App) handleRequestLogExportAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	user := a.requireSessionAPIUser(w, r)
	if user == nil {
		return
	}
	settings, err := a.store.GetUserSettings(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settings_error", err.Error())
		return
	}
	loggingSettings, err := a.store.GetUserLoggingSettings(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "logging_settings_error", err.Error())
		return
	}
	exportAll := r.URL.Query().Get("scope") == "all"
	query, err := a.parseRequestLogAPIQuery(r.URL.Query(), user, settings, loggingSettings, exportAll)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_log_query", err.Error())
		return
	}
	if !exportAll {
		query.Page = 1
		query.PageSize = 0
	}
	rows, _, err := a.filteredRequestLogRows(user.ID, settings.Timezone, query)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_log_query", err.Error())
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	if format == "" {
		format = "jsonl"
	}
	columns := query.Columns
	if exportAll {
		columns = allRequestLogColumnIDs()
	}
	switch format {
	case "jsonl":
		a.writeRequestLogJSONL(w, rows, columns)
	case "csv":
		a.writeRequestLogCSV(w, rows, columns)
	default:
		writeError(w, http.StatusBadRequest, "invalid_format", "format must be jsonl or csv")
	}
}

func (a *App) writeRequestLogJSONL(w http.ResponseWriter, rows []map[string]any, columns []string) {
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="activity-logs.jsonl"`)
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	for _, row := range rows {
		projected := map[string]any{}
		for _, col := range columns {
			projected[col] = row[col]
		}
		if err := encoder.Encode(projected); err != nil {
			return
		}
	}
}

func projectLogRows(rows []map[string]any, columns []string) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		projected := map[string]any{}
		for _, col := range columns {
			projected[col] = row[col]
		}
		out = append(out, projected)
	}
	return out
}

func (a *App) requireSessionAPIUser(w http.ResponseWriter, r *http.Request) *User {
	user, err := a.currentUserFromSession(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", err.Error())
		return nil
	}
	if user == nil || user.IsSuspended {
		writeError(w, http.StatusUnauthorized, "auth_required", "valid browser session required")
		return nil
	}
	return user
}

func (a *App) requestLogViewSettings(userID string) (*UserLogViewSettings, error) {
	settings, err := a.store.GetUserLogViewSettings(userID, requestLogViewKey)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return &UserLogViewSettings{UserID: userID, ViewKey: requestLogViewKey, VisibleColumns: defaultRequestLogColumns(), ColumnWidths: map[string]int{}, PageSize: 100}, nil
	}
	settings.VisibleColumns = normalizeRequestLogColumns(settings.VisibleColumns)
	if len(settings.VisibleColumns) == 0 {
		settings.VisibleColumns = defaultRequestLogColumns()
	}
	settings.ColumnWidths = normalizeRequestLogColumnWidths(settings.ColumnWidths)
	if settings.PageSize < 50 || settings.PageSize > 250 {
		settings.PageSize = 100
	}
	return settings, nil
}

func normalizeRequestLogColumns(columns []string) []string {
	known := requestLogColumnByID()
	seen := map[string]bool{}
	out := []string{}
	for _, col := range columns {
		col = strings.TrimSpace(col)
		if col == "" || seen[col] {
			continue
		}
		if _, ok := known[col]; !ok {
			continue
		}
		seen[col] = true
		out = append(out, col)
	}
	return out
}

func parseRequestLogColumnWidths(raw string) (map[string]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]int{}, nil
	}
	var widths map[string]int
	if err := json.Unmarshal([]byte(raw), &widths); err != nil {
		return nil, fmt.Errorf("column widths must be JSON")
	}
	return normalizeRequestLogColumnWidths(widths), nil
}

func normalizeRequestLogColumnWidths(widths map[string]int) map[string]int {
	known := requestLogColumnByID()
	out := map[string]int{}
	for id, width := range widths {
		if _, ok := known[id]; !ok {
			continue
		}
		if width < 48 {
			width = 48
		}
		if width > 20000 {
			width = 20000
		}
		out[id] = width
	}
	return out
}

func parsePageSize(raw string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(raw))
	if n < 50 {
		return 50
	}
	if n > 250 {
		return 250
	}
	return n
}

func parsePositivePage(raw string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(raw))
	if n < 1 {
		return 1
	}
	return n
}

func parseActivityLogTypes(values url.Values) (map[string]bool, error) {
	rawValues, ok := values["log_type"]
	if !ok {
		return nil, nil
	}
	known := activityLogTypeByID()
	selected := map[string]bool{}
	for _, raw := range rawValues {
		for _, part := range strings.Split(raw, ",") {
			logType := strings.TrimSpace(part)
			if logType == "" {
				continue
			}
			if _, ok := known[logType]; !ok {
				return nil, fmt.Errorf("unknown log type %q", logType)
			}
			selected[logType] = true
		}
	}
	return selected, nil
}

func (a *App) parseRequestLogAPIQuery(values url.Values, user *User, settings *UserSettings, loggingSettings *UserLoggingSettings, exportAll bool) (requestLogAPIQuery, error) {
	query := requestLogAPIQuery{
		Page:      parsePositivePage(values.Get("page")),
		PageSize:  parsePageSize(values.Get("page_size")),
		Filters:   map[string]*regexp.Regexp{},
		ExportAll: exportAll,
	}
	retentionDays := 7
	if loggingSettings != nil && loggingSettings.RetentionDays > 0 {
		retentionDays = loggingSettings.RetentionDays
	}
	retentionDays = effectiveLoggingRetentionDays(user, retentionDays)
	startBound, endBound := logRetentionBounds(settings.Timezone, retentionDays)
	start, end, err := parseLogRange(values, settings.Timezone, startBound, endBound)
	if err != nil {
		return query, err
	}
	if start.Before(startBound) {
		return query, fmt.Errorf("start time is outside the retention window")
	}
	if end.After(endBound.Add(5 * time.Minute)) {
		return query, fmt.Errorf("end time cannot be in the future")
	}
	if end.After(endBound) {
		end = endBound
	}
	if end.Before(start) {
		return query, fmt.Errorf("end time must be after start time")
	}
	query.Start = start
	query.End = end
	query.AgentID = strings.TrimSpace(values.Get("agent_id"))
	if exportAll {
		query.Page = 1
		query.PageSize = 0
		query.Columns = allRequestLogColumnIDs()
		return query, nil
	}
	logTypes, err := parseActivityLogTypes(values)
	if err != nil {
		return query, err
	}
	query.LogTypes = logTypes
	query.Columns = normalizeRequestLogColumns(strings.Split(values.Get("columns"), ","))
	if len(query.Columns) == 0 {
		viewSettings, err := a.requestLogViewSettings(user.ID)
		if err != nil {
			return query, err
		}
		query.Columns = viewSettings.VisibleColumns
	}
	known := requestLogColumnByID()
	for key, raw := range values {
		if !strings.HasPrefix(key, "filter_") || len(raw) == 0 {
			continue
		}
		colID := strings.TrimPrefix(key, "filter_")
		col, ok := known[colID]
		if !ok || !col.RegexpFilter {
			return query, fmt.Errorf("unknown filter column %q", colID)
		}
		pattern := strings.TrimSpace(raw[0])
		if pattern == "" {
			continue
		}
		if len(pattern) > 512 {
			return query, fmt.Errorf("filter for %s is too long", col.Label)
		}
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return query, fmt.Errorf("invalid regex for %s: %w", col.Label, err)
		}
		query.Filters[colID] = compiled
	}
	return query, nil
}

func parseOptionalLogTime(raw string, fallback time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp %q", raw)
	}
	return t.UTC(), nil
}

func parseLogRange(values url.Values, timezone string, startBound, endBound time.Time) (time.Time, time.Time, error) {
	if strings.TrimSpace(values.Get("start_date")) != "" || strings.TrimSpace(values.Get("end_date")) != "" {
		loc, err := time.LoadLocation(strings.TrimSpace(timezone))
		if err != nil {
			loc = time.UTC
		}
		start := startBound
		if raw := strings.TrimSpace(values.Get("start_date")); raw != "" {
			start, err = parseLogDateStart(raw, loc)
			if err != nil {
				return time.Time{}, time.Time{}, err
			}
		}
		end := endBound
		if raw := strings.TrimSpace(values.Get("end_date")); raw != "" {
			end, err = parseLogDateStart(raw, loc)
			if err != nil {
				return time.Time{}, time.Time{}, err
			}
			end = end.AddDate(0, 0, 1)
			if end.After(endBound) {
				end = endBound
			}
		}
		return start, end, nil
	}
	start, err := parseOptionalLogTime(values.Get("start"), startBound)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := parseOptionalLogTime(values.Get("end"), endBound)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, end, nil
}

func parseLogDateStart(raw string, loc *time.Location) (time.Time, error) {
	day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(raw), loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid log date %q", raw)
	}
	return day.UTC(), nil
}

func logRetentionBounds(timezone string, retentionDays int) (time.Time, time.Time) {
	loc, err := time.LoadLocation(strings.TrimSpace(timezone))
	if err != nil {
		loc = time.UTC
	}
	nowLocal := nowUTC().In(loc)
	startLocal := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -(retentionDays - 1))
	return startLocal.UTC(), nowUTC()
}

func formatLogDateKey(t time.Time, timezone string) string {
	loc, err := time.LoadLocation(strings.TrimSpace(timezone))
	if err != nil {
		loc = time.UTC
	}
	return t.In(loc).Format("2006-01-02")
}

type activityLogRow struct {
	CreatedAt time.Time
	Row       map[string]any
}

func (a *App) filteredRequestLogRows(userID, timezone string, query requestLogAPIQuery) ([]map[string]any, int, error) {
	activityRows, err := a.activityLogRowsInRange(userID, timezone, query.Start, query.End)
	if err != nil {
		return nil, 0, err
	}
	matched := make([]map[string]any, 0, len(activityRows))
	for _, activityRow := range activityRows {
		if query.AgentID != "" && fmt.Sprint(activityRow.Row["agent_id"]) != query.AgentID {
			continue
		}
		if query.LogTypes != nil && !query.LogTypes[fmt.Sprint(activityRow.Row["log_type"])] {
			continue
		}
		if requestLogRowMatches(activityRow.Row, query.Filters) {
			matched = append(matched, activityRow.Row)
		}
	}
	total := len(matched)
	if query.PageSize == 0 {
		return matched, total, nil
	}
	start := (query.Page - 1) * query.PageSize
	if start >= total {
		return []map[string]any{}, total, nil
	}
	end := start + query.PageSize
	if end > total {
		end = total
	}
	return matched[start:end], total, nil
}

func (a *App) activityLogRowsInRange(userID, timezone string, start, end time.Time) ([]activityLogRow, error) {
	entries, err := a.store.ListRequestLogsInRange(userID, start, end)
	if err != nil {
		return nil, err
	}
	rows := make([]activityLogRow, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, activityLogRow{CreatedAt: entry.CreatedAt, Row: requestLogRowMap(entry, timezone)})
	}
	for _, source := range []struct {
		table          string
		logType        string
		service        string
		targetType     string
		defaultOutcome string
	}{
		{table: "policy_audit_logs", logType: "Policy audit", service: "policy", targetType: "Policy", defaultOutcome: "Success"},
		{table: "workspace_audit_logs", logType: "Workspace audit", service: "workspace", targetType: "Workspace", defaultOutcome: "Success"},
		{table: "drive_folder_audit_logs", logType: "Drive folder audit", service: "drive-folder", targetType: "Drive folder", defaultOutcome: "Success"},
		{table: "user_audit_logs", logType: "User audit", service: "user", targetType: "User account", defaultOutcome: "Success"},
	} {
		audits, err := a.store.ListAuditLogsInRange(source.table, userID, start, end)
		if err != nil {
			return nil, err
		}
		for _, audit := range audits {
			rows = append(rows, activityLogRow{CreatedAt: audit.CreatedAt, Row: auditLogRowMap(audit, timezone, source.logType, source.service, source.targetType, source.defaultOutcome)})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return fmt.Sprint(rows[i].Row["log_id"]) > fmt.Sprint(rows[j].Row["log_id"])
		}
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})
	return rows, nil
}

func requestLogRowMatches(row map[string]any, filters map[string]*regexp.Regexp) bool {
	for column, filter := range filters {
		if !filter.MatchString(fmt.Sprint(row[column])) {
			return false
		}
	}
	return true
}

func requestLogRowMap(entry RequestLogEntry, timezone string) map[string]any {
	return map[string]any{
		"timestamp_local":         formatUserTime(entry.CreatedAt, timezone),
		"log_id":                  entry.ID,
		"log_type":                "Request",
		"action":                  requestLogAction(entry),
		"request_id":              entry.RequestID,
		"agent_id":                entry.AgentID,
		"user_email":              entry.UserEmail,
		"workspace_email":         entry.WorkspaceEmail,
		"method":                  entry.Method,
		"service":                 entry.Service,
		"path":                    entry.Path,
		"query":                   entry.Query,
		"target_object_type":      entry.TargetObjectType,
		"target_object_id":        entry.TargetObjectID,
		"target_name":             requestLogTargetName(entry),
		"user_agent":              entry.UserAgent,
		"remote_addr":             entry.RemoteAddr,
		"x_forwarded_for":         entry.XForwardedFor,
		"x_real_ip":               entry.XRealIP,
		"forwarded":               entry.Forwarded,
		"cf_connecting_ip":        entry.CFConnectingIP,
		"agent_name":              entry.AgentName,
		"agent_location":          entry.AgentLocation,
		"agent_motive":            entry.AgentMotive,
		"human_approval":          entry.HumanApproval,
		"outcome":                 entry.Outcome,
		"http_status":             entry.HTTPStatus,
		"upstream_status":         entry.UpstreamStatus,
		"policy_id":               entry.PolicyID,
		"policy_name":             entry.PolicyName,
		"policy_capability_key":   entry.PolicyCapabilityKey,
		"policy_capability_title": entry.PolicyCapabilityTitle,
		"policy_rule_name":        entry.PolicyRuleName,
		"error_message":           entry.ErrorMessage,
		"details_json":            "",
	}
}

func requestLogAction(entry RequestLogEntry) string {
	method := strings.TrimSpace(entry.Method)
	service := strings.TrimSpace(entry.Service)
	if method == "" {
		return service
	}
	if service == "" {
		return method
	}
	return method + " " + service
}

func requestLogTargetName(entry RequestLogEntry) string {
	return strings.TrimSpace(entry.TargetObjectID)
}

func auditLogRowMap(entry AuditLogEntry, timezone, logType, service, targetType, defaultOutcome string) map[string]any {
	targetType = auditTargetObjectType(entry, targetType)
	service = auditServiceLabel(entry, service)
	return map[string]any{
		"timestamp_local":         formatUserTime(entry.CreatedAt, timezone),
		"log_id":                  entry.ID,
		"log_type":                logType,
		"action":                  entry.Action,
		"request_id":              "",
		"user_email":              entry.UserEmail,
		"workspace_email":         entry.WorkspaceEmail,
		"method":                  "",
		"service":                 service,
		"path":                    "",
		"query":                   "",
		"target_object_type":      targetType,
		"target_object_id":        entry.TargetID,
		"target_name":             entry.TargetName,
		"user_agent":              auditClientField(entry.UserAgent, entry.DetailsJSON, "user_agent"),
		"remote_addr":             auditClientField(entry.RemoteAddr, entry.DetailsJSON, "remote_addr"),
		"x_forwarded_for":         auditClientField(entry.XForwardedFor, entry.DetailsJSON, "x_forwarded_for"),
		"x_real_ip":               auditClientField(entry.XRealIP, entry.DetailsJSON, "x_real_ip"),
		"forwarded":               auditClientField(entry.Forwarded, entry.DetailsJSON, "forwarded"),
		"cf_connecting_ip":        auditClientField(entry.CFConnectingIP, entry.DetailsJSON, "cf_connecting_ip"),
		"agent_name":              "",
		"agent_location":          "",
		"agent_motive":            "",
		"human_approval":          "",
		"outcome":                 defaultOutcome,
		"http_status":             "",
		"upstream_status":         "",
		"policy_id":               auditPolicyID(entry),
		"policy_name":             auditPolicyName(entry),
		"policy_capability_key":   "",
		"policy_capability_title": "",
		"policy_rule_name":        "",
		"error_message":           "",
		"details_json":            entry.DetailsJSON,
	}
}

func auditClientField(value, detailsJSON, key string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return auditDetailString(detailsJSON, key)
}

func auditTargetObjectType(entry AuditLogEntry, fallback string) string {
	switch entry.Action {
	case "agent_created", "agent_profile_updated", "agent_status_updated", "agent_deleted":
		return "Agent profile"
	case "agent_grants_updated":
		return "Agent Workspace grant"
	case "agent_drive_folder_grants_updated":
		return "Agent Drive folder grant"
	case "agent_token_rotated":
		return "Agent API key"
	case "agent_firewall_status_updated":
		return "Agent firewall"
	case "agent_firewall_address_added", "agent_firewall_address_deleted":
		return "Agent firewall address"
	case "logging_settings_updated":
		return "Logging settings"
	case "two_factor_enabled", "two_factor_reset", "two_factor_challenge_failed":
		return "Two-factor authentication"
	case "user_settings_updated":
		if entry.TargetID == requestLogViewKey {
			return "Activity log view settings"
		}
		return "User settings"
	case "agent_skill_downloaded":
		return "Agent skill package"
	case "user_signed_in", "user_signed_out":
		return "User session"
	case "default_policy_changed", "policy_created", "policy_deleted", "policy_updated", "workspace_policy_applied":
		return "Policy"
	case "workspace_connected", "workspace_auth_refreshed", "workspace_reconnected", "workspace_auth_disconnected", "workspace_settings_deleted", "workspace_friendly_name_updated":
		return "Workspace account"
	case "workspace_order_updated":
		return "Workspace ordering"
	case "drive_folder_added", "drive_folder_updated", "drive_folder_refreshed", "drive_folder_deleted":
		return "Allowed Drive folder"
	}
	if strings.TrimSpace(fallback) == "" {
		return "Object"
	}
	return fallback
}

func auditServiceLabel(entry AuditLogEntry, fallback string) string {
	switch entry.Action {
	case "agent_created", "agent_profile_updated", "agent_grants_updated", "agent_drive_folder_grants_updated", "agent_token_rotated", "agent_status_updated", "agent_deleted", "agent_skill_downloaded":
		return "agent"
	case "agent_firewall_status_updated", "agent_firewall_address_added", "agent_firewall_address_deleted":
		return "firewall"
	case "logging_settings_updated":
		return "logging"
	case "two_factor_enabled", "two_factor_reset", "two_factor_challenge_failed":
		return "auth"
	case "user_settings_updated":
		return "settings"
	case "user_signed_in", "user_signed_out":
		return "auth"
	default:
		return fallback
	}
}

func auditPolicyID(entry AuditLogEntry) string {
	switch entry.Action {
	case "default_policy_changed", "policy_created", "policy_deleted", "policy_updated", "workspace_policy_applied":
		return entry.TargetID
	default:
		return ""
	}
}

func auditPolicyName(entry AuditLogEntry) string {
	switch entry.Action {
	case "default_policy_changed", "policy_created", "policy_deleted", "policy_updated", "workspace_policy_applied":
		return entry.TargetName
	default:
		return ""
	}
}

func auditDetailString(detailsJSON, key string) string {
	var details map[string]any
	if err := json.Unmarshal([]byte(detailsJSON), &details); err != nil {
		return ""
	}
	value, ok := details[key]
	if !ok {
		return ""
	}
	return fmt.Sprint(value)
}

func (a *App) writeRequestLogCSV(w http.ResponseWriter, rows []map[string]any, columns []string) {
	known := requestLogColumnByID()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="activity-logs.csv"`)
	writer := csv.NewWriter(w)
	headers := []string{}
	for _, colID := range columns {
		if col, ok := known[colID]; ok && col.Exportable {
			headers = append(headers, col.Label)
		}
	}
	_ = writer.Write(headers)
	for _, row := range rows {
		record := []string{}
		for _, colID := range columns {
			if col, ok := known[colID]; ok && col.Exportable {
				record = append(record, csvSafe(fmt.Sprint(row[colID])))
			}
		}
		_ = writer.Write(record)
	}
	writer.Flush()
}

func csvSafe(value string) string {
	if value == "" {
		return value
	}
	switch value[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + value
	default:
		return value
	}
}

func sortedLogCategories(columns []logColumnDef) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, col := range columns {
		if !seen[col.Category] {
			seen[col.Category] = true
			out = append(out, col.Category)
		}
	}
	sort.Strings(out)
	return out
}
