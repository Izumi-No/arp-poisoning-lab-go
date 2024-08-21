#!/bin/bash


go mod tidy &&
env GOOS=linux GOARCH=amd64 go build -o dist/server server/server.go &&
env GOOS=linux GOARCH=amd64 go build -o dist/client client/client.go

 