#!/bin/sh 
export GOPATH='/workspace'

cd artificer 
go version
go get -v
go build -v

