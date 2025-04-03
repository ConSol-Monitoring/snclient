#!/usr/bin/make -f

MAKE:=make
SHELL:=bash

test:
	go test -v

testf: vendor
	go test -v -run "$(filter-out $@,$(MAKECMDGOALS))" 2>&1 | grep -v "no test files" | grep -v "no tests to run" | grep -v "^PASS"

clean:
	rm -f snclient
	rm -f snclient.exe

# just skip unknown make targets
.DEFAULT:
	@if [[ "$(MAKECMDGOALS)" =~ ^testf ]]; then \
		: ; \
	else \
		echo "unknown make target(s): $(MAKECMDGOALS)"; \
		exit 1; \
	fi


