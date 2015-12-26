VERSION := $(shell git describe --abbrev=6 --dirty=-unreleased --always --tags)
V := "blablabla.go"

all: corectl docs
	@git status

corectl: clean Makefile
	@echo "package main" > $(V)
	@echo "var Version = \"$(VERSION)\"" >> $(V)
	@mkdir -p ./documentation/
	godep go build -o corectl
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
	@mkdir documentation/man
	@./corectl utils mkMan
	@for p in $$(ls documentation/man/*.1); do \
		sed -i "s/$$(/bin/date '+%h %Y')//" "$$p" ;\
		sed -i '/spf13\/cobra$$/d' "$$p" ;\
	done

documentation/markdown: force
		@mkdir documentation/markdown
		@./corectl utils mkMkdown
		@for p in $$(ls documentation/markdown/*.md); do \
			sed -i '/spf13\/cobra/d' "$$p" ;\
		done

.PHONY: clean all docs force
