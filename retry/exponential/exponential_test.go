package exponential

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/kylelemons/godebug/pretty"
)

var (
	zeroTime      = time.Time{}
	_10millTime   = time.Time{}.Add(10 * time.Millisecond)
	_1secondTime  = time.Time{}.Add(1 * time.Second)
	_2secondsTime = time.Time{}.Add(2 * time.Second)
)

// testClock provides a clock implementation for testing.
// Use moveTime to move the clock forward by a duration.
// This can be used by &testClock{} with no parameters, which starts at the zero time.
type testClock struct {
	now time.Time
	mu  sync.Mutex

	timers []*timer

	// onTimer fires after a new timer is created.
	onTimer func(t *testClock, d time.Duration)
}

// moveTime moves the clock forward by d, firing any timers that are due.
func (c *testClock) moveTime(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = c.now.Add(d)

	keep := []*timer{}
	for _, t := range c.timers {
		if t.stopped {
			continue
		}
		if t.when.Compare(c.now) <= 0 {
			t.c <- t.when
			continue
		}
		keep = append(keep, t)
	}
	c.timers = keep
}

// Now returns the current time as set by the internal time.
func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now
}

// Until returns the duration until the given time based on the internal time.
func (c *testClock) Until(t time.Time) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return t.Sub(c.now)
}

// NewTimer creates a new timer that will fire after d. This is based on the internal time.
func (c *testClock) NewTimer(d time.Duration) *timer {
	defer func() {
		if c.onTimer != nil {
			c.onTimer(c, d)
		}
	}()

	c.mu.Lock()
	defer c.mu.Unlock()

	ch := make(chan time.Time, 1)

	t := &timer{
		C:    ch,
		c:    ch,
		when: c.now.Add(d),
	}
	c.timers = append(c.timers, t)
	return t
}

// TestPolicyValidate tests that the Policy.validate() correctly validates the struct.
func TestPolicyValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		policy Policy
		want   error
	}{
		{
			name: "valid policy",
			policy: Policy{
				InitialInterval:     100 * time.Millisecond,
				Multiplier:          2.0,
				RandomizationFactor: 0.5,
				MaxInterval:         60 * time.Second,
			},
			want: nil,
		},
		{
			name: "Err: initial interval zero",
			policy: Policy{
				InitialInterval:     0,
				Multiplier:          2.0,
				RandomizationFactor: 0.5,
				MaxInterval:         60 * time.Second,
			},
			want: errors.New("Policy.InitialInterval must be greater than 0"),
		},
		{
			name: "Err: multiplier not greater than 1",
			policy: Policy{
				InitialInterval:     100 * time.Millisecond,
				Multiplier:          1.0,
				RandomizationFactor: 0.5,
				MaxInterval:         60 * time.Second,
			},
			want: errors.New("Policy.Multiplier must be greater than 1"),
		},
		{
			name: "Err: randomization factor out of range",
			policy: Policy{
				InitialInterval:     100 * time.Millisecond,
				Multiplier:          2.0,
				RandomizationFactor: 1.1,
				MaxInterval:         60 * time.Second,
			},
			want: errors.New("Policy.RandomizationFactor must be between 0 and 1"),
		},
		{
			name: "Err: max interval zero",
			policy: Policy{
				InitialInterval:     100 * time.Millisecond,
				Multiplier:          2.0,
				RandomizationFactor: 0.5,
				MaxInterval:         0,
			},
			want: errors.New("Policy.MaxInterval must be greater than 0"),
		},
		{
			name: "Err: initial interval greater than max interval",
			policy: Policy{
				InitialInterval:     2 * time.Minute,
				Multiplier:          2.0,
				RandomizationFactor: 0.5,
				MaxInterval:         1 * time.Minute,
			},
			want: errors.New("Policy.InitialInterval must be less than or equal to Policy.MaxInterval"),
		},
		{
			name:   "Default policy must be valid",
			policy: defaults(),
			want:   nil,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := test.policy.validate()
			if diff := pretty.Compare(got, test.want); diff != "" {
				t.Errorf("Validate(): -got +want: %v", diff)
			}
		})
	}
}

