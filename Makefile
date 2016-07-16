PROG = corectl
DAEMON = corectld
TEAM = TheNewNormal
ORGANIZATION = github.com/$(TEAM)
REPOSITORY = $(ORGANIZATION)/$(PROG)

GOARCH ?= $(shell go env GOARCH)
GOOS ?= $(shell go env GOOS)
CGO_ENABLED = 1
GO15VENDOREXPERIMENT = 1
MACOS = $(shell sw_vers -productVersion | /usr/bin/awk -F '.' '{print $$1 "." $$2}')

BUILD_DIR ?= $(shell pwd)/bin
GOPATH := $(shell cd ../../../.. ; pwd):$(shell mkdir -p Godeps && \
			godep go env GOPATH)
GODEP = GOPATH=$(GOPATH) GO15VENDOREXPERIMENT=$(GO15VENDOREXPERIMENT) \
    GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=$(CGO_ENABLED) godep
GOBUILD = $(GODEP) go build

VERSION := $(shell git describe --abbrev=6 --dirty=+untagged --always --tags)
BUILDDATE = $(shell /bin/date "+%FT%T%Z")

HYPERKIT_GIT = "https://github.com/docker/hyperkit.git"
HYPERKIT_COMMIT = 1e4b9b8d252c2fb5eee39830591a819c490eaf5e

MKDIR = /bin/mkdir -p
CP = /bin/cp
MV = /bin/mv
RM = /bin/rm -rf
DATE = /bin/date
SED = /usr/bin/sed
GREP = /usr/bin/grep
TOUCH = /usr/bin/touch
GIT = /usr/bin/git

ifeq ($(DEBUG),true)
    GO_GCFLAGS := $(GO_GCFLAGS) -N -l
	GOBUILD = $(GOBUILD) -race
else
    GO_LDFLAGS := $(GO_LDFLAGS) -w -s
endif

GO_LDFLAGS := $(GO_LDFLAGS) \
	-X $(REPOSITORY)/release.Version=$(VERSION) \
	-X $(REPOSITORY)/release.BuildDate=$(BUILDDATE)

default: documentation

documentation: documentation/man documentation/markdown
	-$(GIT) status

all: clean Godeps hyperkit documentation

cmd: cmd/client cmd/server

cmd/client: force
	$(RM) $(BUILD_DIR)/$(PROG)
	$(MKDIR) $(BUILD_DIR)
	cd $@; $(GOBUILD) -o $(BUILD_DIR)/$(PROG) \
		-gcflags "$(GO_GCFLAGS)" -ldflags "$(GO_LDFLAGS)"
	@$(TOUCH) $@

cmd/server: force
	$(RM) $(BUILD_DIR)/$(DAEMON)
	$(MKDIR) $(BUILD_DIR)
	cd $@; $(GOBUILD) -o $(BUILD_DIR)/$(DAEMON) \
		-gcflags "$(GO_GCFLAGS)" -ldflags "$(GO_LDFLAGS)"
	@$(TOUCH) $@

components/common/assets: force
	cd $@; \
		$(RM) assets_vfsdata.go ; \
		$(GODEP) go run assets_generator.go -tags=dev

clean: components/common/assets
	$(RM) $(BUILD_DIR)/*
	$(RM) hyperkit
	$(RM) documentation/
	$(RM) $(PROG)-$(VERSION).tar.gz

tarball: $(PROG)-$(VERSION).tar.gz

$(PROG)-$(VERSION).tar.gz: documentation hyperkit
	cd bin; tar cvzf ../$@ *

release: force
	github-release release -u $(TEAM) -r $(PROG) --tag $(VERSION) --draft
	github-release upload -u $(TEAM) -r $(PROG) \
		--label "macOS unsigned blobs (built in $(MACOS))" \
		--tag $(VERSION) --name "corectl-$(VERSION)-macOS-$(GOARCH).tar.gz" \
		--file $(PROG)-$(VERSION).tar.gz

update-godeps:
	for x in $$($(GREP) ImportPath Godeps/Godeps.json | $(GREP) -v \
		$(ORGANIZATION) | $(SED) -e "s,.*:,," -e "s|,||" -e "s|\"||g" ) ; \
			do echo "==$$x"; go get -u $$x; done

Godeps: force
	$(RM) $@
	$(RM) vendor/
	# XXX godep won't save this as a build dep run a runtime one so we cheat...
	$(SED) -i.bak \
		-e s"|github.com/helm/helm/log|github.com/shurcooL/vfsgen|" \
		-e "s|import (|import ( \"github.com/shurcooL/httpfs/vfsutil\"|" \
			components/common/assets/assets.go
	$(GODEP) save ./...
	# ... and then un-cheat
	$(CP) components/common/assets/assets.go.bak \
		components/common/assets/assets.go
	$(RM) components/common/assets/assets.go.bak
	-$(GIT) status

hyperkit: force
	# - ocaml stack
	#   - 1st run
	# 	  - brew install opam
	# 	  - opam init -y
	# 	  - opam pin add qcow-format
	#					"git://github.com/mirage/ocaml-qcow#master" -y
	# 	  - opam install --yes uri qcow-format ocamlfind
	#   - maintenance
	#     - opam update && opam upgrade -y
	# 	  - opam pin add qcow-format
	#					"git://github.com/mirage/ocaml-qcow#master" -y
	# 	  - opam install --yes uri qcow-format ocamlfind
	#   - build
	#     - make clean
	#     - eval `opam config env` && make all
	$(MKDIR) $(BUILD_DIR)
	$(RM) $@
	$(GIT) clone $(HYPERKIT_GIT)
	cd $@; \
		$(GIT) checkout $(HYPERKIT_COMMIT); \
		$(MAKE) clean; \
		$(shell opam config env) $(MAKE) all
	$(CP) $@/build/com.docker.hyperkit $(BUILD_DIR)/corectld.runner
	$(RM) examples/dtrace
	cd $@; \
		$(SED) -i.bak -e "s,com.docker.hyperkit,corectld.runner,g" dtrace/*.d; \
		$(RM) dtrace/*.bak ; \
		$(CP) -r dtrace ../examples

documentation/man: cmd force
	$(MKDIR) $@
	$(BUILD_DIR)/$(PROG) utils genManPages
	$(BUILD_DIR)/$(DAEMON) utils genManPages
	for p in $$(ls $@/*.1); do \
		$(SED) -i.bak "s/$$($(DATE) '+%h %Y')//" "$$p" ;\
		$(SED) -i.bak "/spf13\/cobra$$/d" "$$p" ;\
		$(RM) "$$p.bak" ;\
	done

documentation/markdown: cmd force
	$(MKDIR) $@
	$(BUILD_DIR)/$(PROG) utils genMarkdownDocs
	$(BUILD_DIR)/$(DAEMON) utils genMarkdownDocs
	for p in $$(ls $@/*.md); do \
		$(SED) -i.bak "/spf13\/cobra/d" "$$p" ;\
		$(RM) "$$p.bak" ;\
	done

.PHONY: clean all docs force assets cmd update-godeps
