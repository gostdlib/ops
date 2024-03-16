# Exponential - The Exponential Backoff Package

[![GoDoc][godoc image]][godoc] [![Go Report Card](https://goreportcard.com/badge/github.com/gostdlib/ops)](https://goreportcard.com/report/github.com/gostdlib/ops)

<p align="center">
  <img src="https://raw.githubusercontent.com/gostdlib/ops/main/docs/imgs/backoff.webp" width="500">
</p>

## Introduction

This package provides an implementation of the exponential backoff algorithm.

[Exponential backoff][exponential backoff wiki]
is an algorithm that uses feedback to multiplicatively decrease the rate of some process,
in order to gradually find an acceptable rate.
The retries exponentially increase and stop increasing when a certain threshold is met.

This is a rewrite of an existing package ![github.com/cenkalti/backoff]. The orignal package works as intended. But I found was more difficult to understand and with the inclusions of genercis in the latest version, now has a lot of unnecessary function calls and return values to do similar things.

Like that package, this package has its heritage from [Google's HTTP Client Library for Java][google-http-java-client].

## Usage

The import path is `github.com/gostdlib/ops/retry/exponential`.

This package has a lot of different options, but can be used with the default settings like this:

```go
boff := exponential.New()

// Captured return data.
var data Data

// This sets the maximum time in the operation can be retried to 30 seconds.
// This is based on the parent context, so a cancel on the parent cancels
// this context.
ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)

err := boff.Retry(ctx, func(ctx context.Context, r Record) error {
	var err error
	data, err = getData(ctx)
	return err
})
cancel() // Always cancel the context when done to avoid lingering goroutines.
```

There a many different options for the backoff such as:

- Setting a custom `Policy` for the backoff.
- Logging backoff attempts with the `Record` object.
- Forcing the backoff to stop on permanent errors.
- Influence the backoff with a retry timer set to a specific time.
- Using a Transformer to deal with common errors like gRPC, HTTP, or SQL errors.
- Using the timetable tool to see the results of a custom backoff policy.
- ...

Use https://pkg.go.dev/github.com/gostdlib/ops/retry/exponential to view the documentation.

## Contributing

This package is a part of the gostdlib project. The gostdlib project is a collection of packages that should useful to many Go projects.

Please see guidlines for contributing to the gostdlib project.

[godoc]: https://pkg.go.dev/github.com/gostdlib/ops/retry/exponential
[godoc image]: https://godoc.org/github.com/gostdlib/ops/retry/exponential?status.png
[google-http-java-client]: https://github.com/google/google-http-java-client/blob/da1aa993e90285ec18579f1553339b00e19b3ab5/google-http-client/src/main/java/com/google/api/client/util/ExponentialBackOff.java
[exponential backoff wiki]: http://en.wikipedia.org/wiki/Exponential_backoff
