#!/bin/bash


go mod tidy &&
env GOOS=linux GOARCH=amd64 go build -o dist/server.elf server/server.go &&
env GOOS=linux GOARCH=amd64 go build -o dist/client.elf client/client.go

 