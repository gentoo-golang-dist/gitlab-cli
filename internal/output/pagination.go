package output

import (
	"iter"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type ListFunc[T any] = func() ([]*T, *gitlab.Response, error)

func Paginate[T any](listFunc ListFunc[T], opts *gitlab.ListOptions, all bool) (iter.Seq2[*T, error], *gitlab.Response, error) {

	items, resp, err := listFunc()
	if err != nil {
		return nil, nil, err
	}

	seq := func(yield func(*T, error) bool) {
		for _, v := range items {
			if !yield(v, nil) {
				return
			}
		}

		for all && resp.NextPage > 0 {
			opts.Page = resp.NextPage
			items, resp, err = listFunc()
			if err != nil {
				yield(nil, err)
				return
			}
			for _, v := range items {
				if !yield(v, nil) {
					return
				}
			}
		}
	}

	return seq, resp, nil
}

func ToSlice[T any](seq iter.Seq2[T, error]) ([]T, error) {
	var out []T
	for item, err := range seq {
		if err != nil {
			return out, err
		}
		out = append(out, item)
	}
	return out, nil
}
