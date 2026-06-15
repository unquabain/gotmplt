package main

import (
	"fmt"
	"strconv"
	"strings"
)

// dataRef is a mutable reference into a nested data structure. parent is the
// containing map[string]any or []any; key indexes into it (string key for maps,
// decimal integer for slices).
type dataRef struct {
	parent any
	key    string
}

func (r *dataRef) Get() (any, bool) {
	switch p := r.parent.(type) {
	case map[string]any:
		v, ok := p[r.key]
		return v, ok
	case []any:
		idx, err := strconv.Atoi(r.key)
		if err != nil || idx < 0 || idx >= len(p) {
			return nil, false
		}
		return p[idx], true
	}
	return nil, false
}

func (r *dataRef) Set(v any) error {
	switch p := r.parent.(type) {
	case map[string]any:
		p[r.key] = v
		return nil
	case []any:
		idx, err := strconv.Atoi(r.key)
		if err != nil || idx < 0 || idx >= len(p) {
			return fmt.Errorf("invalid slice index %q", r.key)
		}
		p[idx] = v
		return nil
	}
	return fmt.Errorf("cannot set into %T", r.parent)
}

// dig walks data along a dot-separated path. On success, the returned ref points
// at the value (ref.Get / ref.Set reach it) and remainder is empty. On failure,
// ref.parent is the deepest container reached and remainder is the unresolved
// suffix; if descent was blocked by a scalar, ref.parent is that scalar.
func dig(data any, path string) (*dataRef, bool, string) {
	if path == "" {
		return &dataRef{parent: data}, false, ""
	}
	parts := strings.Split(path, ".")
	var current any = data
	for i, part := range parts {
		switch curr := current.(type) {
		case map[string]any:
			next, ok := curr[part]
			if !ok {
				return &dataRef{parent: curr, key: part}, false, strings.Join(parts[i:], ".")
			}
			if i == len(parts)-1 {
				return &dataRef{parent: curr, key: part}, true, ""
			}
			current = next
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(curr) {
				return &dataRef{parent: curr, key: part}, false, strings.Join(parts[i:], ".")
			}
			if i == len(parts)-1 {
				return &dataRef{parent: curr, key: part}, true, ""
			}
			current = curr[idx]
		default:
			return &dataRef{parent: current}, false, strings.Join(parts[i:], ".")
		}
	}
	return &dataRef{parent: data}, false, path
}

// digSet sets a value in a nested data structure according to a dot-separated path.
// Intermediate maps are created as needed. Slice indices must already exist.
func digSet(data any, path string, value any) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	ref, found, remainder := dig(data, path)
	if found {
		return ref.Set(value)
	}
	switch ref.parent.(type) {
	case map[string]any, []any:
	default:
		return fmt.Errorf("cannot set %s: parent is %T, not a container", path, ref.parent)
	}
	parts := strings.Split(remainder, ".")
	parent := ref.parent
	for _, part := range parts[:len(parts)-1] {
		next := make(map[string]any)
		if err := (&dataRef{parent: parent, key: part}).Set(next); err != nil {
			return err
		}
		parent = next
	}
	return (&dataRef{parent: parent, key: parts[len(parts)-1]}).Set(value)
}
