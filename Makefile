export GOARCH=amd64
export GOOS=darwin
export CGO_ENABLED=1

VERSION := $(shell git describe --abbrev=6 --dirty=-unreleased --always --tags)

ifeq ($(DEBUG),true)
    GO_GCFLAGS := -gcflags "-N -l"
else
    GO_LDFLAGS := $(GO_LDFLAGS) -w -s
endif

GO_LDFLAGS := $(GO_LDFLAGS) -X main.Version=$(VERSION)

all: corectl docs
	@git status

homebrewHack:
	@echo godep go build -o corectl ${GO_GCFLAGS} -ldflags \"${GO_LDFLAGS}\"

corectl: clean Makefile
	godep go build -o corectl ${GO_GCFLAGS} -ldflags "${GO_LDFLAGS}"
	@touch $@

clean:
	@rm -rf corectl ./documentation/

godeps_diff:
	@godep diff

godeps_save: godeps_diff
	@rm -rf Godeps/
	@godep save ./...
	@git status

docs: corectl documentation/markdown documentation/man

documentation/man: force
	@mkdir -p documentation/man
	@./corectl utils mkMan
	@for p in $$(ls documentation/man/*.1); do \
		gsed -i "s/$$(/bin/date '+%h %Y')//" "$$p" ;\
		gsed -i '/spf13\/cobra$$/d' "$$p" ;\
	done

documentation/markdown: force
	@mkdir -p documentation/markdown
	@./corectl utils mkMkdown
	@for p in $$(ls documentation/markdown/*.md); do \
		gsed -i '/spf13\/cobra/d' "$$p" ;\
	done

.PHONY: clean all docs force
