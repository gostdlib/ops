/*
Package bilateral provides a generic Signal type for communication between
two goroutines.

This package provides the boilerplate for signaling between goroutines where
we want to send data from goroutine A to goroutine B and receive data
back in goroutine A. This communication can be blocking or non-blocking.

This packages solves much of the boilerplate code you may be tired of writing.

This package is generic, with the data type sent from A to B represented with
the generic S type (for send).  Data from B to A is represented with the
generic R type (for receive).

This package also is close to zero allocation, with the only allocations
done being channels. You can recycle acknowledgement channels using the
AckSyncPool() option with the Recyle() method.

Example of blocking Signal and Acknowledge pattern:

	// Create a new signaller, which sends a string to a receiver and
	// receives a string back.
	sig := signal.New[string, string]()

	// This goroutine will receive a signal via sig.Receive() and
	// return an acknowledgement to the receiver.
	go func() {
		// Blocks until it receives a signal.
		ack := <-sig.Receive()

		// Acknowledge receipt and return "I'm out!" to the sender.
		defer ack.Ack("I'm out!")

		// This will print "hello everyone" which is sent below.
		fmt.Println(<-ack.Data())
	}()

	// This sends "hello everyone" to our receiver above. We have
	// specified the Wait() option, so this will block until we receive
	// data back.
	retData := sig.Signal(ctx, "hello everyone", signal.Wait[string]())

	// This will print "I'm out", which was sent from above.
	fmt.Println(retData)

Example of a Promise pattern for asynchronous return values:

	// Create a new signaller, which sends a string to a receiver and
	// receives a int back.
	sig := signal.New[string, int]()

	// This goroutine will receive a signal with a string type
	// and count how many words are in the sentence and send
	// the value back.
	go func() {
		ack := sig.Receive()
		s := strings.TrimSpace(ack.Data())
		ack.Ack(len(strings.Split(s))
	}()

	// We make a channel to receive data back from the above goroutine.
	p := make(chan int, 1)

	// Sends a signal with a sentence and the Promise option that will
	// receive the answer. The return value on Signal is dropped(), which is
	// simple the zero value for int(0), because the real value will come on
	// the passed channel.
	sig.Signal(
		ctx,
		"count the number of words in this sentence",
		signal.Promise[string, int](p),
	)

	...
	// Do some other stuff
	...

	fmt.Println(<-p) // Prints "8"

Example of multiple senders to multiple receivers:

	sig := signal.New[int, int]()

	// Setup our receivers
	for i := 0; i < 100; i++ {
		go func() {
			for ack := range sig.Receive() {
				ack.Ack(ack.Data() * 2)
			}
		}()
	}

	wg := sync.WaitGroup{}

	// Send all our data from multiple go routines.
	for i := 0; i < 5; i++ {
		i := i

		p := make(chan int, 1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			sig.Signal(ctx, i, signal.Promise[int, int](p))
			v := <-p
			if v != i * 2 {
				panic("did not receive correct value")
			}
		}()
	}

	wg.Wait()
	sig.Close()
*/
package bilateral

import (
	"context"
	"fmt"
	"sync"
)

// Acker provides the ability to acknowledge a Signal.
type Acker[S, R any] struct {
	data S
	ack  chan R
}

// Data returns any data sent by the sender.
func (a Acker[S, R]) Data() S {
	return a.data
}

// Ack acknowledges a Signal has been received. "x" is any data you wish to return.
func (a Acker[S, R]) Ack(x R) {
	a.ack <- x
}

type signalOptions[S, R any] struct {
	wait    bool
	promise chan R
}

// SignalOption provides an option to Signaler.Signal()
type SignalOption[S, R any] func(s signalOptions[S, R]) signalOptions[S, R]

// Wait indicates that Signal() should block until the Acker has had Ack() called.
func Wait[S, R any]() SignalOption[S, R] {
	return func(s signalOptions[S, R]) signalOptions[S, R] {
		s.wait = true
		return s
	}
}

