package bilateral

import (
	"context"
	"sort"
	"sync"
	"testing"

	"github.com/kylelemons/godebug/pretty"
)

func TestBufferSize(t *testing.T) {
	ctx := context.Background()

	sig := New(BufferSize[int, int](5))
	for i := 0; i < 5; i++ {
		sig.Signal(ctx, i)
	}

	for i := 0; i < 5; i++ {
		ack := <-sig.Receive()
		if ack.Data() != i {
			t.Errorf("TestBufferSize: loop %d: got %d, want %d", i, ack.Data(), i)
		}
		ack.Ack(0)
	}
}

func TestPromise(t *testing.T) {
	ctx := context.Background()
	sig := New[int, int]()
	promises := make([]chan int, 0, 100)
	want := make([]int, 0, 100)

	for i := 0; i < 100; i++ {
		i := i
		go func() {
			ack := <-sig.Receive()
			defer ack.Ack(i)
		}()
		p := make(chan int, 1)
		promises = append(promises, p)
		want = append(want, i)
	}

	for i := 0; i < 100; i++ {
		sig.Signal(ctx, 0, Promise[int, int](promises[i]))
	}

	got := make([]int, 0, 100)
	for _, p := range promises {
		i := <-p
		got = append(got, i)
	}

	sort.Ints(got)
	sort.Ints(want)
	if diff := pretty.Compare(want, got); diff != "" {
		t.Errorf("TestPromise: -want/+got:\n%s", diff)
	}
}

func TestClose(t *testing.T) {
	ctx := context.Background()
	sig := New(BufferSize[string, string](3))
	wg := sync.WaitGroup{}
	wg.Add(1)
	got := []string{}
	go func() {
		defer wg.Done()
		for v := range sig.Receive() {
			v.Ack("")
			got = append(got, v.Data())
		}
	}()

	sig.Signal(ctx, "Hello")
	sig.Signal(ctx, "World")
	sig.Close()
	wg.Wait()

	if diff := pretty.Compare([]string{"Hello", "World"}, got); diff != "" {
		t.Errorf("TestClose: -want/+got:\n%s", diff)
	}
}

func TestSignalWait(t *testing.T) {
	ctx := context.Background()
	const hello = "hello everyone"
	const out = "I'm out!"

	var goReturn string

	sig := New[string, string]()

	go func(sig Signaler[string, string]) {
		ack := <-sig.Receive()
		defer ack.Ack(out)

		goReturn = ack.Data()
	}(sig)

	ackReturn, err := sig.Signal(ctx, hello, Wait[string, string]())
	if err != nil {
		panic(err)
	}

	if goReturn != hello {
		t.Errorf("TestSignalWait: got goReturn == %s, want %s", goReturn, hello)
	}
	if ackReturn != out {
		t.Errorf("TestSignalWait: got ackReturn == %s, want %s", ackReturn, out)
	}
}

func TestMultipleSendersAndRecievers(t *testing.T) {
	sig := New[int, int]()
	receivers := sync.WaitGroup{}

	// Setup our receivers
	for i := 0; i < 100; i++ {
		receivers.Add(1)
		go func() {
			defer receivers.Done()
			for ack := range sig.Receive() {
				ack.Ack(ack.Data() * 2)
			}
		}()
	}

	wg := sync.WaitGroup{}

	// Send all our data from multiple go routines.
	for i := 0; i < 50; i++ {
		i := i

		p := make(chan int, 1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			sig.Signal(ctx, i, Promise[int, int](p))
			v := <-p
			if v != i*2 {
				panic("did not receive correct value")
			}
		}()
	}

	wg.Wait()
	sig.Close()
	receivers.Wait()
}
