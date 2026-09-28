// Package ignoreforms tests how //goroutinectx:ignore is read: a trailing
// comment is a reason, an unknown checker is reported and silences nothing,
// and an ignore is used only when it silences a report.
package ignoreforms

import (
	"context"

	"github.com/sourcegraph/conc"
	"golang.org/x/sync/errgroup"
)

// [GOOD]: A reason after //, with no checker, is a bare ignore.
func goodSlashReason(ctx context.Context) {
	//goroutinectx:ignore // fire and forget
	go func() {}()
}

// [GOOD]: The same, written with no space after the slashes.
func goodSlashReasonNoSpace(ctx context.Context) {
	//goroutinectx:ignore //fire and forget
	go func() {}()
}

// [GOOD]: A reason after " - " holding a URL is still one reason.
func goodDashReasonURL(ctx context.Context) {
	//goroutinectx:ignore - see https://example.com/detached
	go func() {}()
}

// [BAD]: A reason without " - " is not a checker. It is reported, and the
// directive silences nothing.
func badReasonWithoutDash(ctx context.Context) {
	//goroutinectx:ignore intentionally detached // want `unknown checker "intentionally detached" in goroutinectx:ignore \(want one of goroutine, goroutinederive, waitgroup, errgroup, conc, spawner, spawnerlabel, gotask; write a reason after " - " or "//"\)`
	go func() {}() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: A checker followed by words is one unknown name.
func badCheckerThenWords(ctx context.Context) {
	//goroutinectx:ignore goroutine intentionally detached // want `unknown checker "goroutine intentionally detached"`
	go func() {}() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: A misspelled checker.
func badMisspelled(ctx context.Context) {
	//goroutinectx:ignore gorutine // want `unknown checker "gorutine"`
	go func() {}() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: One unknown name makes the whole directive unknown, as in
// declscope. The valid name beside it silences nothing either.
func badOneUnknownInList(ctx context.Context) {
	//goroutinectx:ignore goroutine,typo // want `unknown checker "typo"`
	go func() {}() // want `goroutine does not propagate context "ctx"`
}

// [BAD]: A bare ignore on a go statement that reports nothing is unused.
func badUnusedOnGoStmt(ctx context.Context) {
	//goroutinectx:ignore // want `^unused goroutinectx:ignore directive$`
	go func() {
		_ = ctx
	}()
}

// [BAD]: A bare ignore on a call that reports nothing is unused.
func badUnusedOnCall(ctx context.Context) {
	g := new(errgroup.Group)
	//goroutinectx:ignore // want `^unused goroutinectx:ignore directive$`
	g.Go(func() error {
		_ = ctx
		return nil
	})
	_ = g.Wait()
}

// [GOOD]: conc names the conc checker.
func goodConcIgnore(ctx context.Context) {
	p := &conc.Pool{}
	//goroutinectx:ignore conc
	p.Go(func() {})
	p.Wait()
}

// [GOOD]: errgroup silenced conc before conc had a name, and still does.
func goodConcIgnoredAsErrgroup(ctx context.Context) {
	p := &conc.Pool{}
	//goroutinectx:ignore errgroup
	p.Go(func() {})
	p.Wait()
}

// [BAD]: conc on a conc call that reports nothing is unused.
func badUnusedConc(ctx context.Context) {
	p := &conc.Pool{}
	//goroutinectx:ignore conc // want `unused goroutinectx:ignore directive for checker\(s\): conc`
	p.Go(func() {
		_ = ctx
	})
	p.Wait()
}
