#!/bin/bash

GOOS=linux GOARCH=amd64

go mod tidy && go build -o dist/server server/server.go &&  go build -o dist/client client/client.go

 