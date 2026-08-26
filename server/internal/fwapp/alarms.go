package fwapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// maxAlarmsInbox caps get-alarms list size (absurd upstream payloads).
const maxAlarmsInbox = 500

// AlarmsList is the parsed get alarms reply.
type AlarmsList struct {
	Count  int64
	Alarms []AlarmSample
}

// GetAlarms fetches active alarms via get item "alarms".
func (s *Service) GetAlarms(ctx context.Context) (AlarmsList, error) {
	var zero AlarmsList
	raw, err := s.sendGet(ctx, map[string]any{
		"item":  "alarms",
		"value": map[string]any{},
	})
	if err != nil {
		return zero, err
	}
	return parseAlarmsList(raw)
}

// IgnoreAlarm sends cmd alarm:ignore for one alarmID (string).
func (s *Service) IgnoreAlarm(ctx context.Context, alarmID string) error {
	id := strings.TrimSpace(alarmID)
	if id == "" {
		return fmt.Errorf("alarm id required")
	}
	_, err := s.sendCmd(ctx, map[string]any{
		"item":  "alarm:ignore",
		"value": map[string]any{"alarmID": id},
	})
	return err
}

// IgnoreAllAlarms sends cmd alarm:ignoreAll.
func (s *Service) IgnoreAllAlarms(ctx context.Context) error {
	_, err := s.sendCmd(ctx, map[string]any{
		"item":  "alarm:ignoreAll",
		"value": map[string]any{},
	})
	return err
}

func (s *Service) sendGet(ctx context.Context, data map[string]any) (json.RawMessage, error) {
	if !s.secretsReady() {
		return nil, fmt.Errorf("FIREPROXY_SECRETS_KEY required")
	}
	c, ok, err := s.vault.Load()
	if err != nil {
		return nil, err
	}
	if !ok || c.SymKey == "" {
		return nil, ErrNotPaired
	}
	raw, err := s.send(ctx, c, MTypeGet, data, "0.0.0.0")
	if err != nil {
		s.mu.Lock()
		s.lastPingOK = false
		s.lastPingAt = time.Now().UTC()
		s.state = "lan-down"
		s.lastErr = err.Error()
		s.mu.Unlock()
		return nil, err
	}
	s.mu.Lock()
	s.lastPingOK = true
	s.lastPingAt = time.Now().UTC()
	s.state = "lan-ok"
	s.lastErr = ""
	s.mu.Unlock()
	return raw, nil
}

func parseAlarmsList(raw json.RawMessage) (AlarmsList, error) {
	var zero AlarmsList
	data, err := unwrapCmdData(raw)
	if err != nil {
		return zero, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return zero, fmt.Errorf("fwapp: parse alarms: %w", err)
	}
	var alarmsRaw []json.RawMessage
	if arr, ok := m["alarms"]; ok && arr != nil {
		b, err := json.Marshal(arr)
		if err != nil {
			return zero, fmt.Errorf("fwapp: parse alarms list: %w", err)
		}
		if err := json.Unmarshal(b, &alarmsRaw); err != nil {
			return zero, fmt.Errorf("fwapp: parse alarms list: %w", err)
		}
	}
	if len(alarmsRaw) > maxAlarmsInbox {
		alarmsRaw = alarmsRaw[:maxAlarmsInbox]
	}
	return AlarmsList{
		Count:  int64(jsonFloat(m, "count")),
		Alarms: parseAlarmSamples(alarmsRaw),
	}, nil
}
