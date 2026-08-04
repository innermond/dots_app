#!/bin/bash
cd "$(dirname "$0")"

if ! [[ "$1" =~ ^[0-9]+$ ]]; then
  echo "not a number $1"
  exit 1
fi

migrate -path ./ -database postgres://dots_owner:d0ts_0wn3r@postgres.dots.volt.com:5432/dots?sslmode=disable -verbose force "$1"
