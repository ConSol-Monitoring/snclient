#!/usr/bin/make -f

MAKE:=make
SHELL:=bash
GOVERSION:=$(shell \
    go version | \
    awk -F'go| ' '{ split($$5, a, /\./); printf ("%04d%04d", a[1], a[2]); exit; }' \
)
MINGOVERSION:=00010019
MINGOVERSIONSTR:=1.19
BUILD:=$(shell git rev-parse --short HEAD)
REVISION:=$(shell printf "%04d" $$( git rev-list --all --count))
# see https://github.com/go-modules-by-example/index/blob/master/010_tools/README.md
# and https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
TOOLSFOLDER=$(shell pwd)/tools
export GOBIN := $(TOOLSFOLDER)
export PATH := $(GOBIN):$(PATH)

VERSION ?= $(shell ./buildtools/get_version)
ARCH    ?= $(shell go env GOARCH)
DEBFILE ?= snclient-$(VERSION)-$(BUILD)-$(ARCH).deb
RPM_TOPDIR=$(shell pwd)/rpm.top

all: build

CMDS = $(shell cd ./cmd && ls -1)

tools: versioncheck vendor dump
	go mod download
	set -e; for DEP in $(shell grep "_ " buildtools/tools.go | awk '{ print $$2 }'); do \
		go install $$DEP; \
	done
	go mod tidy
	go mod vendor

updatedeps: versioncheck
	$(MAKE) clean
	go mod download
	set -e; for DEP in $(shell grep "_ " buildtools/tools.go | awk '{ print $$2 }'); do \
		go get $$DEP; \
	done
	go get -u ./...
	go get -t -u ./...
	go mod tidy

vendor:
	go mod download
	go mod tidy
	go mod vendor

