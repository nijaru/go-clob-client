package clob

import "iter"

// Collect drains seq into a slice, returning the first error encountered.
// It is a convenience wrapper for iterators returned by Iter* methods.
func Collect[T any](seq iter.Seq2[T, error]) ([]T, error) {
	var out []T
	for v, err := range seq {
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
