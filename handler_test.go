// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errc

import (
	"fmt"
	"testing"
)

type intErr int

func (i intErr) Error() string { return fmt.Sprint(int(i)) }

var (
	err0 = intErr(0)
	err1 = intErr(1)
	err2 = intErr(2)
	err3 = intErr(3)

	identity = HandlerFunc(func(s State, err error) error {
		return err
	})

	inc = HandlerFunc(func(s State, err error) error {
		i := err.(intErr)
		return i + 1
	})

	dec = HandlerFunc(func(s State, err error) error {
		i := err.(intErr)
		return i - 1
	})
)

func TestOptions(t *testing.T) {
	// Error unconditionally generated in the second statement.
	testCases := []struct {
		desc        string
		defHandlers []Handler
		handlers1   []Handler
		handlersD1  []Handler
		handlers2   []Handler
		def1        interface{}
		err1        error
		err2        error
		want        error
	}{{
		desc: "no option",
		err1: nil,
		want: nil,
	}, {
		desc:        "default option",
		defHandlers: []Handler{inc},
		err1:        err0,
		want:        err1,
	}, {
		desc:        "default twice",
		defHandlers: []Handler{inc, inc},
		err1:        err0,
		want:        err2,
	}, {
		desc:        "mask default",
		defHandlers: []Handler{inc},
		handlers1:   []Handler{dec},
		err1:        err2,
		want:        err1,
	}, {
		desc:        "test DefaultFunc",
		defHandlers: []Handler{inc},
		handlers1:   []Handler{dec},
		err1:        err2,
		want:        err1,
	}, {
		desc:      "handler once",
		handlers1: []Handler{inc},
		err1:      err1,
		want:      err2,
	}, {
		desc:      "handler twice",
		handlers1: []Handler{inc, inc},
		err1:      err1,
		want:      err3,
	}, {
		desc:      "erase",
		handlers1: []Handler{Discard},
		handlers2: []Handler{Discard},
		err1:      err1,
		err2:      err2,
		want:      nil,
	}, {
		desc:        "erase in default handler",
		defHandlers: []Handler{Discard},
		err1:        err1,
		err2:        err2,
		want:        nil,
	}, {
		desc:      "handler cannot clear error",
		handlers2: []Handler{Discard},
		err1:      err1,
		err2:      err2,
		want:      err1,
	}}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := func() (err error) {
				e := Catch(&err, tc.defHandlers...)
				defer e.Handle()
				args := []interface{}{tc.def1, tc.err1}
				for _, h := range tc.handlers1 {
					args = append(args, h)
				}
				e.Must(tc.err1, tc.handlers1...)
				e.Defer(tc.def1, tc.handlersD1...)
				e.Must(tc.err2, tc.handlers2...)
				return nil
			}()
			if got != tc.want {
				t.Errorf("got %v; want %v", got, tc.want)
			}
		})
	}
}

type msg string

func (m msg) Handle(s State, err error) error {
	return err
}

func TestOptionAlloc(t *testing.T) {
	var e Catcher
	f := testing.AllocsPerRun(10, func() {
		// Technically an allocation, although it is a really cheap one as of
		// Go 1.9.
		e.Must(nil, msg("foo"))
	})
	if f > 1 {
		t.Errorf("got %v; want %v", f, 0)
	}
}

func BenchmarkNoOption(b *testing.B) {
	var err error
	e := Catch(&err)
	defer e.Handle()
	for i := 0; i < b.N; i++ {
		e.Must(nil)
	}
}

func BenchmarkSavedHandlerOption(b *testing.B) {
	test := Handler(msg("test"))
	var err error
	e := Catch(&err)
	defer e.Handle()
	for i := 0; i < b.N; i++ {
		e.Must(nil, test)
	}

}

func BenchmarkStringOption(b *testing.B) {
	var err error
	e := Catch(&err)
	defer e.Handle()
	for i := 0; i < b.N; i++ {
		e.Must(nil, msg("error doing benchmark"))
	}

}

func BenchmarkFuncOption(b *testing.B) {
	var err error
	e := Catch(&err)
	defer e.Handle()
	for i := 0; i < b.N; i++ {
		e.Must(nil, HandlerFunc(func(s State, err error) error {
			return err
		}))
	}
}
