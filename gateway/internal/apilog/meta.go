// Package apilog carries allowlisted /v1 request metadata across middleware.
package apilog

import "context"

type contextKey struct{}
type recordStateKey struct{}

type recordState struct{ recorded bool }

// Meta deliberately contains no request or response content and no API key.
type Meta struct {
	UserID       string
	KeyID        string
	Model        string
	Provider     string
	InputTokens  int
	OutputTokens int
	CachedTokens int
	CostTokens   int64
	Fallback     bool
	ErrorCode    string
}

func With(ctx context.Context) (context.Context, *Meta) {
	meta := &Meta{}
	state := &recordState{}
	ctx = context.WithValue(ctx, contextKey{}, meta)
	ctx = context.WithValue(ctx, recordStateKey{}, state)
	return ctx, meta
}

func From(ctx context.Context) *Meta {
	meta, _ := ctx.Value(contextKey{}).(*Meta)
	return meta
}

// SetError records a machine-readable rejection without carrying request data.
func SetError(ctx context.Context, code string) {
	if meta := From(ctx); meta != nil {
		meta.ErrorCode = code
	}
}

func Recorded(ctx context.Context) bool {
	state, _ := ctx.Value(recordStateKey{}).(*recordState)
	return state != nil && state.recorded
}

func markRecorded(ctx context.Context) {
	if state, _ := ctx.Value(recordStateKey{}).(*recordState); state != nil {
		state.recorded = true
	}
}
