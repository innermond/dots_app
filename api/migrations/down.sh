#!/bin/bash
cd "$(dirname "$0")"

migrate -path ./ -database postgres://dots_owner:d0ts_0wn3r@postgres.dots.volt.com:5432/dots?sslmode=disable -verbose down "${1:-1}"

