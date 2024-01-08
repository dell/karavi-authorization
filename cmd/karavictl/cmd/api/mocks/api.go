package mocks

import (
	"context"
	"net/url"
)

// FakeClient is a mock api client
type FakeClient struct {
	GetFn func(ctx context.Context,
		path string,
		headers map[string]string,
		query url.Values,
		resp interface{}) error

	PostFn func(
		ctx context.Context,
		path string,
		headers map[string]string,
		query url.Values,
		body, resp interface{}) error

	PatchFn func(
		ctx context.Context,
		path string,
		headers map[string]string,
		query url.Values,
		body, resp interface{}) error

	DeleteFn func(
		ctx context.Context,
		path string,
		headers map[string]string,
		query url.Values,
		body, resp interface{}) error
}

// Get executes the mock Get request
func (f *FakeClient) Get(ctx context.Context,
	path string,
	headers map[string]string,
	query url.Values,
	resp interface{},
) error {
	if f.GetFn != nil {
		return f.GetFn(ctx, path, headers, query, resp)
	}
	return nil
}

// Post executes the mock Post request
func (f *FakeClient) Post(
	ctx context.Context,
	path string,
	headers map[string]string,
	query url.Values,
	body, resp interface{},
) error {
	if f.PostFn != nil {
		return f.PostFn(ctx, path, headers, query, body, resp)
	}
	return nil
}

// Patch executes the mock Patch request
func (f *FakeClient) Patch(
	ctx context.Context,
	path string,
	headers map[string]string,
	query url.Values,
	body, resp interface{},
) error {
	if f.PatchFn != nil {
		return f.PatchFn(ctx, path, headers, query, body, resp)
	}
	return nil
}

// Delete executes the mock Delete request
func (f *FakeClient) Delete(
	ctx context.Context,
	path string,
	headers map[string]string,
	query url.Values,
	body, resp interface{},
) error {
	if f.DeleteFn != nil {
		return f.DeleteFn(ctx, path, headers, query, body, resp)
	}
	return nil
}