func TestPolicyTimetable(t *testing.T) {
	t.Parallel()

	// Make a representation of our default timetable if we limit to success on attempt 1.
	zerott := copyDefaultTimeTable()
	zerott.MaxTime = 0
	zerott.MinTime = 0
	zerott.Entries = zerott.Entries[:1]

	// Make a representation of our default timetable if we limit to success on attempt 3.
	_3tt := copyDefaultTimeTable()
	_3tt.Entries = zerott.Entries[:3]
	_3tt.MinTime = 0
	_3tt.MaxTime = 0
	for _, e := range _3tt.Entries {
		_3tt.MinTime += e.MinInterval
		_3tt.MaxTime += e.MaxInterval
	}

	tests := []struct {
		name    string
		policy  Policy
		attempt int
		want    TimeTable
	}{
		{
			name:    "Attempt -1: All attempts until we hit max interval",
			policy:  defaults(),
			attempt: -1,
			want:    defaultTimeTable, // in file default_timetable_test.go
		},
		{
			name:    "Attempt 0: Only our first attempt",
			policy:  defaults(),
			attempt: 0,
			want:    zerott,
		},
		{
			name:    "Attempt 3: Only our first 3 attempts",
			policy:  defaults(),
			attempt: 3,
			want:    _3tt,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := test.policy.TimeTable(test.attempt)

			if diff := pretty.Compare(got, test.want); diff != "" {
				t.Errorf("timetable(): -got +want: %v", diff)
			}
		})
	}
}

// TestDefaults tests that we get the expected default values for the Policy struct.
func TestDefaults(t *testing.T) {
	t.Parallel()

	want := Policy{
		InitialInterval:     100 * time.Millisecond,
		Multiplier:          2.0,
		RandomizationFactor: 0.5,
		MaxInterval:         60 * time.Second,
	}

	conf := pretty.Config{
		PrintStringers: true,
	}

	got := defaults()

	if diff := conf.Compare(got, want); diff != "" {
		t.Errorf("defaults(): -got +want: %v", diff)
	}
}

func TestWithOptions(t *testing.T) {
	nonDefaultPolicy := Policy{
		InitialInterval:     100 * time.Second,
		Multiplier:          2.0,
		RandomizationFactor: 0.5,
		MaxInterval:         200 * time.Second,
	}

	tests := []struct {
		name   string
		option func() Option
		tester func(*Backoff) error
	}{
		{
			name: "WithPolicy",
			option: func() Option {
				return WithPolicy(nonDefaultPolicy)
			},
			tester: func(b *Backoff) error {
				if b.policy.InitialInterval != 100*time.Second {
					return fmt.Errorf("WithPolicy() option does not work")
				}
				return nil
			},
		},
		{
			name:   "WithTesting",
			option: func() Option { return WithTesting() },
			tester: func(b *Backoff) error {
				if !b.useTest {
					return fmt.Errorf("WithTesting() option does not work")
				}
				return nil
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			b, err := New(test.option())
			if err != nil {
				panic(err)
			}
			if err := test.tester(b); err != nil {
				t.Errorf("%s: %v", test.name, err)
			}
		})
	}

}

