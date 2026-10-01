set windows-shell := ["powershell.exe", "-NoProfile", "-Command"]

default:
    go run .

run *args:
    go run . {{args}}

import *args:
    go run . import {{args}}

players:
    go run . players

update:
    go run . update

find *args:
    go run . find {{args}}

path:
    go run . path

build:
    go build .

check:
    go vet ./...
    go test ./...
