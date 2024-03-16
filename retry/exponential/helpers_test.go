package exponential

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kylelemons/godebug/pretty"
)

// Failures is used as arguments for NewRetryTester to inform us on how our
// fake function should behave.
type Failures struct {
	// errors is a list of errors to return from an Op.
	// If this is set, no other failiures options are used.
	errors []error

	// numFailures is the number of times to return an error from an Op.
	// If < 0, always returns a failure.
	numFailures int
	// failPermanentOn is the failure number to send a permanent error on.
	// If set to 0, it is ignored.
	failPermanent int
}

// RetryData is data that is returned by RetryTester.Run.
type RetryData struct {
	// SuccessOn is the attempt number that the function succeeded on.
	// This is only valid if the function had no error. 0 when not accompanied
	// by an error indicates success on the first attempt.
	SuccessOn int
}

// RetryTester is used inside an Op to test the retry logic.
// Create with NewRetryTester.
type RetryTester struct {
	failures Failures
	data     RetryData

	count int
}

// NewRetryTester creates a new RetryTester. Failures instructs on how the function in
// the Op should behave.
func NewRetryTester(failures Failures) *RetryTester {
	return &RetryTester{
		failures: failures,
	}
}

var zeroRetryData = RetryData{}

func (r *RetryTester) Run(ctx context.Context) (RetryData, error) {
	defer func() { r.count++ }()
	if len(r.failures.errors) > 0 {
		if r.count < len(r.failures.errors) {
			err := r.failures.errors[r.count]
			return zeroRetryData, err
		}
		return RetryData{SuccessOn: r.count + 1}, nil
	}

	if r.failures.numFailures < 0 {
		return zeroRetryData, errors.New("transient error")
	}

	if r.count < r.failures.numFailures {
		if r.count == r.failures.failPermanent && r.failures.failPermanent > 0 {
			return zeroRetryData, ErrPermanent
		}
		return zeroRetryData, errors.New("transient error")
	}
	return RetryData{SuccessOn: r.count + 1}, nil
}

// RecordCheck is the range of the values contained in Record when it is done.
// Because a Retry can have multiple attempts with some amount of jitter, we can't
// check directly against a Record. While we could make the settings have no jitter for
// a direct check, we want to test the jitter as well.
type RecordCheck struct {
	AttemptMin, AttemptMax             int
	LastIntervalMin, LastIntervalMax   time.Duration
	TotalIntervalMin, TotalIntervalMax time.Duration
	Err                                error
}

// NewRecordCheck creates a new RecordCheck given a Policy and the number of attempts.
// If the number of attempts will end in an error, you must manually set the Err field.
func NewRecordCheck(p Policy, attempts int) RecordCheck {
	tt := p.TimeTable(attempts)
	return RecordCheck{
		AttemptMin:       1,
		AttemptMax:       attempts,
		LastIntervalMin:  tt.Entries[attempts-1].MinInterval,
		LastIntervalMax:  tt.Entries[attempts-1].MaxInterval,
		TotalIntervalMin: tt.MinTime,
		TotalIntervalMax: tt.MaxTime,
	}
}

// IsZero returns true if the RecordCheck is the zero value.
func (r RecordCheck) IsZero() bool {
	if r.AttemptMin == 0 {
		return true
	}
	return false
}

// AddErr adds an error to the RecordCheck and returns a new RecordCheck.
func (r RecordCheck) AddErr(err error) RecordCheck {
	r.Err = err
	return r
}

// Check checks if the given Record is within the range of the RecordCheck.
func (r RecordCheck) Check(rec Record) error {
	if rec.Attempt < r.AttemptMin || rec.Attempt > r.AttemptMax {
		return fmt.Errorf("Attempt: got %d, want between %d and %d", rec.Attempt, r.AttemptMin, r.AttemptMax)
	}
	if rec.LastInterval < r.LastIntervalMin || rec.LastInterval > r.LastIntervalMax {
		return fmt.Errorf("LastInterval: got %v, want between %v and %v", rec.LastInterval, r.LastIntervalMin, r.LastIntervalMax)
	}
	if rec.TotalInterval < r.TotalIntervalMin || rec.TotalInterval > r.TotalIntervalMax {
		return fmt.Errorf("TotalInterval: got %v, want between %v and %v", rec.TotalInterval, r.TotalIntervalMin, r.TotalIntervalMax)
	}

	if diff := pretty.Compare(rec.Err, r.Err); diff != "" {
		return fmt.Errorf("Err: -got +want: %v", diff)
	}
	return nil
}