// TestRetry tests the Retry method and New function. It is the overall test for the package with
// other tests for all methods and functions that are used by Retry. This tests all options to make
// sure they are used while other tests focus on all possibilities within individual options.
// NOTE(jdoak): I probably could have made this easier on myself by taking the contents of
// the Retry() for loop into a sub-function and testing that sub-fuction, and using an attribute
// to store the random intervals. However, I needed the ability to predict random intervals anyways,
// so... I guess it's a wash.
// NOTE(jdoak): This test and all other have been tested in a loop for flakiness.
func TestRetry(t *testing.T) {
	nonPermErr := errors.New("transient error")
	perm := PermanentErr(errors.New("permanent error"))

	tests := []struct {
		// name is the name of the test.
		name string
		// failures is the Failures to use in the RetryTester.
		failures Failures
		// dataWant is the expected data returned by the function in the Op.
		dataWant RetryData
		// options are Option(s) to use in the New constructor for Backoff.
		options []Option
		// ctx is the context to use for the test. If nil, context.Background() is used.
		ctx context.Context
		// cancelCtx is the duration to cancel the context after.
		cancelCtx time.Duration
		// clock is the clock to use for the test. If nil, the normal time package is used.
		clock clock
		// newErr is true if New() should return an error.
		newErr bool
		// retryErr inidicates if the Retry() function ends with an error.
		retryErr bool
		// retryErrPermanent indicates if the error returned by Retry() is permanent.
		retryErrPermanent bool
		// retryErrCancelled indicates if the error returned by Retry() is context.Canceled.
		retryErrCancelled bool
		// retryIsCancelled indicates if the last error returned by the function in the Op
		// is context.Canceled/context.DeadlineExceeded. This is different than the Retry()
		// loop ending with a context.Canceled error. See Error for more details.
		retryIsCancelled bool
		// recCheck is the expected range of record when completed.
		recCheck RecordCheck
		// wantClockMin is the minimum time we want the testClock to be at when the function is done.
		// wantClockMax is the maximum time we want the testClock at when the function is done.
		wantClockMin, wantClockMax time.Time
	}{
		{
			name: "New with invalid policy",
			options: []Option{
				WithPolicy(Policy{InitialInterval: -1 * time.Second}),
			},
			newErr: true,
		},
		{
			name:     "Success on first attempt",
			failures: Failures{},
			dataWant: RetryData{SuccessOn: 1},
			recCheck: NewRecordCheck(defaults(), 1),
		},
		{
			name:              "Permanent error on first attempt",
			failures:          Failures{errors: []error{perm}},
			dataWant:          RetryData{},
			retryErr:          true,
			retryErrPermanent: true,
			recCheck:          NewRecordCheck(defaults(), 1).AddErr(perm),
		},
		{
			name:              "Permanent error on second attempt",
			failures:          Failures{errors: []error{nonPermErr, perm}},
			dataWant:          RetryData{},
			retryErr:          true,
			retryErrPermanent: true,
			recCheck:          NewRecordCheck(defaults(), 2).AddErr(perm),
		},
		{
			name: "Context deadlines after 1 second",
			ctx: &fakeContext{
				deadline: time.Time{}.Add(1 * time.Second),
				err:      context.DeadlineExceeded,
			},
			failures: Failures{
				numFailures: -1, // Continue failing until the context times out.
			},
			dataWant:          RetryData{},
			retryErr:          true,
			retryErrCancelled: true,
			clock: &testClock{
				onTimer: func(t *testClock, d time.Duration) {
					t.moveTime(d)
				},
			},
			wantClockMin: time.Time{}.Add(400 * time.Millisecond),
			wantClockMax: time.Time{}.Add(time.Duration(4.8 * float64(time.Second))),
		},
		{
			name:      "Context is cancelled (manually) after 1 second",
			cancelCtx: 1 * time.Second,
			failures: Failures{
				numFailures: -1, // Continue failing until the context times out.
			},
			dataWant:          RetryData{},
			retryErr:          true,
			retryErrCancelled: true,
			clock: &testClock{
				onTimer: func(t *testClock, d time.Duration) {
					t.moveTime(d)
				},
			},
			wantClockMin: time.Time{}.Add(400 * time.Millisecond),
			wantClockMax: time.Time{}.Add(time.Duration(4.8 * float64(time.Second))),
		},
		{
			name: "Retry doesn't exceed MaxInterval * RandomizationFactor",
			failures: Failures{
				numFailures: 11, // Continue failing until the max interval is reached.
			},
			dataWant: RetryData{SuccessOn: 12},
			clock: &testClock{
				onTimer: func(t *testClock, d time.Duration) {
					t.moveTime(d)
				},
			},
			wantClockMin: time.Time{}.Add(1 * time.Minute).Add(21 * time.Second).Add(15000 * time.Millisecond),
			wantClockMax: time.Time{}.Add(4 * time.Minute).Add(3 * time.Second).Add(45000 * time.Millisecond),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			b, err := New(test.options...)
			if err != nil {
				if !test.newErr {
					t.Errorf("unexpected New() error: %v", err)
				}
				return
			}
			b.clock = test.clock

			f := NewRetryTester(test.failures)

			d := RetryData{}
			finalRec := Record{}

			var ctx = test.ctx
			var cancel context.CancelFunc

			if ctx == nil {
				ctx = context.Background()
			} else if test.clock != nil {
				fc := ctx.(*fakeContext)
				fc.clock = test.clock.(*testClock)
			}
			if test.cancelCtx > 0 {
				if _, ok := ctx.(*fakeContext); ok {
					panic("cannot use fakeContext with cancelCtx")
				}
				ctx, cancel = context.WithCancel(ctx)
				go func() {
					time.Sleep(test.cancelCtx)
					cancel()
				}()
			}

			err = b.Retry(ctx, func(ctx context.Context, r Record) error {
				finalRec = r

				var err error
				d, err = f.Run(ctx)
				return err
			})

			switch {
			case err == nil && test.retryErr:
				t.Errorf("got err == nil, want err != nil")
				return
			case err != nil && !test.retryErr:
				t.Errorf("got err == %s, want err == nil", err)
				return
			case err != nil:
				e := isError(err)
				if e == nil {
					t.Errorf("Retry() returned non-nil error, but IsError(err) == nil, BUG!!!!")
				}
				if e.Cancelled() != test.retryErrCancelled {
					t.Errorf("Retry() returned Error.Cancelled() == %v, want %v", e.Cancelled(), test.retryErrCancelled)
				}
				if e.IsPermanent() != test.retryErrPermanent {
					t.Errorf("Retry() returned Error.IsPermanent() == %v, want %v", e.IsPermanent(), test.retryErrPermanent)
				}
				if e.IsCancelled() != test.retryIsCancelled {
					t.Errorf("Retry() returned Error.IsCancelled() == %v, want %v", e.IsCancelled(), test.retryIsCancelled)
				}
				return
			}

			if diff := pretty.Compare(d, test.dataWant); diff != "" {
				t.Errorf("Retry(): -got +want: %v", diff)
			}
			if !test.recCheck.IsZero() {
				if err := test.recCheck.Check(finalRec); err != nil {
					t.Errorf("Retry(): final record check had error: %v", err)
				}
			}
			if test.clock != nil {
				got := test.clock.Now()
				if got.Before(test.wantClockMin) || got.After(test.wantClockMax) {
					t.Errorf("Retry(): got clock time %v, want between %v and %v", got, test.wantClockMin, test.wantClockMax)
				}
			}
		})
	}
}

