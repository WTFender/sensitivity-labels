#!/bin/bash
GOOS=darwin  GOARCH=amd64 go build -o "./bin/labels" ./main.go