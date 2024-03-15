package exponential

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kylelemons/godebug/pretty"
)

// TestError tests the Error struct and its methods.
func TestError(t *testing.T) {
	t.Parallel()

	testErr := errors.New("test error")

	tests := []struct {
		name        string
		err         *Error
		Error       string
		Record      Record
		Canceled    bool
		Unwrap      error
		IsCancelled bool
		IsPermanent bool
	}{
		{
			name:        "Error is nil",
			err:         nil,
			Error:       "",
			Record:      Record{},
			Canceled:    false,
			Unwrap:      nil,
			IsCancelled: false,
			IsPermanent: false,
		},
		{
			name:        "Error is not nil, but wrapped error is nil",
			err:         &Error{},
			Error:       "",
			Record:      Record{},
			Canceled:    false,
			Unwrap:      nil,
			IsCancelled: false,
			IsPermanent: false,
		},
		{
			name: "Error with all fields set",
			err: &Error{
				err: testErr,
				rec: Record{
					Attempt:       1,
					LastInterval:  1 * time.Second,
					TotalInterval: 1 * time.Second,
					Err:           testErr,
				},
				cancelled: true,
				permanent: true,
			},
			Error: testErr.Error(),
			Record: Record{
				Attempt:       1,
				LastInterval:  1 * time.Second,
				TotalInterval: 1 * time.Second,
				Err:           testErr,
			},
			Canceled:    true,
			Unwrap:      testErr,
			IsPermanent: true,
		},
		{
			name:        "Error IsCancelled is true because error is context.Canceled",
			err:         &Error{err: context.Canceled},
			Error:       context.Canceled.Error(),
			IsCancelled: true,
			Unwrap:      context.Canceled,
		},
		{
			name:        "Error IsCancelled is true because error is context.DeadlineExceeded",
			err:         &Error{err: context.DeadlineExceeded},
			Error:       context.DeadlineExceeded.Error(),
			IsCancelled: true,
			Unwrap:      context.DeadlineExceeded,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if test.err.Error() != test.Error {
				t.Errorf("Error(): got %v, want %v", test.err.Error(), test.Error)
			}
			if diff := pretty.Compare(test.err.Record(), test.Record); diff != "" {
				t.Errorf("Record(): -got +want: %v", diff)
			}
			if test.err.Cancelled() != test.Canceled {
				t.Errorf("Cancelled(): got %v, want %v", test.err.Cancelled(), test.Canceled)
			}
			if test.err.Unwrap() == nil {
				if test.Unwrap != nil {
					t.Errorf("Unwrap(): got %v, want %v", test.err.Unwrap(), test.Unwrap)
				}
			} else {
				if !errors.Is(test.err.Unwrap(), test.Unwrap) {
					t.Errorf("Unwrap(): got %v, want %v", test.err.Unwrap(), test.Unwrap)
				}
			}

			if test.err.IsCancelled() != test.IsCancelled {
				t.Errorf("IsCancelled(): got %v, want %v", test.err.IsCancelled(), test.IsCancelled)
			}
			if test.err.IsPermanent() != test.IsPermanent {
				t.Errorf("IsPermanent(): got %v, want %v", test.err.IsPermanent(), test.IsPermanent)
			}
		})
	}
}
