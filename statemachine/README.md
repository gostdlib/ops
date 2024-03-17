# StateMachine - The Functional State Machine for Go

[![GoDoc][godoc image]][godoc] [![Go Report Card](https://goreportcard.com/badge/github.com/gostdlib/ops)](https://goreportcard.com/report/github.com/gostdlib/ops)

<p align="center">
  <img src="../docs/imgs/statemachine.jpeg"  width="500">
</p>

## Introduction

This package provides an implementation of a routable statemachine.

This package is designed with inspiration from Rob Pike's talk on [Lexical Scanning in Go](https://www.youtube.com/watch?v=HxaD_trXwRE).

This package incorporates support for OTEL tracing.

You can read about the advantages of statemachine design for sequential processing at: https://medium.com/@johnsiilver/go-state-machine-patterns-3b667f345b5e

If you are interested in a parallel and concurrent state machine, checkout
the [stagedpipe](https://pkg.go.dev/github.com/gostdlib/concurrency/pipelines/stagedpipe) package.

## Usage

The import path is `github.com/gostdlib/ops/statemachine`.

Simple example:

```go

type data struct {
	Name string
}

func printName(r statemachine.Request[data]) statemachine.Request[data] {
	if r.Data.Name == "" {
		r.Err = errors.New("Name is empty")
		return r // <- This will stop the state machine due to the error
	}

	fmt.Println(r.Data.Name)
	r.Next = writeFile // <- Route to the next state
}

func writeFile(r statemachine.Request[data]) statemachine.Request[data] {
	fileName := r.Data.Name + ".txt"
	r.Event("writeFile", "fileName", fileName)
	err := os.WriteFile(fileName, []byte(r.Data.Name), 0644)
	if err != nil {
		// This will write an error event to the OTEL trace, if present.
		// Retruned Errors are automatically recorded in the OTEL trace.
		r.Event("writeFile", "fileName", fileName, "error", err.Error())
		r.Err = err
		return r
	}
	r.Event("writeFile", "fileName", fileName, "success", true)
	return r // <- This will stop the state machine because we did not set .Next
}

func NameHandler(name string) error {
	return stateMachine.Run(statemachine.Request{
		Data: data{
			Name: name,
		},
		Next: printName,
	})
}
```

This is a simple example of how to use the state machine. The state machine will run the `printName` function and then the `writeFile` function. You could
route to other states based on the data in the request. You can change the
data in the request inside the state functions. This means that you can
use stack allocated data instead of heap allocated data.

Use https://pkg.go.dev/github.com/gostdlib/ops/statemachine to view the documentation.

## Contributing

This package is a part of the gostdlib project. The gostdlib project is a collection of packages that should useful to many Go projects.

Please see guidelines for contributing to the gostdlib project.

[godoc]: https://pkg.go.dev/github.com/gostdlib/ops/statemachine
[godoc image]: https://godoc.org/github.com/gostdlib/ops/statemachine?status.png
