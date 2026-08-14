package collect

import (
	"strconv"
	"time"
)

// Firewalla util/TimeSeries.js granularities (redis-timeseries + timedTraffic prefix).
const (
	gran15m  = "15minutes"
	granHour = "1hour"
	granDay  = "1day"

	min15Sec     = int64(900)
	hourSec      = int64(3600)
	daySec       = int64(86400)
	weekSec      = int64(7 * 86400)
	min15TTL     = int64(50 * 3600)    // 50 hours
	hourTTL      = int64(7 * 86400)    // 7 days
	dayTTL       = int64(53 * weekSec) // 53 weeks
	dayPrecision = int64(52 * weekSec) // 52 weeks
)

type hashMGet interface {
	HMGet(key string, fields ...string) ([]string, error)
}

type tsHit struct {
	TS    int64
	Value int64
}

func timedHits(r hashMGet, metric, gran string, count int, now time.Time) ([]tsHit, error) {
	if r == nil || count <= 0 {
		return nil, nil
	}
	duration, keyPrec := granParams(gran)
	cur := now.Unix()
	from := roundTime(duration, cur-int64(count)*duration)
	to := roundTime(duration, cur)

	type bucket struct {
		key    string
		fields []string
		slots  []int64
	}
	order := []string{}
	byKey := map[string]*bucket{}
	for ts := from; ts <= to; ts += duration {
		keyTS := roundTime(keyPrec, ts)
		key := "timedTraffic:" + metric + ":" + gran + ":" + strconv.FormatInt(keyTS, 10)
		b := byKey[key]
		if b == nil {
			b = &bucket{key: key}
			byKey[key] = b
			order = append(order, key)
		}
		b.fields = append(b.fields, strconv.FormatInt(ts, 10))
		b.slots = append(b.slots, ts)
	}

	got := map[int64]int64{}
	for _, key := range order {
		b := byKey[key]
		vals, err := r.HMGet(b.key, b.fields...)
		if err != nil {
			return nil, err
		}
		for i, slot := range b.slots {
			if i >= len(vals) || vals[i] == "" {
				continue
			}
			n, err := strconv.ParseInt(vals[i], 10, 64)
			if err != nil {
				continue
			}
			got[slot] = n
		}
	}

	out := make([]tsHit, 0, int((to-from)/duration)+1)
	for ts := from; ts <= to; ts += duration {
		out = append(out, tsHit{TS: ts, Value: got[ts]})
	}
	if len(out) > count {
		out = out[len(out)-count:]
	}
	return out, nil
}

func granParams(gran string) (duration, keyPrec int64) {
	switch gran {
	case gran15m:
		return min15Sec, min15TTL
	case granDay:
		return daySec, dayPrecision
	default:
		return hourSec, hourTTL
	}
}

func roundTime(precision, ts int64) int64 {
	if precision <= 0 {
		return ts
	}
	return (ts / precision) * precision
}

func sumHits(hits []tsHit) int64 {
	var n int64
	for _, h := range hits {
		n += h.Value
	}
	return n
}