func TestRandomize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		randomizationFactor float64
		interval            time.Duration
		minValue            time.Duration
		maxValue            time.Duration
	}{
		{
			name:                "Randomize 0",
			randomizationFactor: 0,
			interval:            1 * time.Second,
			minValue:            1 * time.Second,
			maxValue:            1 * time.Second,
		},
		{
			name:                "Randomize 0.5",
			randomizationFactor: 0.5,
			interval:            1 * time.Second,
			minValue:            500 * time.Millisecond,
			maxValue:            1500 * time.Millisecond,
		},
		{
			name:                "Randomize 1",
			randomizationFactor: 1,
			interval:            1 * time.Second,
			minValue:            0,
			maxValue:            2 * time.Second,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			b := &Backoff{policy: defaults()}
			b.policy.RandomizationFactor = test.randomizationFactor
			got := b.randomize(test.interval)
			if got < test.minValue || got > test.maxValue {
				t.Errorf("randomize(): got %v, want between %v and %v", got, test.minValue, test.maxValue)
			}
		})
	}
}

// TestEnsureRandomization tests that the randomization factor is actually giving us a random value and
// not the same value every time.
func TestEnsureRandomization(t *testing.T) {
	t.Parallel()

	seen := map[time.Duration]bool{}

	b := &Backoff{policy: defaults()}
	for i := 0; i < 100; i++ {
		got := b.randomize(1 * time.Second)
		if seen[got] {
			continue
		}
		seen[got] = true
	}
	if len(seen) < 50 {
		t.Errorf("randomize(): got %v unique values, want at least 50", len(seen))
	}
}

