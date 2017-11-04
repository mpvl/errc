// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errc

import (
	"errors"
	"fmt"
)

// Catch returns an error Catcher, which is used to funnel errors from panics
// and failed calls to Try. It must be passed the location of an error variable.
// The user must defer a call to Handle immediately after the creation of the
// Catcher as follows:
//     e := errc.Catch(&err)
//     defer e.Handle()
func Catch(err *error, h ...Handler) Catcher {
	ec := Catcher{core{defaultHandlers: h, err: err}}
	ec.deferred = ec.buf[:0]
	return ec
}

const bufSize = 3

type core struct {
	defaultHandlers []Handler
	deferred        []deferData
	buf             [bufSize]deferData
	err             *error
	inPanic         bool
}

// A Catcher coordinates error and defer handling.
type Catcher struct{ core }

var errHandlerFirst = errors.New("errd: handler may not be first argument")

// Must causes a return from a function if err is not nil, and after the error
// is not nullified by any of the Handlers.
func (e *Catcher) Must(err error, h ...Handler) {
	if err != nil {
		processError(e, err, h)
	}
}

// State represents the error state passed to custom error handlers.
type State interface {
	// Panicking reports whether the error resulted from a panic. If true,
	// the panic will be resume after error handling completes. An error handler
	// cannot rewrite an error when panicing.
	Panicking() bool

	// Err reports the first error that passed through an error handler chain.
	// Note that this is always a different error (or nil) than the one passed
	// to an error handler.
	Err() error
}

type state struct{ core }

func (s *state) Panicking() bool { return s.inPanic }

func (s *state) Err() error {
	if s.err == nil {
		return nil
	}
	return *s.err
}

var errOurPanic = errors.New("errd: our panic")

// Handle manages the error handling and defer processing. It must be called
// after any call to Catch.
func (e *Catcher) Handle() {
	switch r := recover(); r {
	case nil:
		finishDefer(e)
	case errOurPanic:
		finishDefer(e)
	default:
		e.inPanic = true
		err2, ok := r.(error)
		if !ok {
			err2 = fmt.Errorf("errd: paniced: %v", r)
		}
		*e.err = err2
		finishDefer(e)
		// Check whether there are still defers left to do and then
		// recursively defer.
		panic(r)
	}
}

func doDefers(e *Catcher, barrier int) {
	for len(e.deferred) > barrier {
		i := len(e.deferred) - 1
		d := e.deferred[i]
		e.deferred = e.deferred[:i]
		if d.f == nil {
			continue
		}
		if err := d.f((*state)(e), d.x); err != nil {
			processDeferError(e, err)
		}
	}
}

// finishDefer processes remaining defers after we already have a panic.
// We therefore ignore any panic caught here, knowing that we will panic on an
// older panic after returning.
func finishDefer(e *Catcher) {
	if len(e.deferred) > 0 {
		defer e.Handle()
		doDefers(e, 0)
	}
}

type errorHandler struct {
	e   *Catcher
	err *error
}

func (h errorHandler) handle(eh Handler) (done bool) {
	newErr := eh.Handle((*state)(h.e), *h.err)
	if newErr == nil {
		return true
	}
	*h.err = newErr
	return false

}

func processDeferError(e *Catcher, err error) {
	eh := errorHandler{e: e, err: &err}
	hadHandler := false
	// Apply handlers added by Defer methods. A zero deferred value signals that
	// we have custom defer handler for the subsequent fields.
	for i := len(e.deferred); i > 0 && e.deferred[i-1].f == nil; i-- {
		hadHandler = true
		if eh.handle(e.deferred[i-1].x.(Handler)) {
			return
		}
	}
	if !hadHandler {
		for _, h := range e.defaultHandlers {
			if eh.handle(h) {
				return
			}
		}
	}
	if *e.err == nil {
		*e.err = err
	}
}

func processError(e *Catcher, err error, handlers []Handler) {
	eh := errorHandler{e: e, err: &err}
	for _, h := range handlers {
		if eh.handle(h) {
			return
		}
	}
	if len(handlers) == 0 {
		for _, h := range e.defaultHandlers {
			if eh.handle(h) {
				return
			}
		}
	}
	if *e.err == nil {
		*e.err = err
	}
	bail(e)
}

func bail(e *Catcher) {
	// Do defers now and save an extra defer.
	doDefers(e, 0)
	panic(errOurPanic)
}
