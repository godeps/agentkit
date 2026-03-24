package checkpoint

import (
	"encoding/json"
	"time"
)

type envelope struct {
	Version   int             `json:"version,omitempty"`
	ID        string          `json:"id,omitempty"`
	Session   string          `json:"session,omitempty"`
	Kind      Kind            `json:"kind,omitempty"`
	State     json.RawMessage `json:"state,omitempty"`
	CreatedAt time.Time       `json:"created_at,omitempty"`
}

func EncodeRecord(rec Record) ([]byte, error) {
	state, err := json.Marshal(rec.State)
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope{
		Version:   1,
		ID:        rec.ID,
		Session:   rec.Session,
		Kind:      rec.Kind,
		State:     state,
		CreatedAt: rec.CreatedAt,
	})
}

func DecodeRecord(data []byte) (Record, error) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return Record{}, err
	}
	var state any
	if len(env.State) > 0 {
		if err := json.Unmarshal(env.State, &state); err != nil {
			return Record{}, err
		}
	}
	return Record{
		ID:        env.ID,
		Session:   env.Session,
		Kind:      env.Kind,
		State:     state,
		CreatedAt: env.CreatedAt,
	}, nil
}
