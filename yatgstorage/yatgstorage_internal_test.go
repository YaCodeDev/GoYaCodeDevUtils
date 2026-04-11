package yatgstorage

import (
	"errors"
	"testing"
)

func TestRecoverableStateWriteError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil",
			err:  nil,
			want: false,
		},
		{
			name: "no such key",
			err:  errors.New("ERR no such key"),
			want: true,
		},
		{
			name: "missing path",
			err:  errors.New("ERR path does not exist"),
			want: true,
		},
		{
			name: "root required",
			err:  errors.New("ERR new objects must be created at the root"),
			want: true,
		},
		{
			name: "wrong type",
			err:  errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"),
			want: true,
		},
		{
			name: "unrelated",
			err:  errors.New("context deadline exceeded"),
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := recoverableStateWriteError(test.err); got != test.want {
				t.Fatalf("recoverableStateWriteError() = %t, want %t", got, test.want)
			}
		})
	}
}
