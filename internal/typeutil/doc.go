// Package typeutil provides type utilities for goroutinectx.
//
// # Overview
//
// [UnwrapPointer] takes off every pointer and alias around a type.
// [Embeds] tells whether a type, past its pointers, is one a match function
// accepts, or an interface that embeds one, or a type parameter constrained
// by one:
//
//	Embeds(MyCtx, isContextNamed)  // true, for type MyCtx interface{ context.Context }
//	Embeds(C, isContextNamed)      // true, for [C context.Context]
//
// The ctxkind package tells context.Context with it, and the carrier
// package a carrier. What counts as a context is up to ctxkind.
//
// # Package Paths
//
// A function or a type given in a flag names its package by import path.
// [CheckPkgPath] rejects a bare name that is not a standard library package,
// for funcspec and carrier alike, and [PkgPathMatches] matches a path,
// allowing a major version suffix such as /v2.
//
// # List Items
//
// [CheckItem] rejects a blank item of a list setting, and an item that
// holds a comma. Only the command line splits a setting at commas.
package typeutil
