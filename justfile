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

watch *args:
    go run . watch {{args}}

path:
    go run . path

db-export *args:
    go run . db-export {{args}}

db-import *args:
    go run . db-import {{args}}

build:
    go build .

# Regenerate internal/store/*_gen.go after editing migrations or queries.
generate:
    sqlc generate

check:
    go vet ./...
    go test ./...
