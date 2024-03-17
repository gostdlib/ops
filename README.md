# Ops

Packages for dealing with various operational issues, such as retrying function calls.

[![Go Reference](https://pkg.go.dev/badge/github.com/gostdlib/ops/ops.svg)](https://pkg.go.dev/github.com/gostdlib/ops/)
[![Go Report Card](https://goreportcard.com/badge/github.com/gostdlib/ops)](https://goreportcard.com/report/github.com/gostdlib/ops)

<p align="center">
  <img src="./docs/imgs/ops.jpeg"  width="500">
</p>

# Introduction

The packages contained within this repository provide help around execution of operations. Operations here are defined as complex calls that usually involve remote systems.

Complex operations which involve remote systems can fail for different reasons and packages here help with dealing with the nature of those operations.

# A quick look

- `retry/` : A set of packages for retrying operations
  - Use [`retry/exponential`](https://pkg.go.dev/github.com/gostdlib/ops/retry/exponential) if you want:
    - Exponential retry of some operation
    - The ability to customize your own retry policy
    - The ability to visualize your retry policy
    - The ability to transform errors before retrying (like automatic handling of gRPC, HTTP, or SQL errors)
    - The ability to log retry attempts
    - The ability to stop retrying on permanent errors
    - The ability to influence the backoff with a retry timer set to a specific time
- `statemachine/` : A set of packages for creating functional state machines
  - Use [`statemachine`](https://pkg.go.dev/github.com/gostdlib/ops/statemachine) if you want:
    - A simple state machine
    - A state machine with OTEL tracing
    - Low allocations
    - A way to simplify complex sequential processing
    - Easier ways to test than a sequential call chain
