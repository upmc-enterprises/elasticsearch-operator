#!/bin/bash -e

dep ensure $@
dep prune

# remove files we don't want
find vendor \( -name BUILD -o -name .travis.yml -o -name '*_test.go' \) -exec rm {} \;