package fwapp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"fireproxy/server/internal/tplink"
)

const alarmsStubReply = `{"code":200,"data":{"count":2,"alarms":[{"aid":"1","type":"ALARM_LARGE_UPLOAD","message":"x","timestamp":"1700000000","p.device.mac":"aa:bb:cc:dd:ee:ff","p.device.ip":"10.0.0.2","p.device.name":"box"}]}}`

func TestGetAlarmsSendsGetItem(t *testing.T) {
	svc, sent := pairedMutateService(t)
	var gotMType string
	svc.SetSendFn(func(ctx context.Context, creds Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		gotMType = mtype
		item, _ := data["item"].(string)
		*sent = append(*sent, sentCmd{item: item, data: data, target: target})
		return json.RawMessage(alarmsStubReply), nil
	})

	list, err := svc.GetAlarms(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotMType != MTypeGet {
		t.Fatalf("mtype %q want %q", gotMType, MTypeGet)
	}
	if len(*sent) != 1 || (*sent)[0].item != "alarms" {
		t.Fatalf("sent %+v", *sent)
	}
	val, _ := (*sent)[0].data["value"].(map[string]any)
	if val == nil {
		t.Fatalf("missing value: %+v", (*sent)[0].data)
	}
	if list.Count != 2 {
		t.Fatalf("count=%d", list.Count)
	}
	if len(list.Alarms) != 1 {
		t.Fatalf("alarms=%d", len(list.Alarms))
	}
	a := list.Alarms[0]
	if a.AID != 1 || a.Type != "ALARM_LARGE_UPLOAD" || a.Message != "x" {
		t.Fatalf("%+v", a)
	}
	if a.Timestamp != 1700000000 || a.DeviceMAC != "AA:BB:CC:DD:EE:FF" || a.DeviceIP != "10.0.0.2" || a.DeviceName != "box" {
		t.Fatalf("%+v", a)
	}
}

func TestIgnoreAlarmSendsCmd(t *testing.T) {
	svc, sent := pairedMutateService(t)
	var gotMType string
	svc.SetSendFn(func(ctx context.Context, creds Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		gotMType = mtype
		item, _ := data["item"].(string)
		*sent = append(*sent, sentCmd{item: item, data: data, target: target})
		return json.RawMessage(`{"code":200}`), nil
	})

	if err := svc.IgnoreAlarm(context.Background(), "42"); err != nil {
		t.Fatal(err)
	}
	if gotMType != MTypeCmd {
		t.Fatalf("mtype %q want %q", gotMType, MTypeCmd)
	}
	if len(*sent) != 1 || (*sent)[0].item != "alarm:ignore" {
		t.Fatalf("sent %+v", *sent)
	}
	val, _ := (*sent)[0].data["value"].(map[string]any)
	if val["alarmID"] != "42" {
		t.Fatalf("value %+v", val)
	}
}

func TestIgnoreAllAlarmsSendsCmd(t *testing.T) {
	svc, sent := pairedMutateService(t)
	svc.SetSendFn(func(ctx context.Context, creds Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		if mtype != MTypeCmd {
			t.Fatalf("mtype %q", mtype)
		}
		item, _ := data["item"].(string)
		*sent = append(*sent, sentCmd{item: item, data: data, target: target})
		return json.RawMessage(`{"code":200}`), nil
	})

	if err := svc.IgnoreAllAlarms(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 1 || (*sent)[0].item != "alarm:ignoreAll" {
		t.Fatalf("sent %+v", *sent)
	}
	val, _ := (*sent)[0].data["value"].(map[string]any)
	if val == nil {
		t.Fatalf("missing value: %+v", (*sent)[0].data)
	}
}

func TestIgnoreAlarmNotPaired(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("11", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &CredentialVault{Store: NewMemStore(), Key: key}
	svc := NewServiceWithVault(v, NewLANClient())

	err = svc.IgnoreAlarm(context.Background(), "1")
	if !errors.Is(err, ErrNotPaired) {
		t.Fatalf("got %v want ErrNotPaired", err)
	}
}