type fakeContext struct {
	context.Context
	done     chan struct{}
	clock    *testClock
	deadline time.Time
	err      error
}

func (c *fakeContext) WithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	c.deadline = c.clock.Now().Add(timeout)
	return c, func() { c.err = context.Canceled }
}

func (c *fakeContext) Deadline() (time.Time, bool) {
	return c.deadline, true
}

func (c *fakeContext) Err() error {
	return c.err
}

func TestCtxOK(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      func(clock *testClock) context.Context
		interval time.Duration
		want     bool
	}{
		{
			name:     "Context with no error and no deadline",
			ctx:      func(clock *testClock) context.Context { return context.Background() },
			interval: time.Second,
			want:     true,
		},
		{
			name: "Context with no error and deadline after interval",
			ctx: func(clock *testClock) context.Context {
				return &fakeContext{clock: clock, deadline: clock.Now().Add(2 * time.Second)}
			},
			interval: time.Second,
			want:     true,
		},

		{
			name: "Context with no error and deadline before interval",
			ctx: func(clock *testClock) context.Context {
				return &fakeContext{clock: clock, deadline: clock.Now().Add(time.Second)}
			},
			interval: 2 * time.Second,
			want:     false,
		},
		{
			name: "Context with error",
			ctx: func(clock *testClock) context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel the context immediately
				return ctx
			},
			interval: time.Second,
			want:     false,
		},
		{
			name: "Context with deadline just enough for interval",
			ctx: func(clock *testClock) context.Context {
				// 1 second + 1 nanosecond
				return &fakeContext{clock: clock, deadline: clock.Now().Add(time.Second).Add(1)}
			},
			interval: time.Second,
			want:     true,
		},
		{
			name: "Context with canceled deadline",
			ctx: func(clock *testClock) context.Context {
				fakeCtx := &fakeContext{clock: clock, deadline: clock.Now().Add(2 * time.Second)}
				fakeCtx.err = context.Canceled
				return fakeCtx
			},
			interval: time.Second,
			want:     false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			clock := &testClock{}
			b := &Backoff{clock: clock}
			if got := b.ctxOK(test.ctx(clock), test.interval); got != test.want {
				t.Errorf("got %t, want %t", got, test.want)
			}
		})
	}
}

func TestBackoffIsPermanent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		err           error
		want          bool
	}{
		{
			name: "Permanent error through PermanentErr",
			err:  PermanentErr(errors.New("permanent failure")),
			want: true,
		},
		{
			name: "Standard error",
			err:  &Error{err: errors.New("transient failure"), permanent: false},
			want: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			b := &Backoff{}
			if got := b.isPermanent(test.err); got != test.want {
				t.Errorf("isPermanent() = %v, want %v", got, test.want)
			}
		})
	}
}

// isError returns the *Error if the error is a *Error. If not, it returns nil.
// All calls to Retry() will return an *Error. But in case of some bug, this
// should always be used and checked for nil.
func isError(err error) *Error {
	if e, ok := err.(*Error); ok {
		return e
	}
	return nil
}
