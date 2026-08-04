#!/bin/bash
cd "$(dirname "$0")"

# replace spaces with hyphens
name=$(echo "$@" | sed 's/ /-/g')
migrate create -ext sql -dir ./ -seq -digits 3 "$name"


