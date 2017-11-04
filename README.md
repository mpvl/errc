# errc [![GoDoc](https://godoc.org/github.com/mpvl/errc?status.svg)](http://godoc.org/github.com/mpvl/errc) [![Travis-CI](https://travis-ci.org/mpvl/errc.svg)](https://travis-ci.org/mpvl/errc) [![Report card](https://goreportcard.com/badge/github.com/mpvl/errc)](https://goreportcard.com/report/github.com/mpvl/errc) [![codecov](https://codecov.io/gh/mpvl/errc/branch/master/graph/badge.svg)](https://codecov.io/gh/mpvl/errc)


Package `errc` simplifies error and defer handling.

```go get github.com/mpvl/errc```

Also note the sibling package `go get github.com/mpvl/errd`. Package `errc`
probably looks more like a language feature would look like. Package `errd`
however is a bit safer to use as well as a bit faster.



## Overview

Package `errc` is a _burner package_, a proof-of-concept to analyze how to
improve error handling for future iterations of Go. The idiomatic way to
handle errors in Go looks like:

```
    func foo() (err error) {
        r, err := getReader()
        if err != nil {
            return err
        }
        defer r.Close()
        // etc.
```

The implied promise of simplicity of this pattern, though a bit verbose, often
does not hold.
Take the following example:


```go
func writeToGS(ctx context.Context, bucket, dst string, r io.Reader) (err error) {
    client, err := storage.NewClient(ctx)
    if err != nil {
        return err
    }
    defer client.Close()

    w := client.Bucket(bucket).Object(dst).NewWriter(ctx)
    err = errPanicking
    defer func() {
        if err != nil {
            _ = w.CloseWithError(err)
        } else {
            err = w.Close()
        }
    }
    _, err = io.Copy(w, r) {
    return err
}
```

This function atomically writes the contents of an io.Reader to a Google Cloud
Storage file.
It ensures the following:
1. An error resulting from closing `w` is returned if there wasn't any error already
2. In case of a panic, neither Close() nor CloseWithError(nil) is called.

The first condition is necessary to ensure any retry logic will know the file
was not successfully written. The second condition ensures no partial file is
written in case of a panic. A panic may occur, for instance, when the server is
killed by a cluster manager because it uses too much memory.

Using package `errc`, the same is achieved by:

```go
func writeToGS(ctx context.Context, bucket, dst, src string) (err error) {
    e := errc.Catch(&err)
    defer e.Handle()

    client, err := storage.NewClient(ctx)
    e.Must(err)
    e.Defer(client.Close, errc.Discard)

    w := client.Bucket(bucket).Object(dst).NewWriter(ctx)
    e.Defer(w.CloseWithError)

    _, err = io.Copy(w, r)
    e.Must(err)
}
```

In this case, the above guarantees are met by applying the idiomatic
check-and-defer pattern.
The handling of errors around panics, `Must` and `Defer` is such that
applying the check-and-defer pattern yields the correct results without much
further thought.


## Error Handlers

Package `errc` defines a Handler type to allow inline processing of errors.

Suppose we want to use `github.com/pkg/errors` to decorate errors.
A simple handler can be defined as:

```go
type msg string

func (m msg) Handle(s errc.State, err error) error {
    return errors.WithMessage(err, string(m))
}
```

This handler can then be used as follows:

```go
func writeToGS(ctx context.Context, bucket, dst, src string) error {
    e := errc.Catch(&err)
    defer e.Handle()

    client, err := storage.NewClient(ctx)
    e.Must(err, msg("creating client failed"))
    e.Defer(client.Close, errc.Discard)

    w := client.Bucket(bucket).Object(dst).NewWriter(ctx)
    e.Defer(w.CloseWithError)

    _, err = io.Copy(w, r)
    e.Must(err, msg("copy failed"))
    return nil
}
```

It is also possible to pass a default Handler to the Catch function, which will
be applied if no Handler is given at the point of detection.

## Principles

As said, `errc` is a "burner package".
The goal is to improve error handling focussing on semantics first, rather than
considering syntax first.

The main requirements for error handling addressed by `errc` are:
 - errors are and remain values
 - Make it easy to decorate errors with additional information
   (and play nice with packages like `github.com/pkg/errors`).
 - Using an idiomatic way to handling errors should typically result in
   correct behavior.


## Error funnel

The main `errc` concept is that of an error funnel: a single variable associated
with each function in which the current error state is recorded.
It is very much like having a named error return argument in which to record
all errors, but ensuring that the following holds:

- there is a single error variable,
- an error detected by a call to `Must` or `Defer` will only be recorded in
  the error variable if the error variable is `nil`,
- some processing is allowed unconditionally for any error that is detected,
- if a panic occurs, the current error variable will be overwritten by
  a wrapped panic error, and
- it is still possible to override any previous error value by explicitly
  writing to the error variable.

## Errors versus Panics

One could classify error values as recoverable errors while panics are
unrecoverable errors.
In practice things are a bit more subtle. Cleanup code that is called through
defers is still called after a panic.
Although easily ignored, it may be important for such code to consider a panic
as being in an erroring state.
However, by default in Go panic and errors are treated completely separately.
Package `errc` preserves panic semantics, while also treating panics as an
error.
More specifically, a panic will keep unwinding until an explicit recover, while
at the same time it assigns a panic-related error to
the error variable to communicate that the function is currently in an erroring
state.

## How it works

Package `errc` uses Go's `panic` and `recover` mechanism to force the exit from
`Run` if an error is encountered.
On top of that, package `errc` manages its own defer state, which is
necessary to properly interweave error and defer handling.


## Performance

Package `errc` adds a defer block to do all its management.
If the original code only does error checking, this is a relatively
big price to pay.
If the original code already does a defer, the damage is limited.
If the original code uses multiple defers, package `errc` may even be faster.

Passing string-type error handlers, like in the example on error handlers,
causes an allocation.
However, in 1.9 this special case does not incur noticeable overhead over
passing a pre-allocated handler.


## Caveat Emptor

As `errc` uses `defer`, it does not work across goroutine boundaries.
In general, it is advisable not to pass an `errc.Catcher` value as an argument
to any function call.


## What's next
Package `errc` is about exploring better ways and semantics for handling errors
and defers. The main goal here is to come up with a good improvement for Go 2.

