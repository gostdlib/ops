package exponential

import (
	"context"
	"errors"
)

// Error implements error for this package.
type Error struct {
	err       error
	permanent bool
	rec       Record
	// cancelled is true if a Retry() was cancelled through a context cancel or deadline.
	cancelled bool
}

// Error implements error.Error().
func (e *Error) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

// Record returns the record of the final attempt.
func (e *Error) Record() Record {
	if e == nil {
		return Record{}
	}
	return e.rec
}

// Cancelled returns true if the retry was cancelled either through a deadline or a context cancel.
// This is different than IsCancelled, in that the error contained will be the whatever the last
// error from the Op was. If the Op returned a context.Canceled or context.DeadlineExceeded, then that
// will be detectable with IsCancelled().
func (e *Error) Cancelled() bool {
	if e == nil {
		return false
	}
	return e.cancelled
}

// Unwrap implements errors.Unwrap().
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.err
}

// IsCancelled returns true if the error returned by the Op is a context.Canceled or context.DeadlineExceeded.
// This is different from Cancelled() in that Cancelled() returns true if the retry was cancelled either through
// a deadline or a context cancel.
func (e *Error) IsCancelled() bool {
	if e == nil {
		return false
	}
	if errors.Is(e.err, context.Canceled) {
		return true
	}
	if errors.Is(e.err, context.DeadlineExceeded) {
		return true
	}
	return false
}

// IsPermanent returns true if the error is an *Error or contains a wrapped error of type *Error
// that is marked as a permanent error.
func (e *Error) IsPermanent() bool {
	// This prevents when a *Error == nil from being put
	// in an error interface via IsPermanent, which is never good.
	if e == nil {
		return false
	}
	return IsPermanent(e)
}

// PermanentErr creates a permanent error. This will stop the retries. If err is already a *Error, then
// it will set the permanent flag to true. If not, it will wrap the error in a *Error and set the permanent
func PermanentErr(err error) error {
	if pe, ok := err.(*Error); ok {
		pe.permanent = true
		return pe
	}
	return &Error{err: err, permanent: true}
}

// IsPermanent returns true if the error contains an *Error where IsPermanent() returns true.
func IsPermanent(err error) bool {
	for {
		if err == nil {
			return false
		}

		if e, ok := err.(*Error); ok {
			// If the error is nil, then it's not permanent.
			// This happens on badly written code that stores a nil pointer to a type
			// inside an error interface.
			if e == nil {
				return false
			}
			if e.permanent {
				return true
			}
		}
		err = errors.Unwrap(err)
	}
}
