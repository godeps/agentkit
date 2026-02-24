package model

import (
	"context"
	"errors"
)

// StreamOnlyModel wraps a Model so that Complete() internally uses
// CompleteStream() to collect the response. This works around API proxies
// that return empty tool_use.input in non-streaming mode but work
// correctly in streaming mode.
type StreamOnlyModel struct {
	Inner Model
}

// NewStreamOnlyModel returns a wrapper that forces all completions
// through the streaming path.
func NewStreamOnlyModel(inner Model) *StreamOnlyModel {
	return &StreamOnlyModel{Inner: inner}
}

// Complete calls CompleteStream internally and assembles the final Response.
func (s *StreamOnlyModel) Complete(ctx context.Context, req Request) (*Response, error) {
	if s.Inner == nil {
		return nil, errors.New("stream wrapper: inner model is nil")
	}

	var resp *Response
	err := s.Inner.CompleteStream(ctx, req, func(sr StreamResult) error {
		if sr.Final && sr.Response != nil {
			resp = sr.Response
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("stream wrapper: no final response received")
	}
	return resp, nil
}

// CompleteStream delegates directly to the inner model.
func (s *StreamOnlyModel) CompleteStream(ctx context.Context, req Request, cb StreamHandler) error {
	if s.Inner == nil {
		return errors.New("stream wrapper: inner model is nil")
	}
	return s.Inner.CompleteStream(ctx, req, cb)
}

// StreamOnlyProvider wraps a Provider so that the Model it returns always
// routes Complete() through CompleteStream(). Use this when the upstream
// API proxy only returns correct tool_use.input in streaming mode.
type StreamOnlyProvider struct {
	Inner Provider
}

// Model implements Provider.
func (p *StreamOnlyProvider) Model(ctx context.Context) (Model, error) {
	if p.Inner == nil {
		return nil, errors.New("stream wrapper: inner provider is nil")
	}
	mdl, err := p.Inner.Model(ctx)
	if err != nil {
		return nil, err
	}
	// Avoid double-wrapping
	if _, ok := mdl.(*StreamOnlyModel); ok {
		return mdl, nil
	}
	return NewStreamOnlyModel(mdl), nil
}
