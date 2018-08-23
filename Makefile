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
GOPATH := $(shell cd ../../../.. ; pwd)
GOCFG = GOPATH=$(GOPATH) GO15VENDOREXPERIMENT=$(GO15VENDOREXPERIMENT) \
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=$(CGO_ENABLED)
GOBUILD = $(GOCFG) go build

OSVER = $(shell uname -r | cut -d'.' -f1)
VERSION := $(shell git describe --abbrev=6 --dirty=+untagged --always --tags)
BUILDDATE = $(shell /bin/date "+%FT%T%Z")

OPAMROOT ?= ~/.opam
HYPERKIT_GIT = "https://github.com/moby/hyperkit.git"
HYPERKIT_COMMIT = "v0.20180123"

MKDIR = /bin/mkdir -p
CP = /bin/cp
MV = /bin/mv
RM = /bin/rm -rf
DATE = /bin/date
LN = /bin/ln -sf
SED = /usr/bin/sed
GREP = /usr/bin/grep
TOUCH = /usr/bin/touch
GIT = /usr/bin/git

ifeq ($(DEBUG),true)
    GO_GCFLAGS := $(GO_GCFLAGS) -N -l
;	GOBUILD = $(GOBUILD) -race
else
    GO_LDFLAGS := $(GO_LDFLAGS) -w -s
endif

GO_LDFLAGS := $(GO_LDFLAGS) \
	-X $(REPOSITORY)/release.Version=$(VERSION) \
	-X $(REPOSITORY)/release.BuildDate=$(BUILDDATE)

default: documentation

documentation: documentation/man documentation/markdown
	-$(GIT) status

all: clean hyperkit documentation

cmd: force
	$(RM) $(BUILD_DIR)/$(PROG)
	$(RM) $(BUILD_DIR)/$(DAEMON)
	$(MKDIR) $(BUILD_DIR)
	cd $@; $(GOBUILD) -o $(BUILD_DIR)/$(PROG) \
		-gcflags "$(GO_GCFLAGS)" -ldflags "$(GO_LDFLAGS)"
	cd $(BUILD_DIR); $(LN) $(PROG) $(DAEMON)
	@$(TOUCH) $@

components/common/assets: force
	cd $@; \
		$(RM) assets_vfsdata.go ; \
		$(GOCFG) go run assets_generator.go -tags=dev

clean: components/common/assets
	$(RM) $(BUILD_DIR)/*
	$(RM) hyperkit qcow-tool
	$(RM) documentation/
	$(RM) $(PROG)-$(VERSION).tar.gz

tarball: $(PROG)-$(VERSION).tar.gz

$(PROG)-$(VERSION).tar.gz: documentation hyperkit qcow-tool
	cd bin; tar cvzf ../$@ *

release: force
	github-release release -u $(TEAM) -r $(PROG) --tag $(VERSION) --draft
	github-release upload -u $(TEAM) -r $(PROG) \
		--label "macOS unsigned blobs (built in $(MACOS))" \
		--tag $(VERSION) --name "corectl-$(VERSION)-macOS-$(GOARCH).tar.gz" \
		--file $(PROG)-$(VERSION).tar.gz

hyperkit: force
	# - ocaml stack
	#   - 1st run
	# 	  - brew install opam libev
	# 	  - opam init -y
	# 	  - opam install --yes uri qcow-format ocamlfind conf-libev
	#   - maintenance
	#     - opam update && opam upgrade -y
	#   - build
	#     - make clean
	#     - eval `opam config env` && make all
	$(MKDIR) $(BUILD_DIR)
	$(RM) $@
	$(GIT) clone $(HYPERKIT_GIT);
	cd $@; \
		$(shell [[ $(OSVER) -eq 15 ]] && $(GIT) checkout 2c8a736c9bd1c86c95b7c4dac0c159c5712ac604 src/lib/block_if.c) \
		$(GIT) checkout $(HYPERKIT_COMMIT); \
		$(MAKE) clean; \
		$(SED) -i'' -e 's/zd/lld/g' src/lib/firmware/bootrom.c; \
		$(shell opam config env) $(MAKE) all;
	$(CP) $@/build/hyperkit $(BUILD_DIR)/corectld.runner
	$(RM) examples/dtrace
	cd $@; \
		$(SED) -i.bak -e "s,hyperkit,corectld.runner,g" dtrace/*.d; \
		$(RM) dtrace/*.bak ; \
		$(CP) -r dtrace ../examples

qcow-tool: force
	$(RM) $(BUILD_DIR)/$@
	$(CP) $(OPAMROOT)/system/bin/$@ $(BUILD_DIR)/$@

documentation/man: cmd force
	$(RM) $@
	$(MKDIR) $@
	$(BUILD_DIR)/$(PROG) utils genManPages
	$(BUILD_DIR)/$(DAEMON) utils genManPages
	for p in $$(ls $@/*.1); do \
		$(SED) -i.bak "s/$$($(DATE) '+%h %Y')//" "$$p" ;\
		$(SED) -i.bak "/spf13\/cobra$$/d" "$$p" ;\
		$(RM) "$$p.bak" ;\
	done

documentation/markdown: cmd force
	$(RM) $@
	$(MKDIR) $@
	$(BUILD_DIR)/$(PROG) utils genMarkdownDocs
	$(BUILD_DIR)/$(DAEMON) utils genMarkdownDocs
	for p in $$(ls $@/*.md); do \
		$(SED) -i.bak "/spf13\/cobra/d" "$$p" ;\
		$(RM) "$$p.bak" ;\
	done

.PHONY: clean all docs force assets cmd
