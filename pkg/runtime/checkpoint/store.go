package checkpoint

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("checkpoint: record not found")

type Kind string

const (
	KindPlan     Kind = "plan"
	KindApproval Kind = "approval"
)

type Record struct {
	ID        string
	Session   string
	Kind      Kind
	State     any
	CreatedAt time.Time
}

type Store interface {
	Save(context.Context, Record) error
	Load(context.Context, string) (Record, error)
	Delete(context.Context, string) error
}
