
export GOARCH=amd64
export GOOS=darwin
export CGO_ENABLED=1

export GO15VENDOREXPERIMENT=1

PROG = corectl
ORGANIZATION = github.com/TheNewNormal
REPOSITORY = $(ORGANIZATION)/$(PROG)

export GOPATH=$(shell echo $(PWD) | sed -e "s,src/$(REPOSITORY).*,,")

VERSION := $(shell git describe --abbrev=6 --dirty=-unreleased --always --tags)
BUILDDATE = $(shell /bin/date "+%FT%T%Z")

ifeq ($(DEBUG),true)
    GO_GCFLAGS := $(GO_GCFLAGS) -N -l
else
    GO_LDFLAGS := $(GO_LDFLAGS) -w -s
endif

GO_LDFLAGS := $(GO_LDFLAGS) -X main.Version=$(VERSION) \
    -X main.BuildDate=$(BUILDDATE)

all: $(PROG) docs
	@git status

$(PROG): clean Makefile
	godep go build -o $(PROG) -gcflags "$(GO_GCFLAGS)" -ldflags "$(GO_LDFLAGS)"
	@touch $@

clean:
	@rm -rf $(PROG) documentation/

godeps_diff:
	@godep diff

godeps_save: godeps_diff
	@rm -rf Godeps/
	@godep save
	@git status

docs: ${NAME} documentation/markdown documentation/man

documentation/man: force
	@mkdir -p documentation/man
	./$(PROG) utils mkMan
	@for p in $$(ls documentation/man/*.1); do \
		sed -i.bak "s/$$(/bin/date '+%h %Y')//" "$$p" ;\
		sed -i.bak "/spf13\/cobra$$/d" "$$p" ;\
		rm "$$p.bak" ;\
	done

documentation/markdown: force
	@mkdir -p documentation/markdown
	./$(PROG) utils mkMkdown
	@for p in $$(ls documentation/markdown/*.md); do \
		sed -i.bak "/spf13\/cobra/d" "$$p" ;\
		rm "$$p.bak" ;\
	done

.PHONY: clean all docs force
