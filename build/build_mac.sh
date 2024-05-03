#!/bin/bash
outFile="./bin/labels"
entryFile="./cmd/labels/main.go"
GOOS=darwin
GOARCH=amd64
go build -o $outFile $entryFile