include xhyve.mk

build:
	go build -o xhyve cmd/xhyve/main.go

clone-xhyve:
	-git clone https://github.com/xhyve-xyz/xhyve.git vendor/xhyve
	# cherry-picked from https://github.com/mist64/xhyve/pull/81
	# Fix non-deterministic delays when accessing a vcpu in "running" or "sleeping" state.
	-cd vendor/xhyve; curl -Ls https://patch-diff.githubusercontent.com/raw/mist64/xhyve/pull/81.patch | patch -N -p1
	# experimental support for raw devices - https://github.com/mist64/xhyve/pull/80
	-cd vendor/xhyve; curl -Ls https://patch-diff.githubusercontent.com/raw/mist64/xhyve/pull/80.patch | patch -N -p1

sync: clone-xhyve apply-patch
	find . \( -name \*.orig -o -name \*.rej \) -delete
	for file in $(SRC); do \
		cp -f $$file $$(basename $$file) ; \
	done
	cp -r vendor/xhyve/include include

apply-patch:
	-cd vendor/xhyve; patch -N -p1 < ../../xhyve.patch

generate-patch: apply-patch
	cd vendor/xhyve; git diff > ../../xhyve.patch

clean:
	rm -rf *.c vendor include

.PHONY: build clone-xhyve sync apply-patch generate-patch clean
