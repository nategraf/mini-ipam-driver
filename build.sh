#!/usr/bin/env bash
set -e

export GOARCH=amd64
export GOOS=linux
go build -o mini-ipam-driver.Linux.x64

export GOARCH=386
go build -o mini-ipam-driver.Linux.x86

export GOOS=windows
go build -o mini-ipam-driver.Windows.x86.exe

export GOARCH=amd64
go build -o mini-ipam-driver.Windows.x64.exe

export GOARCH=
export GOOS=
