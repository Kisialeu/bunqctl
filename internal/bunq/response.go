package bunq

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"strings"
)

// pageSize is the maximum count bunq accepts.
const pageSize = 200

// envelope is the common bunq response body. Every Response item is an object
// with a single key, the type name, e.g. {"UserPerson": {...}}.
type envelope struct {
	Response   []json.RawMessage `json:"Response"`
	Pagination *pagination       `json:"Pagination"`
}

type pagination struct {
	OlderURL *string `json:"older_url"`
}

// Item is one Response item with its bunq type name.
type Item struct {
	Type string
	Body json.RawMessage
}

func decodeEnvelope(raw []byte) (*envelope, error) {
	var env envelope
	if len(raw) == 0 {
		return &env, nil
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &env, nil
}

func (e *envelope) items() ([]Item, error) {
	out := make([]Item, 0, len(e.Response))
	for _, raw := range e.Response {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("decode response item: %w", err)
		}
		for typ, body := range m {
			out = append(out, Item{Type: typ, Body: body})
		}
	}
	return out, nil
}

// first decodes the first item of one of the given types into a T. Without
// types it decodes the first item of any type.
func first[T any](e *envelope, types ...string) (T, error) {
	var v T
	items, err := e.items()
	if err != nil {
		return v, err
	}
	if len(types) == 0 && len(items) > 0 {
		if err := json.Unmarshal(items[0].Body, &v); err != nil {
			return v, fmt.Errorf("decode %s: %w", items[0].Type, err)
		}
		return v, nil
	}
	for _, it := range items {
		for _, typ := range types {
			if it.Type == typ {
				if err := json.Unmarshal(it.Body, &v); err != nil {
					return v, fmt.Errorf("decode %s: %w", typ, err)
				}
				return v, nil
			}
		}
	}
	return v, fmt.Errorf("response has no %s", strings.Join(types, " or "))
}

// createdID returns the id of an object created by a POST.
func createdID(e *envelope) (int, error) {
	id, err := first[struct {
		ID int `json:"id"`
	}](e, "Id")
	return id.ID, err
}

// get calls GET path and decodes the first item of one of types.
func get[T any](ctx context.Context, c *Client, path string, types ...string) (T, error) {
	env, err := c.do(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		var zero T
		return zero, err
	}
	return first[T](env, types...)
}

// create calls POST path with body and returns the created id.
func create(ctx context.Context, c *Client, path string, body any) (int, error) {
	env, err := c.do(ctx, http.MethodPost, path, nil, body)
	if err != nil {
		return 0, err
	}
	return createdID(env)
}

// update calls PUT path with body.
func update(ctx context.Context, c *Client, path string, body any) error {
	_, err := c.do(ctx, http.MethodPut, path, nil, body)
	return err
}

// remove calls DELETE path.
func remove(ctx context.Context, c *Client, path string) error {
	_, err := c.do(ctx, http.MethodDelete, path, nil, nil)
	return err
}

// download calls GET path and returns the raw, signature-verified body.
func download(ctx context.Context, c *Client, path string) ([]byte, error) {
	return c.roundTrip(ctx, request{method: http.MethodGet, path: path})
}

// upload POSTs a raw body with its content type and extra headers and
// returns the created id.
func upload(ctx context.Context, c *Client, path, contentType string, header map[string]string, body []byte) (int, error) {
	raw, err := c.roundTrip(ctx, request{method: http.MethodPost, path: path, body: body, contentType: contentType, header: header})
	if err != nil {
		return 0, err
	}
	env, err := decodeEnvelope(raw)
	if err != nil {
		return 0, err
	}
	return createdID(env)
}

// list iterates all items of a listing, newest first, following older_url.
// extra adds query parameters to the first page. Stopping the iteration stops
// paging.
func (c *Client) list(ctx context.Context, path string, extra url.Values) iter.Seq2[Item, error] {
	return func(yield func(Item, error) bool) {
		query := url.Values{"count": {fmt.Sprint(pageSize)}}
		for k, v := range extra {
			query[k] = v
		}
		for path != "" {
			env, err := c.do(ctx, http.MethodGet, path, query, nil)
			if err != nil {
				yield(Item{}, err)
				return
			}
			items, err := env.items()
			if err != nil {
				yield(Item{}, err)
				return
			}
			for _, it := range items {
				if !yield(it, nil) {
					return
				}
			}

			path, query = "", nil
			if env.Pagination != nil && env.Pagination.OlderURL != nil {
				u, err := url.Parse(*env.Pagination.OlderURL)
				if err != nil {
					yield(Item{}, fmt.Errorf("parse older_url: %w", err))
					return
				}
				path, query = u.Path, u.Query()
			}
		}
	}
}

// typeSetter is implemented by types that keep the bunq type name of an item,
// e.g. accounts, where one listing mixes several types.
type typeSetter interface{ setType(string) }

// listAs iterates a listing and decodes every item into a T. query may be nil.
func listAs[T any](ctx context.Context, c *Client, path string, query url.Values) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for it, err := range c.list(ctx, path, query) {
			var v T
			if err == nil {
				err = json.Unmarshal(it.Body, &v)
			}
			if ts, ok := any(&v).(typeSetter); ok {
				ts.setType(it.Type)
			}
			if !yield(v, err) || err != nil {
				return
			}
		}
	}
}

// apiPath joins parts into an API path, e.g. apiPath("user", 1) is "/v1/user/1".
func apiPath(parts ...any) string {
	var b strings.Builder
	b.WriteString("/v1")
	for _, p := range parts {
		b.WriteString("/")
		b.WriteString(url.PathEscape(fmt.Sprint(p)))
	}
	return b.String()
}
