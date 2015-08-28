#!/bin/sh

VERSION=$(git describe --abbrev=6 --dirty --always --tags)
V="blablabla.go"

echo "package main" > ${V}
echo "var Version = \"${VERSION}\"" >> ${V}
godep save ./...
git status
godep go build -o coreos-xhyve *.go
mkdir -p ./documentation/{man,markdown}
COREOS_DEBUG=true ./coreos-xhyve utils mkMan
(pushd ./documentation/man
    for page in $(ls *.1); do
        sed -i '/^\.TH/ d' ${page}
        sed -i '/spf13\/cobra$/ d' ${page}
    done
popd
COREOS_DEBUG=true ./coreos-xhyve utils mkMkdown
pushd ./documentation/markdown
    for page in $(ls *.md); do
        sed -i '/spf13\/cobra/d' ${page}
    done
popd) >/dev/null
