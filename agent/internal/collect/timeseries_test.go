package collect

import (
	"strconv"
	"testing"
	"time"
)

type hashMem struct {
	h map[string]map[string]string
}

func (m *hashMem) HMGet(key string, fields ...string) ([]string, error) {
	out := make([]string, len(fields))
	bag := m.h[key]
	for i, f := range fields {
		if bag == nil {
			continue
		}
		out[i] = bag[f]
	}
	return out, nil
}

func TestHourlyHitsReadsTimedTrafficHash(t *testing.T) {
	now := time.Unix(1_700_000_400, 0) // 2023-11-14 22:13:20 UTC
	slot := (now.Unix() / 3600) * 3600
	bucket := (slot / (7 * 86400)) * (7 * 86400)
	key := "timedTraffic:download:1hour:" + strconv.FormatInt(bucket, 10)
	r := &hashMem{h: map[string]map[string]string{
		key: {strconv.FormatInt(slot, 10): "1024"},
	}}

	hits, err := timedHits(r, "download", granHour, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("len=%d", len(hits))
	}
	if hits[0].TS != slot || hits[0].Value != 1024 {
		t.Fatalf("%+v", hits[0])
	}
}

func TestFifteenMinuteHitsReadsTimedTrafficHash(t *testing.T) {
	now := time.Unix(1_700_000_400, 0)
	slot := (now.Unix() / 900) * 900
	bucket := (slot / (50 * 3600)) * (50 * 3600)
	key := "timedTraffic:upload:15minutes:" + strconv.FormatInt(bucket, 10)
	r := &hashMem{h: map[string]map[string]string{
		key: {strconv.FormatInt(slot, 10): "4096"},
	}}
	hits, err := timedHits(r, "upload", gran15m, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].TS != slot || hits[0].Value != 4096 {
		t.Fatalf("%+v", hits)
	}
}

func TestHourlyHitsMissingIsZero(t *testing.T) {
	now := time.Unix(1_700_000_400, 0)
	hits, err := timedHits(&hashMem{}, "upload", granHour, 2, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("len=%d", len(hits))
	}
	if hits[0].Value != 0 || hits[1].Value != 0 {
		t.Fatalf("%+v", hits)
	}
}
