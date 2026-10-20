package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"fireproxy/server/internal/controlhist"
)

func (s *Server) postFWAppAlarmIgnore(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		AlarmID any `json:"alarm_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	aid := flexAlarmID(body.AlarmID)
	if aid == "" {
		http.Error(w, "alarm_id required", http.StatusBadRequest)
		return
	}
	kind, actor := s.controlActor(r)
	err := svc.IgnoreAlarm(r.Context(), aid)
	s.controlHist().Record(controlhist.Outcome{
		Scheme:    controlhist.SchemeFirewalla,
		Action:    controlhist.ActionAlarmIgnore,
		Target:    aid,
		Summary:   "ignore",
		ActorKind: kind,
		Actor:     actor,
		Err:       err,
	})
	if err != nil {
		writeFWAppRulesErr(w, svc, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "alarm_id": aid})
}

func (s *Server) postFWAppAlarmIgnoreAll(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	kind, actor := s.controlActor(r)
	err := svc.IgnoreAllAlarms(r.Context())
	s.controlHist().Record(controlhist.Outcome{
		Scheme:    controlhist.SchemeFirewalla,
		Action:    controlhist.ActionAlarmIgnoreAll,
		Target:    "all",
		Summary:   "ignore_all",
		ActorKind: kind,
		Actor:     actor,
		Err:       err,
	})
	if err != nil {
		writeFWAppRulesErr(w, svc, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func flexAlarmID(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case json.Number:
		return strings.TrimSpace(x.String())
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}
