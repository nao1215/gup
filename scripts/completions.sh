#!/bin/sh
# scripts/completions.sh
set -eux

rm -rf completions
mkdir completions

for sh in bash zsh fish; do
    go run main.go completion "$sh" >"completions/gup.$sh"
done
