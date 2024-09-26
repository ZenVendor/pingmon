#!/bin/bash

gofmt -s -w .
go build -o test/pingmon .
