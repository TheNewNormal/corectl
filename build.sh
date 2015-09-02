#!/bin/sh

VERSION=$(git describe --abbrev=6 --dirty --always --tags)
V="blablabla.go"

echo "package main" > ${V}
echo "var Version = \"${VERSION}\"" >> ${V}
rm -rf ./Godeps
godep save ./...
git status
godep go build -o coreos-xhyve ./*.go

rm -rf ./documentation/*
mkdir -p ./documentation/{man,markdown}
COREOS_DEBUG=true ./coreos-xhyve utils mkMan
(pushd ./documentation/man
    for page in *.1; do
        sed -i '/spf13\/cobra$/ d' "${page}"
    done
popd
COREOS_DEBUG=true ./coreos-xhyve utils mkMkdown
pushd ./documentation/markdown
    for page in *.md; do
        sed -i '/spf13\/cobra/d' "${page}"
    done
popd) >/dev/null
