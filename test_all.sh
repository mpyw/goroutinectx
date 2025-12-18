#!/bin/sh

set -x

go run github.com/santhosh-tekuri/jsonschema/cmd/jv@29cbed9 testdata/metatest/structure.schema.json testdata/metatest/structure.json
go test ./testdata/metatest/validation_test.go
go test -v ./...
