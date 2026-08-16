package controlhist

import (
	"encoding/json"
	"log"

	"fireproxy/server/internal/store"
)

// Recorder records control-plane write outcomes.
type Recorder interface {
	Record(Outcome)
}

// Inserter persists one control history row.
type Inserter interface {
	InsertControlEvent(e store.ControlEvent) error
}

type persistRecorder struct {
	ins Inserter
}

// New returns a Recorder backed by the given inserter (typically *store.Persist).
func New(ins Inserter) Recorder {
	if ins == nil {
		return nopRecorder{}
	}
	return &persistRecorder{ins: ins}
}

type nopRecorder struct{}

func (nopRecorder) Record(Outcome) {}

func (r *persistRecorder) Record(o Outcome) {
	if ShouldSkip(o.Err) || ShouldSkip(o.Skip) {
		return
	}
	beforeJSON, err := marshalSnapshot(o.Before)
	if err != nil {
		log.Printf("controlhist: marshal before: %v", err)
		return
	}
	afterJSON := ""
	if o.Err == nil {
		afterJSON, err = marshalSnapshot(o.After)
		if err != nil {
			log.Printf("controlhist: marshal after: %v", err)
			return
		}
	}
	errMsg := ""
	if o.Err != nil {
		errMsg = o.Err.Error()
	}
	e := store.ControlEvent{
		Scheme:     o.Scheme,
		Action:     o.Action,
		ActorKind:  o.ActorKind,
		Actor:      o.Actor,
		Target:     o.Target,
		Summary:    o.Summary,
		Result:     MapError(o.Err),
		Error:      errMsg,
		BeforeJSON: beforeJSON,
		AfterJSON:  afterJSON,
	}
	if err := r.ins.InsertControlEvent(e); err != nil {
		log.Printf("controlhist: insert: %v", err)
	}
}

func marshalSnapshot(v map[string]any) (string, error) {
	if len(v) == 0 {
		return "", nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