dump:
	if [ $(shell grep -r Dump *.go ./pkg/*/*.go ./internal/*/*.go ./cmd/*/*.go | grep -v DumpRe | grep -v dump.go | wc -l) -ne 0 ]; then \
		sed -i.bak -e 's/\/\/go:build.*/\/\/ :build with debug functions/' -e 's/\/\/ +build.*/\/\/ build with debug functions/' internal/dump/dump.go; \
	else \
		sed -i.bak -e 's/\/\/ :build.*/\/\/go:build ignore/' -e 's/\/\/ build.*/\/\/ +build ignore/' internal/dump/dump.go; \
	fi
	rm -f internal/dump/dump.go.bak

build: vendor
	set -e; for CMD in $(CMDS); do \
		cd ./cmd/$$CMD && CGO_ENABLED=0 go build -ldflags "-s -w -X main.Build=$(BUILD) -X main.Revision=$(REVISION)" -o ../../$$CMD; cd ../..; \
	done

# run build watch, ex. with tracing: make build-watch -- -vv
build-watch: vendor
	ls *.go cmd/*/*.go pkg/*/*.go ./internal/*/*.go snclient.ini | entr -sr "$(MAKE) && ./snclient $(filter-out $@,$(MAKECMDGOALS))"

build-linux-amd64: vendor
	set -e; for CMD in $(CMDS); do \
		cd ./cmd/$$CMD && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w -X main.Build=$(BUILD) -X main.Revision=$(REVISION)" -o ../../$$CMD.linux.amd64; cd ../..; \
	done

build-windows-i386: vendor
	set -e; for CMD in $(CMDS); do \
		cd ./cmd/$$CMD && GOOS=windows GOARCH=386 CGO_ENABLED=0 go build -ldflags "-s -w -X main.Build=$(BUILD) -X main.Revision=$(REVISION)" -o ../../$$CMD.windows.i386.exe; cd ../..; \
	done

build-windows-amd64: vendor
	set -e; for CMD in $(CMDS); do \
		cd ./cmd/$$CMD && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w -X main.Build=$(BUILD) -X main.Revision=$(REVISION)" -o ../../$$CMD.windows.amd64.exe; cd ../..; \
	done

test: fmt dump vendor
	go test -short -v -timeout=1m ./ ./pkg/*/. ./internal/*/.
	if grep -rn TODO: *.go ./cmd/ ./pkg/ ./internal/; then exit 1; fi
	if grep -rn Dump *.go ./cmd/ ./pkg/ ./internal/ | grep -v dump.go | grep -v DumpRe; then exit 1; fi

longtest: fmt dump vendor
	go test -v -timeout=1m ./ ./pkg/*/. ./internal/*/.

citest: vendor
	#
	# Checking gofmt errors
	#
	if [ $$(gofmt -s -l *.go ./cmd/ ./pkg/ ./internal/  | wc -l) -gt 0 ]; then \
		echo "found format errors in these files:"; \
		gofmt -s -l *.go ./cmd/ ./pkg/ ./internal/; \
		exit 1; \
	fi
	#
	# Checking TODO items
	#
	if grep -rn TODO: *.go ./cmd/ ./pkg/ ./internal/; then exit 1; fi
	#
	# Checking remaining debug calls
	#
	if grep -rn Dump *.go ./cmd/ ./pkg/ ./internal/ | grep -v dump.go | grep -v DumpRe; then exit 1; fi
	#
	# Darwin and Linux should be handled equal
	#
	diff snclient_linux.go snclient_darwin.go
	diff snclient_linux.go snclient_freebsd.go
	#
	# Run other subtests
	#
	$(MAKE) golangci
	-$(MAKE) govulncheck
	$(MAKE) fmt
	#
	# Normal test cases
	#
	go test -v -timeout=1m ./ ./pkg/*/. ./internal/*/.
	#
	# Benchmark tests
	#
	go test -v -timeout=1m -bench=B\* -run=^$$ -benchmem ./ ./pkg/*/. ./internal/*/.
	#
	# Race rondition tests
	#
	$(MAKE) racetest
	#
	# Test cross compilation
	#
	$(MAKE) build-linux-amd64
	$(MAKE) build-windows-amd64
	$(MAKE) build-windows-i386
	#
	# All CI tests successful
	#
	go mod tidy

benchmark: fmt
	go test -timeout=1m -ldflags "-s -w -X main.Build=$(BUILD)" -v -bench=B\* -run=^$$ -benchmem ./ ./pkg/*/. ./internal/*/.

racetest: fmt
	go test -race -v -timeout=3m -coverprofile=coverage.txt -covermode=atomic ./ ./pkg/*/. ./internal/*/.

covertest: fmt
	go test -v -coverprofile=cover.out -timeout=1m ./ ./pkg/*/. ./internal/*/.
	go tool cover -func=cover.out
	go tool cover -html=cover.out -o coverage.html

coverweb: fmt
	go test -v -coverprofile=cover.out -timeout=1m ./ ./pkg/*/. ./internal/*/.
	go tool cover -html=cover.out

clean:
	set -e; for CMD in $(CMDS); do \
		rm -f ./cmd/$$CMD/$$CMD; \
	done
	rm -f $(CMDS)
	rm -f *.windows.*.exe
	rm -f *.linux.*
	rm -f cover.out
	rm -f coverage.html
	rm -f coverage.txt
	rm -rf vendor/
	rm -rf $(TOOLSFOLDER)
	rm -rf dist/
	rm -rf build-deb/
	rm -rf build-rpm/

fmt: tools
	goimports -w *.go ./cmd/ ./pkg/ ./internal/
	go vet -all -assign -atomic -bool -composites -copylocks -nilfunc -rangeloops -unsafeptr -unreachable .
	set -e; for CMD in $(CMDS); do \
		go vet -all -assign -atomic -bool -composites -copylocks -nilfunc -rangeloops -unsafeptr -unreachable ./cmd/$$CMD; \
	done
	set -e; for dir in $(shell ls -d1 pkg/*); do \
		go vet -all -assign -atomic -bool -composites -copylocks -nilfunc -rangeloops -unsafeptr -unreachable ./$$dir; \
	done
	set -e; for dir in $(shell ls -d1 internal/*); do \
		go vet -all -assign -atomic -bool -composites -copylocks -nilfunc -rangeloops -unsafeptr -unreachable ./$$dir; \
	done
	gofmt -w -s *.go ./cmd/ ./pkg/ ./internal/
	./tools/gofumpt -w *.go ./cmd/ ./pkg/ ./internal/
	./tools/gci write *.go ./cmd/. ./pkg/. ./internal/.  --skip-generated

versioncheck:
	@[ $$( printf '%s\n' $(GOVERSION) $(MINGOVERSION) | sort | head -n 1 ) = $(MINGOVERSION) ] || { \
		echo "**** ERROR:"; \
		echo "**** SNClient+ requires at least golang version $(MINGOVERSIONSTR) or higher"; \
		echo "**** this is: $$(go version)"; \
		exit 1; \
	}

golangci: tools
	#
	# golangci combines a few static code analyzer
	# See https://github.com/golangci/golangci-lint
	#
	golangci-lint run ./... ./pkg/*/. ./internal/*/.

govulncheck: tools
	govulncheck ./...

version:
	OLDVERSION="$(shell grep "VERSION =" ./snclient.go | awk '{print $$3}' | tr -d '"')"; \
	NEWVERSION=$$(dialog --stdout --inputbox "New Version:" 0 0 "v$$OLDVERSION") && \
		NEWVERSION=$$(echo $$NEWVERSION | sed "s/^v//g"); \
		if [ "v$$OLDVERSION" = "v$$NEWVERSION" -o "x$$NEWVERSION" = "x" ]; then echo "no changes"; exit 1; fi; \
		sed -i -e 's/VERSION =.*/VERSION = "'$$NEWVERSION'"/g' *.go cmd/*/*.go

dist:
	mkdir -p ./dist
	openssl req -newkey rsa:2048 -nodes -keyout dist/server.key -out dist/server.csr -subj "/CN=localhost"
	openssl x509 -req -days 3650 -in dist/server.csr -signkey dist/server.key -out dist/server.crt
	openssl req -nodes -new -x509 -keyout dist/ca.key -out dist/ca.crt -days 3650 -subj "/CN=localhost"
	cat dist/ca.crt dist/ca.key > dist/cacert.pem
	rm -f dist/ca.crt dist/ca.key dist/server.csr
	cp \
		./README.md \
		./LICENSE \
		./packaging/snclient.ini \
		./packaging/snclient.logrotate \
		./dist/
	[ -f snclient ] || $(MAKE) build
	if [ "$(GOOS)" = "windows" ]; then cp ./snclient -p ./dist/snclient.exe; else cp -p ./snclient ./dist/snclient; fi
	chmod u+x ./snclient
	-help2man --no-info --section=8 --version-string="snclient $(VERSION)" \
		--help-option=-h --include=./packaging/help2man.include \
		-n "Agent that runs and provides system checks and metrics." \
		./snclient \
		> dist/snclient.8

snclient: build

deb: | dist
	mkdir -p \
		build-deb/etc/snclient \
		build-deb/usr/bin \
		build-deb/lib/systemd/system \
		build-deb/etc/logrotate.d \
		build-deb/usr/share/doc/snclient \
		build-deb/usr/share/doc/snclient

	rm -rf ./build-deb/DEBIAN
	cp -r ./packaging/debian ./build-deb/DEBIAN
	cp ./dist/snclient.ini ./dist/server.crt ./dist/server.key ./dist/cacert.pem ./build-deb/etc/snclient
	cp -p ./dist/snclient build-deb/usr/bin/snclient
	cp ./packaging/snclient.service build-deb/lib/systemd/system/
	cp ./packaging/snclient.logrotate build-deb/etc/logrotate.d/snclient
	rm -f build-deb/usr/share/doc/snclient/changelog.gz
	cp Changes build-deb/usr/share/doc/snclient/changelog
	gzip -n -9 build-deb/usr/share/doc/snclient/changelog

	cp ./dist/LICENSE build-deb//usr/share/doc/snclient/copyright
	cp ./dist/README.md build-deb//usr/share/doc/snclient/README

	sed -i build-deb/DEBIAN/control -e 's|^Architecture: .*|Architecture: $(ARCH)|'
	sed -i build-deb/DEBIAN/control -e 's|^Architecture: 386|Architecture: i386|'
	sed -i build-deb/DEBIAN/control -e 's|^Version: .*|Version: $(VERSION)|'

	chmod 644 build-deb/etc/snclient/*

	dpkg-deb --build --root-owner-group ./build-deb ./$(DEBFILE)
	-lintian ./$(DEBFILE)

rpm: | dist
	rm -rf snclient-$(VERSION)
	cp ./packaging/snclient.service dist/
	cp ./packaging/snclient.spec dist/
	sed -i dist/snclient.spec -e 's|^Version: .*|Version: $(VERSION)|'
	sed -i dist/snclient.spec -e 's|^BuildArch: .*|BuildArch: $(ARCH)|'
	cp -rp dist snclient-$(VERSION)
	tar cfz snclient-$(VERSION).tar.gz snclient-$(VERSION)
	rm -rf snclient-$(VERSION)
	mkdir -p $(RPM_TOPDIR)/{SOURCES,BUILD,RPMS,SRPMS,SPECS}
	cp snclient-$(VERSION).tar.gz $(RPM_TOPDIR)/SOURCES
	if [ $(ARCH) = "386" ]; then \
		RPM_ARCH=i386; \
	elif [ $(ARCH) = "amd64" ]; then \
		RPM_ARCH=x86_64; \
	elif [ $(ARCH) = "arm64" ]; then \
		RPM_ARCH=aarch64; \
	fi; \
	rpmbuild \
		--target $$RPM_ARCH \
		--define "_topdir $(RPM_TOPDIR)" \
		--buildroot=$(shell pwd)/build-rpm \
		-bb dist/snclient.spec; \
	mv $(RPM_TOPDIR)/RPMS/*/snclient-*.rpm snclient-$(VERSION)-$(BUILD)-$$RPM_ARCH.rpm
	rm -rf $(RPM_TOPDIR) build-rpm
	-rpmlint -f packaging/rpmlintrc snclient-$(VERSION)-$(BUILD)-$$RPM_ARCH.rpm
