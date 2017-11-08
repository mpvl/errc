// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errc_test

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mpvl/errc"
)

func ExampleHandler_fatal() {
	func() (err error) {
		e := errc.Catch(&err, errc.Fatal)
		defer e.Handle()

		r, err := newReader()
		e.Must(err)
		e.Defer(r.Close)

		r, err = newFaultyReader()
		e.Must(err)
		e.Defer(r.Close)
		return nil
	}()
}

func newReader() (io.ReadCloser, error) {
	return ioutil.NopCloser(strings.NewReader("Hello World!")), nil
}

func newFaultyReader() (io.ReadCloser, error) {
	return nil, errors.New("errd_test: error")
}

func ExampleCatch() {
	func() (err error) {
		e := errc.Catch(&err)
		defer e.Handle()

		r, err := newReader() // contents: Hello World!
		e.Must(err)
		e.Defer(r.Close)

		_, err = io.Copy(os.Stdout, r)
		e.Must(err)
		return nil
	}()
	// Output:
	// Hello World!
}

func ExampleCatch_pipe() {
	r, w := io.Pipe()
	go func() {
		var err error
		e := errc.Catch(&err)
		defer e.Handle()

		e.Defer(w.CloseWithError)

		r, err := newReader() // contents: Hello World!
		e.Must(err)
		e.Defer(r.Close)

		_, err = io.Copy(w, r)
	}()
	io.Copy(os.Stdout, r)

	// The above goroutine is equivalent to:
	//
	// go func() {
	// 	// err is used to intercept downstream errors. Note that we set it to a
	// 	// sentinel even though we recover the panic below to cover the case of
	// 	// a panic between the two defers. This is very unlikely to be
	// 	// necessary, but remember: a panic may be caused by external factors
	// 	// and code requiring high reliability should always consider the
	// 	// possibility of a panic occurring at any point.
	// 	var err = errors.New("panicking")
	//
	// 	// No need to intercept error: io.PipeWriter.CloseWithError always
	// 	 returns nil.
	// 	defer w.CloseWithError(err)
	//
	// 	// Ensure that CloseWithError is not called with a nil error on panic.
	// 	// In this case use recover: because we set err multiple times, it
	// 	// results seems a bit easier than managing everything by sentinel.
	// 	defer func() {
	// 		if v := recover(); v != nil {
	// 			err = errors.New("panicking")
	// 		}
	// 	}()
	//
	// 	r, err := newReader()
	// 	if err != nil {
	// 		return
	// 	}
	// 	defer func() {
	// 		if errC := r.Close(); err == nil {
	// 			err = errC
	// 		}
	// 	}()
	//
	// 	_, err = io.Copy(w, r)
	// }()

	// Output:
	// Hello World!
}

func do(ctx context.Context) {
	// do something
}

// ExampleCatcher_Defer_cancelHelper shows how a helper function may call a
// defer in the caller's E. Notice how contextWithTimeout taking care of the
// call to Defer is both evil and handy at the same time. Such a thing would
// likely not be allowed if this were a language feature.
func ExampleCatcher_Defer_cancelHelper() {
	contextWithTimeout := func(e *errc.Catcher, req *http.Request) context.Context {
		var cancel context.CancelFunc
		ctx := req.Context()
		timeout, err := time.ParseDuration(req.FormValue("timeout"))
		if err == nil {
			// The request has a timeout, so create a context that is
			// canceled automatically when the timeout expires.
			ctx, cancel = context.WithTimeout(ctx, timeout)
		} else {
			ctx, cancel = context.WithCancel(ctx)
		}
		e.Defer(cancel)
		return ctx
	}

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		var err error
		e := errc.Catch(&err)
		defer e.Handle()

		ctx := contextWithTimeout(&e, req)
		do(ctx)
	})
}
