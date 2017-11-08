// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package errc simplifies error and defer handling.
//
// Overview
//
// Package errc is a burner package: a proof-of-concept to explore better
// semantics for error and defer handling. Error handling and deferring using
// this package looks like:
//
//    func foo() (err error) {
//        e := errc.Catch(&err)
//        defer e.Handle()
//
//        r, err := getReader()
//        e.Must(err, msg("read failed"))
//        e.Defer(r.Close)
//
// Checking for a nil error is replaced by a call to Must on an error catcher.
// A defer statement is similarly replaced by a call to Defer.
//
//
// The Problem
//
// Error handling in Go can also be tricky to get right. For instance, how to
// use defer may depend on the situation. For a Close method that frees up
// resources a simple use of defer suffices. For a CloseWithError method where a
// nil error indicates the successful completion of a transaction, however, it
// should be ensured that a nil error is not passed inadvertently, for instance
// when there is a panic if a server runs out of memory and is killed by a
// cluster manager.
//
// For instance, a correct way to commit a file to Google Cloud Storage is:
//
//    func writeToGS(ctx context.Context, bucket, dst string, r io.Reader) (err error) {
//        client, err := storage.NewClient(ctx)
//        if err != nil {
//            return err
//        }
//        defer client.Close()
//
//        w := client.Bucket(bucket).Object(dst).NewWriter(ctx)
//        err = errPanicking
//        defer func() {
//            if err != nil {
//                _ = w.CloseWithError(err)
//            } else {
//                err = w.Close()
//            }
//        }
//        _, err = io.Copy(w, r)
//        return err
//    }
//
// The err variable is initialized to errPanicking to ensure a non-nil err is
// passed to CloseWithError when a panic occurs. This ensures that a panic
// will not cause a corrupted file. If all went well, a separate path used
// to collect the error returned by Close. Returning the error from Close is
// important to signal retry logic the file was not successfully written.
// Once the Close of w is successful all further errors are irrelevant.
// The error of the first Close is therefor willfully ignored.
//
// These are a lot of subtleties to get the error handling working properly!
//
// The same can be achieved using errc as follows:
//
//    func writeToGS(ctx context.Context, bucket, dst, src string) (err error) {
//        e := errc.Catch(&err)
//        defer e.Handle()
//
//        client, err := storage.NewClient(ctx)
//        e.Must(err)
//        e.Defer(client.Close, errd.Discard)
//
//        w := client.Bucket(bucket).Object(dst).NewWriter(ctx)
//        e.Defer(w.CloseWithError)
//
//        _, err = io.Copy(w, r)
//        return err
//    }
//
// Observe how a straightforward application of idiomatic check-and-defer
// pattern leads to the correct results. The error of the first Close is now
// ignored explicitly using the Discard error handler,
// making it clear that this is what the programmer intended.
//
//
// Error Handlers
//
// Error handlers can be used to decorate errors, log them, or do anything else
// you usually do with errors.
//
// Suppose we want to use github.com/pkg/errors to decorate errors. A simple
// handler can be defined as:
//
//     type msg string
//
//     func (m msg) Handle(s errc.State, err error) error {
//         return errors.WithMessage(err, string(m))
//     }
//
// This handler can then be used as follows:
//
//     func writeToGS(ctx context.Context, bucket, dst, src string) (err error) {
//         e := errc.Catch(&err)
//         defer e.Handle()
//
//         client, err := storage.NewClient(ctx)
//         e.Must(err, msg("error opening client"))
//         e.Defer(client.Close)
//
//         w := client.Bucket(bucket).Object(dst).NewWriter(ctx)
//         e.Defer(w.CloseWithError)
//
//         _, err = io.Copy(w, r)
//         e.Must(err, msg("error copying contents"))
//         return nil
//     }
//
package errc