// Promise can be used to send a signal without waiting for the data to be
// returned, but still get the data at a later point.
// Using Promise() and Wait() will PANIC.
// Passing Promise() a nil pointer will PANIC.
func Promise[S, R any](ch chan R) SignalOption[S, R] {
	return func(s signalOptions[S, R]) signalOptions[S, R] {
		if ch == nil {
			panic("you cannot use a nil channel with Promise()")
		}
		s.promise = ch
		return s
	}
}

// Option is an option for the New() constructor.
type Option[S, R any] func(s Signaler[S, R]) Signaler[S, R]

// BufferSize lets you adjust the internal buffer for how many Signal() calls
// you can make before Signal() blocks waiting someone to call Receive().
func BufferSize[S, R any](n int) Option[S, R] {
	return func(s Signaler[S, R]) Signaler[S, R] {
		s.bufferSize = n
		return s
	}
}

// AckSyncPool provides a sync.Pool that stores the internal Ack channel
// data is returned on when .Recycle(ack) is called.
func AckSyncPool[S, R any]() Option[S, R] {
	return func(s Signaler[S, R]) Signaler[S, R] {
		s.ackPool = &sync.Pool{
			New: func() any {
				return make(chan R, 1)
			},
		}
		return s
	}
}

// Signaler provides an object that can be passed to other goroutines to
// provide for a signal that something has happened.  The receiving goroutine
// can call Receive(), which will block until Signal() is called.
type Signaler[S, R any] struct {
	sendCh     chan Acker[S, R]
	bufferSize int

	ackPool *sync.Pool
}

// New is the constructor for Signal.
func New[S, R any](options ...Option[S, R]) Signaler[S, R] {
	s := Signaler[S, R]{bufferSize: 1}
	for _, o := range options {
		s = o(s)
	}
	s.sendCh = make(chan Acker[S, R], s.bufferSize)
	return s
}

// Signal signals another goroutine that is using .Receive().  This unblocks the
// Receive call on the far side. The return value is data returned by the
// acknowledger. If you pass the Promise() option and the context times out,
// the zero value is returned on the promise.
func (s Signaler[S, R]) Signal(ctx context.Context, x S, options ...SignalOption[S, R]) (R, error) {
	so := signalOptions[S, R]{}
	for _, option := range options {
		so = option(so)
	}

	var rZero R
	if so.promise != nil && so.wait {
		return rZero, fmt.Errorf("Signaler.Signal() cannot be called with both Wait() and Promise()")
	}

	a := Acker[S, R]{
		data: x,
		ack:  make(chan R, 1),
	}
	if s.ackPool == nil {
		a.ack = make(chan R, 1)
	} else {
		a.ack = s.ackPool.Get().(chan R)
	}

	// Send our Acker to the receiver.
	select {
	case <-ctx.Done():
		return rZero, ctx.Err()
	case s.sendCh <- a:
	}

	if so.wait {
		select {
		case <-ctx.Done():
			return rZero, ctx.Err()
		case v := <-a.ack:
			return v, nil
		}
	}

	if so.promise != nil {
		go func() {
			select {
			case <-ctx.Done():
				so.promise <- rZero
			case so.promise <- <-a.ack: // Yes the <- <- is correct.
			}
		}()
	}
	// Return the zero value for the type.
	return rZero, nil
}

// Receive is used by the waiting goroutine to block until Signal() is
// called. The receiver should use the provided Acker.Ack() to inform
// Signal that it can continue (if it is using the Wait() option).
func (s Signaler[S, R]) Receive() <-chan Acker[S, R] {
	return s.sendCh
}

// Close closes all the internal channels for Signaler. This will stop any
// for range loops over the .Receive() channel. This Signaler cannot be used again.
func (s Signaler[S, R]) Close() {
	close(s.sendCh)
}

// Recycle recycles the Acker's internal channel by storing it in a sync.Pool
// for reuse.
func (s Signaler[S, R]) Recycle(ack Acker[S, R]) {
	if s.ackPool == nil {
		return
	}
	s.ackPool.Put(ack.ack)
}
