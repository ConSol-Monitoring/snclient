#!/usr/bin/make -f

PROJECT=snclient
MAKE:=make
SHELL:=bash
GOVERSION:=$(shell \
    go version | \
    awk -F'go| ' '{ split($$5, a, /\./); printf ("%04d%04d", a[1], a[2]); exit; }' \
)
# also update docs/install/build.md and .github/workflows/builds.yml when changing minimum version
# find . -name go.mod
MINGOVERSION:=00010022
MINGOVERSIONSTR:=1.22
BUILD:=$(shell git rev-parse --short HEAD)
REVISION:=$(shell ./buildtools/get_version | awk -F . '{ print $$3 }')
# see https://github.com/go-modules-by-example/index/blob/master/010_tools/README.md
# and https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
TOOLSFOLDER=$(shell pwd)/tools
export GOBIN := $(TOOLSFOLDER)
export PATH := $(GOBIN):$(PATH)

VERSION ?= $(shell ./buildtools/get_version)
GOARCH  ?= $(shell go env GOARCH)
GOOS    ?= $(shell go env GOOS)

RPM_TOPDIR=$(shell pwd)/rpm.top
RPM_ARCH:=$(GOARCH)
ifeq ($(RPM_ARCH),386)
	RPM_ARCH := i386
endif
ifeq ($(RPM_ARCH),amd64)
	RPM_ARCH := x86_64
endif
ifeq ($(RPM_ARCH),arm64)
	RPM_ARCH := aarch64
endif
RPMFILE ?= snclient-$(VERSION)-$(RPM_ARCH).rpm

ifeq ($(GOARCH),i386)
	export GOARCH := 386
endif
ifeq ($(GOARCH),x86_64)
	export GOARCH := amd64
endif
ifeq ($(GOARCH),aarch64)
	export GOARCH := arm64
endif

DEB_ARCH:=$(GOARCH)
ifeq ($(DEB_ARCH),386)
	DEB_ARCH := i386
endif
DEBFILE ?= snclient-$(VERSION)-$(RPM_ARCH).deb

BUILD_FLAGS=-ldflags "-s -w -X pkg/snclient.Build=$(BUILD) -X pkg/snclient.Revision=$(REVISION)"
TEST_FLAGS=-timeout=5m $(BUILD_FLAGS)

NODE_EXPORTER_VERSION=1.7.0
NODE_EXPORTER_FILE=node_exporter-$(NODE_EXPORTER_VERSION).$(GOOS)-$(GOARCH).tar.gz
NODE_EXPORTER_URL=https://github.com/prometheus/node_exporter/releases/download/v$(NODE_EXPORTER_VERSION)/$(NODE_EXPORTER_FILE)

# last i386 exe is the 0.24.0
WINDOWS_EXPORTER_VERSION_I386=0.24.0
WINDOWS_EXPORTER_FILE_I386=windows_exporter-$(WINDOWS_EXPORTER_VERSION_I386)
WINDOWS_EXPORTER_URL_I386=https://github.com/prometheus-community/windows_exporter/releases/download/v$(WINDOWS_EXPORTER_VERSION_I386)/

WINDOWS_EXPORTER_VERSION=0.25.1
WINDOWS_EXPORTER_FILE=windows_exporter-$(WINDOWS_EXPORTER_VERSION)
WINDOWS_EXPORTER_URL=https://github.com/prometheus-community/windows_exporter/releases/download/v$(WINDOWS_EXPORTER_VERSION)/

SED=sed -i
GO=go
CGO_ENABLED=0

# OSX must be build with CGO enabled, otherwise we don't get cpu metrics
DARWIN=0
ifeq ($(shell uname),Darwin)
  DARWIN=1
  SED=sed -i ""
endif
ifeq ($(GOOS),darwin)
  DARWIN=1
endif
ifeq ($(DARWIN),1)
  # cgo is required to retrieve cpu information
  CGO_ENABLED=1
endif

ENTR=ls cmd/*/*.go pkg/*/*.go pkg/*/*/*.go snclient*.ini | entr -sr

.PHONY=docs

all: build

CMDS = $(shell cd ./cmd && ls -1)

tools: | versioncheck
	set -e; for DEP in $(shell grep "_ " buildtools/tools.go | awk '{ print $$2 }' | grep -v pkg/dump); do \
		( cd buildtools && $(GO) install $$DEP@latest ) ; \
	done
	( cd buildtools && $(GO) mod tidy )

updatedeps: versioncheck
	$(MAKE) clean
	$(MAKE) tools
	$(GO) mod download
	set -e; for dir in $(shell ls -d1 pkg/*); do \
		( cd ./$$dir && $(GO) mod download ); \
		( cd ./$$dir && GOPROXY=direct $(GO) get -u ); \
		( cd ./$$dir && GOPROXY=direct $(GO) get -t -u ); \
	done
	GOPROXY=direct $(GO) get -u ./t/
	$(GO) mod download
	$(MAKE) cleandeps

cleandeps:
	set -e; for dir in $(shell ls -d1 pkg/* pkg/snclient/commands cmd/*); do \
		( cd ./$$dir && $(GO) mod tidy ); \
	done
	$(GO) mod tidy
	( cd buildtools && $(GO) mod tidy )

vendor: go.work
	$(GO) mod download
	$(GO) mod tidy
	GOWORK=off $(GO) mod vendor

go.work: pkg/*
	echo "go $(MINGOVERSIONSTR)" > go.work
	$(GO) work use \
		. \
		pkg/* \
		pkg/snclient/commands \
		cmd/* \
		buildtools/. \
		t/. \

build: vendor snclient.ini server.crt server.key
	set -e; for CMD in $(CMDS); do \
		( cd ./cmd/$$CMD && CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(BUILD_FLAGS) -o ../../$$CMD ) ; \
	done

# run build watch, ex. with tracing: make build-watch -- -vv -logfile stderr
build-watch: vendor tools
	$(ENTR) "$(MAKE) && ./snclient $(filter-out $@,$(MAKECMDGOALS))"

# run build watch with other build targets, ex.: make build-watch-make -- build-windows-amd64
build-watch-make: vendor tools
	$(ENTR) "$(MAKE) $(filter-out $@,$(MAKECMDGOALS))"

# run build watch with any command, ex.: make build-watch-cmd -- "make build-windows-amd64 && cp snclient.exe ..."
build-watch-cmd: vendor tools
	$(ENTR) "$(filter-out $@,$(MAKECMDGOALS))"

build-linux-amd64: vendor
	set -e; for CMD in $(CMDS); do \
		( cd ./cmd/$$CMD && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(BUILD_FLAGS) -o ../../$$CMD.linux.amd64 ) ; \
	done

build-linux-i386: vendor
	set -e; for CMD in $(CMDS); do \
		( cd ./cmd/$$CMD && GOOS=linux GOARCH=386 CGO_ENABLED=0 $(GO) build $(BUILD_FLAGS) -o ../../$$CMD.linux.i386 ) ; \
	done

build-windows-i386: vendor rsrc_windows_386.syso
	cp rsrc_windows_386.syso cmd/snclient/
	set -e; for CMD in $(CMDS); do \
		( cd ./cmd/$$CMD && GOOS=windows GOARCH=386 CGO_ENABLED=0 $(GO) build $(BUILD_FLAGS) -o ../../$$CMD.windows.i386.exe ) ; \
	done

build-windows-amd64: vendor rsrc_windows_amd64.syso
	cp rsrc_windows_amd64.syso cmd/snclient/
	set -e; for CMD in $(CMDS); do \
		( cd ./cmd/$$CMD && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(BUILD_FLAGS) -o ../../$$CMD.windows.amd64.exe ) ; \
	done

build-windows-arm64: vendor rsrc_windows_arm64.syso
	cp rsrc_windows_arm64.syso cmd/snclient/
	set -e; for CMD in $(CMDS); do \
		( cd ./cmd/$$CMD && GOOS=windows GOARCH=arm64 CGO_ENABLED=0 $(GO) build $(BUILD_FLAGS) -o ../../$$CMD.windows.arm64.exe ) ; \
	done

build-freebsd-i386: vendor
	set -e; for CMD in $(CMDS); do \
		( cd ./cmd/$$CMD && GOOS=freebsd GOARCH=386 CGO_ENABLED=0 $(GO) build $(BUILD_FLAGS) -o ../../$$CMD.freebsd.i386 ) ; \
	done

build-darwin-aarch64: vendor
	set -e; for CMD in $(CMDS); do \
		( cd ./cmd/$$CMD && GOOS=darwin GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(BUILD_FLAGS) -o ../../$$CMD.darwin.aarch64 ) ; \
	done

winres: | tools
	rm -rf winres
	cp -rp packaging/windows/winres .

rsrc_windows_386.syso: winres | tools
	${TOOLSFOLDER}/go-winres make --arch 386

rsrc_windows_amd64.syso: winres | tools
	${TOOLSFOLDER}/go-winres make --arch amd64

rsrc_windows_arm64.syso: winres | tools
	${TOOLSFOLDER}/go-winres make --arch arm64

test: vendor
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test -short -v $(TEST_FLAGS) pkg/* pkg/*/commands
	if grep -Irn TODO: ./cmd/ ./pkg/ ./packaging/ ; then exit 1; fi
	if grep -Irn Dump ./cmd/ ./pkg/ | grep -v dump.go | grep -v DumpRe | grep -v ThreadDump; then exit 1; fi

# test with filter
testf: vendor
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test -short -v $(TEST_FLAGS) pkg/* pkg/*/commands -run "$(filter-out $@,$(MAKECMDGOALS))" 2>&1 | grep -v "no test files" | grep -v "no tests to run" | grep -v "^PASS"

longtest: vendor
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test -v $(TEST_FLAGS) pkg/* pkg/*/commands

citest: tools vendor
	#
	# Checking gofmt errors
	#
	if [ $$(gofmt -s -l ./cmd/ ./pkg/ | wc -l) -gt 0 ]; then \
		echo "found format errors in these files:"; \
		gofmt -s -l ./cmd/ ./pkg/ ; \
		exit 1; \
	fi
	#
	# Checking TODO items
	#
	if grep -Irn TODO: ./cmd/ ./pkg/ ./packaging/ ; then exit 1; fi
	#
	# Checking remaining debug calls
	#
	if grep -Irn Dump ./cmd/ ./pkg/ | grep -v dump.go | grep -v DumpRe | grep -v ThreadDump; then exit 1; fi
	#
	# Run other subtests
	#
	$(MAKE) golangci
	-$(MAKE) govulncheck
	$(MAKE) fmt
	#
	# Normal test cases
	#
	$(MAKE) test
	$(MAKE) -C t test
	#
	# Benchmark tests
	#
	$(MAKE) benchmark
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
	$(MAKE) build-freebsd-i386
	$(MAKE) build-darwin-aarch64
	#
	# Check docs are updated
	#
	$(MAKE) docs
	$(MAKE) gitcleandocs
	$(MAKE) docs_test
	#
	# All CI tests successful
	#

benchmark:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test $(TEST_FLAGS) -v -bench=B\* -run=^$$ -benchmem ./pkg/* pkg/*/commands

racetest:
	# go: -race requires cgo, so do not use the macro here
	$(GO) test -race $(TEST_FLAGS) -coverprofile=coverage.txt -covermode=atomic ./pkg/* pkg/*/commands

covertest:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test -v $(TEST_FLAGS) -coverprofile=cover.out ./pkg/* pkg/*/commands
	$(GO) tool cover -func=cover.out
	$(GO) tool cover -html=cover.out -o coverage.html

coverweb:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test -v $(TEST_FLAGS) -coverprofile=cover.out ./pkg/* pkg/*/commands
	$(GO) tool cover -html=cover.out

clean:
	set -e; for CMD in $(CMDS); do \
		rm -f ./cmd/$$CMD/$$CMD; \
	done
	rm -f $(CMDS)
	rm -f *.windows.*.exe
	rm -f *.linux.*
	rm -f *.darwin.*
	rm -f *.freebsd.*
	rm -f go.work
	rm -f go.work.sum
	rm -f cover.out
	rm -f coverage.html
	rm -f coverage.txt
	rm -rf vendor/
	rm -rf $(TOOLSFOLDER)
	rm -rf dist/
	rm -rf windist/
	rm -rf build-deb/
	rm -rf build-rpm/
	rm -f release_notes.txt
	rm -rf winres
	rm -f rsrc_windows*.syso
	rm -rf cmd/snclient/rsrc_windows*.syso
	rm -f node_exporter-*.tar.gz
	$(MAKE) -C docs clean

GOVET=$(GO) vet -all
SRCFOLDER=./cmd/. ./pkg/. ./t/. ./buildtools/.
fmt: tools
	set -e; for CMD in $(CMDS); do \
		$(GOVET) ./cmd/$$CMD; \
	done
	set -e; for dir in $(shell ls -d1 pkg/* t/); do \
		$(GOVET) ./$$dir; \
	done
	gofmt -w -s $(SRCFOLDER)
	./tools/gofumpt -w $(SRCFOLDER)
	./tools/gci write --skip-generated $(SRCFOLDER)
	./tools/goimports -w $(SRCFOLDER)

versioncheck:
	@[ $$( printf '%s\n' $(GOVERSION) $(MINGOVERSION) | sort | head -n 1 ) = $(MINGOVERSION) ] || { \
		echo "**** ERROR:"; \
		echo "**** $(PROJECT) requires at least golang version $(MINGOVERSIONSTR) or higher"; \
		echo "**** this is: $$(go version)"; \
		exit 1; \
	}

golangci: tools
	#
	# golangci combines a few static code analyzer
	# See https://github.com/golangci/golangci-lint
	#
	@set -e; for dir in $$(ls -1d pkg/* pkg/snclient/commands cmd t); do \
		echo $$dir; \
		if [ $$dir != "pkg/eventlog" ]; then \
			echo "  - GOOS=linux"; \
			( cd $$dir && GOOS=linux CGO_ENABLED=0 golangci-lint run --timeout=5m ./... ); \
			echo "  - GOOS=darwin"; \
			( cd $$dir && GOOS=darwin CGO_ENABLED=$(CGO_ENABLED) golangci-lint run --timeout=5m ./... ); \
			echo "  - GOOS=freebsd"; \
			( cd $$dir && GOOS=freebsd CGO_ENABLED=0 golangci-lint run --timeout=5m ./... ); \
		fi; \
		echo "  - GOOS=windows"; \
		( cd $$dir && GOOS=windows CGO_ENABLED=0 golangci-lint run --timeout=5m ./... ); \
	done

govulncheck: tools
	govulncheck ./...

version:
	OLDVERSION="$(shell grep "VERSION =" ./pkg/$(PROJECT)/snclient.go | awk '{print $$3}' | tr -d '"')"; \
	NEWVERSION=$$(dialog --stdout --inputbox "New Version:" 0 0 "v$$OLDVERSION") && \
		NEWVERSION=$$(echo $$NEWVERSION | sed "s/^v//g"); \
		if [ "v$$OLDVERSION" = "v$$NEWVERSION" -o "x$$NEWVERSION" = "x" ]; then echo "no changes"; exit 1; fi; \
		sed -i -e 's/VERSION =.*/VERSION = "'$$NEWVERSION'"/g' cmd/*/*.go pkg/$(PROJECT)/*.go

dist:
	mkdir -p ./dist
	openssl genrsa -out dist/ca.key 4096
	openssl req -key dist/ca.key -new -x509 -days 20000 -sha256 -extensions v3_ca -out dist/cacert.pem -subj "/C=DE/ST=Bavaria/L=Earth/O=snclient/OU=IT/CN=Root CA SNClient"
	openssl req -newkey rsa:2048 -nodes -keyout dist/server.key -out dist/server.csr -subj "/CN=snclient" -reqexts SAN -extensions SAN -config <(echo -e "[req]\ndistinguished_name=req\n[SAN]\nsubjectAltName=DNS:snclient")
	openssl x509 -req -CAcreateserial -CA dist/cacert.pem -CAkey dist/ca.key -days 20000 -in dist/server.csr -out dist/server.crt
	rm -f dist/server.csr dist/cacert.srl
	cp \
		./README.md \
		./LICENSE \
		./packaging/snclient.ini \
		./packaging/snclient.logrotate \
		./dist/
	[ -f snclient ] || $(MAKE) build
	if [ "$(GOOS)" = "windows" ]; then cp ./snclient -p ./dist/snclient.exe; else cp -p ./snclient ./dist/snclient; fi
	chmod u+x ./snclient
	-help2man --no-info --section=1 --version-string="snclient $(VERSION)" \
		--help-option=-h --include=./packaging/help2man.include \
		-n "Agent that runs and provides system checks and metrics." \
		./snclient \
		> dist/snclient.1
	-help2man --no-info --section=8 --version-string="snclient $(VERSION)" \
		--help-option=-h --include=./packaging/help2man.include \
		-n "Agent that runs and provides system checks and metrics." \
		./snclient \
		> dist/snclient.8

windist: | dist
	rm -f windist
	mkdir windist
	cp -p dist/cacert.pem \
		dist/server.crt \
		dist/server.key \
		dist/snclient.ini \
		windist/
	# create LICENSE.rtf
	echo '{\rtf1\ansi\deff0\nouicompat{\fonttbl{\f0\fnil\fcharset0 Courier New;}}' > windist/LICENSE.rtf
	echo '\pard\f0\fs22\lang1033' >> windist/LICENSE.rtf
	while read line; do printf "%s\n" "$$line\par"; done < LICENSE >> windist/LICENSE.rtf

	test -f windist/windows_exporter-386.exe   || curl -s -L -o windist/windows_exporter-386.exe   $(WINDOWS_EXPORTER_URL_I386)/$(WINDOWS_EXPORTER_FILE_I386)-386.exe
	test -f windist/windows_exporter-amd64.exe || curl -s -L -o windist/windows_exporter-amd64.exe $(WINDOWS_EXPORTER_URL)/$(WINDOWS_EXPORTER_FILE)-amd64.exe
	test -f windist/windows_exporter-arm64.exe || curl -s -L -o windist/windows_exporter-arm64.exe $(WINDOWS_EXPORTER_URL)/$(WINDOWS_EXPORTER_FILE)-arm64.exe
	cd windist && shasum --ignore-missing -c ../packaging/sha256sums.txt

	$(SED) \
		-e 's/\/etc\/snclient/${exe-path}/g' \
		-e 's/^file name =.*/file name = $${shared-path}\/snclient.log/g' \
		-e 's/^max size =.*/max size = 10MiB/g' \
		windist/snclient.ini
	todos windist/snclient.ini


snclient: build snclient.ini

snclient.ini:
	cp packaging/snclient.ini .
	$(SED) \
		-e 's/^shared\-path =.*/shared\-path = ./g' \
		-e 's/^file name =.*/file name = .\/snclient.log/g' \
		./snclient.ini

server.crt: | dist
	cp dist/server.crt .

server.key: | dist
	cp dist/server.key .

deb: | dist
	mkdir -p \
		build-deb/etc/snclient \
		build-deb/usr/lib/snclient \
		build-deb/usr/bin \
		build-deb/lib/systemd/system \
		build-deb/etc/logrotate.d \
		build-deb/usr/share/doc/snclient \
		build-deb/usr/share/doc/snclient \
		build-deb/usr/share/man/man1 \
		build-deb/usr/share/man/man8 \
		build-deb/usr/share/lintian/overrides/

	test -f $(NODE_EXPORTER_FILE) || curl -s -L -O $(NODE_EXPORTER_URL)
	shasum --ignore-missing -c packaging/sha256sums.txt
	tar zxvf $(NODE_EXPORTER_FILE)
	mv node_exporter-$(NODE_EXPORTER_VERSION).linux-$(GOARCH)/node_exporter build-deb/usr/lib/snclient/node_exporter
	rm -rf node_exporter-$(NODE_EXPORTER_VERSION).linux-$(GOARCH)

	rm -rf ./build-deb/DEBIAN
	cp -r ./packaging/debian ./build-deb/DEBIAN
	cp ./dist/snclient.ini ./dist/server.crt ./dist/server.key ./dist/cacert.pem ./build-deb/etc/snclient
	cp -p ./dist/snclient build-deb/usr/bin/snclient
	cp ./packaging/snclient.service build-deb/lib/systemd/system/
	cp ./packaging/snclient.logrotate build-deb/etc/logrotate.d/snclient
	cp Changes build-deb/usr/share/doc/snclient/Changes
	dch --empty --create --newversion "$(VERSION)" --package "snclient" -D "UNRELEASED" --urgency "low" -c build-deb/usr/share/doc/snclient/changelog "new upstream release"
	rm -f build-deb/usr/share/doc/snclient/changelog.gz
	gzip -9 build-deb/usr/share/doc/snclient/changelog

	cp ./dist/LICENSE build-deb//usr/share/doc/snclient/copyright
	cp ./dist/README.md build-deb//usr/share/doc/snclient/README
	mv ./build-deb/DEBIAN/snclient.lintian-overrides build-deb/usr/share/lintian/overrides/snclient

	sed -i build-deb/DEBIAN/control -e 's|^Architecture: .*|Architecture: $(DEB_ARCH)|'
	sed -i build-deb/DEBIAN/control -e 's|^Version: .*|Version: $(VERSION)|'

	chmod 644 build-deb/etc/snclient/*
	chmod 755 \
		build-deb/usr/bin/snclient \
		build-deb/usr/lib/snclient/node_exporter

	cp -p dist/snclient.1 build-deb/usr/share/man/man1/snclient.1
	gzip -n -9 build-deb/usr/share/man/man1/snclient.1
	cp -p dist/snclient.8 build-deb/usr/share/man/man8/snclient.8
	gzip -n -9 build-deb/usr/share/man/man8/snclient.8

	dpkg-deb -Zxz --build --root-owner-group ./build-deb ./$(DEBFILE)
	rm -rf ./build-deb
	-( cd packaging && lintian ../$(DEBFILE) )

rpm: | dist
	rm -rf snclient-$(VERSION)
	cp ./packaging/snclient.service dist/
	cp ./packaging/snclient.spec dist/
	sed -i dist/snclient.spec -e 's|^Version: .*|Version: $(VERSION)|'
	sed -i dist/snclient.spec -e 's|^BuildArch: .*|BuildArch: $(RPM_ARCH)|'
	cp -rp dist snclient-$(VERSION)
	rm -f snclient-$(VERSION)/ca.key

	test -f $(NODE_EXPORTER_FILE) || curl -s -L -O $(NODE_EXPORTER_URL)
	shasum --ignore-missing -c packaging/sha256sums.txt
	tar zxvf $(NODE_EXPORTER_FILE)
	mv node_exporter-$(NODE_EXPORTER_VERSION).linux-$(GOARCH)/node_exporter snclient-$(VERSION)/node_exporter
	rm -rf node_exporter-$(NODE_EXPORTER_VERSION).linux-$(GOARCH)

	chmod 755 \
		snclient-$(VERSION)/snclient \
		snclient-$(VERSION)/node_exporter

	tar cfz snclient-$(VERSION).tar.gz snclient-$(VERSION)
	rm -rf snclient-$(VERSION)
	mkdir -p $(RPM_TOPDIR)/{SOURCES,BUILD,RPMS,SRPMS,SPECS}
	mv snclient-$(VERSION).tar.gz $(RPM_TOPDIR)/SOURCES
	rpmbuild \
		--target $(RPM_ARCH) \
		--define "_topdir $(RPM_TOPDIR)" \
		--buildroot=$(shell pwd)/build-rpm \
		-bb dist/snclient.spec
	mv $(RPM_TOPDIR)/RPMS/*/snclient-*.rpm $(RPMFILE)
	rm -rf $(RPM_TOPDIR) build-rpm
	-rpmlint -f packaging/rpmlintrc $(RPMFILE)

osx: | dist
	rm -rf build-pkg

	mkdir -p \
		build-pkg/Library/LaunchDaemons \
		build-pkg/usr/local/bin \
		build-pkg/etc/snclient \
		build-pkg/usr/local/share/man/man1 \
		build-pkg/usr/local/share/man/man8

	test -f $(NODE_EXPORTER_FILE) || curl -s -L -O $(NODE_EXPORTER_URL)
	shasum --ignore-missing -c packaging/sha256sums.txt
	tar zxvf $(NODE_EXPORTER_FILE)
	mv node_exporter-$(NODE_EXPORTER_VERSION).darwin-$(GOARCH)/node_exporter build-pkg/usr/local/bin/node_exporter
	chmod 755 build-pkg/usr/local/bin/node_exporter
	rm -rf node_exporter-$(NODE_EXPORTER_VERSION).darwin-$(GOARCH)

	cp packaging/osx/com.snclient.snclient.plist build-pkg/Library/LaunchDaemons/

	cp dist/snclient build-pkg/usr/local/bin/
	chmod 755 build-pkg/usr/local/bin/snclient

	cp packaging/osx/uninstall.sh build-pkg/usr/local/bin/snclient_uninstall.sh
	chmod 755 build-pkg/usr/local/bin/snclient_uninstall.sh

	cp dist/snclient.ini dist/server.crt dist/server.key dist/cacert.pem build-pkg/etc/snclient

	$(SED) \
		-e 's/^max size =.*/max size = 10MiB/g' \
		-e 's|/usr/lib/snclient/node_exporter|/usr/local/bin/node_exporter|g' \
		build-pkg/etc/snclient/snclient.ini

	cp -p dist/snclient.1 build-pkg/usr/local/share/man/man1/snclient.1
	cp -p dist/snclient.8 build-pkg/usr/local/share/man/man8/snclient.8

	pkgbuild --root "build-pkg" \
			--identifier com.snclient.snclient \
			--version $(VERSION) \
			--install-location / \
			--scripts packaging/osx/. \
			"snclient-$(VERSION).pkg"
	rm -rf build-pkg


release:
	./buildtools/release.sh

release_notes.txt: Changes
	echo "Changes:" >> release_notes.txt
	echo '```' >> release_notes.txt
	# changes start with 4rd line until first empty line
	tail -n +4 Changes | sed '/^$$/,$$d' | sed -e 's/^         //g' >> release_notes.txt
	echo '```' >> release_notes.txt

release_blog.md: release_notes.txt
	$(MAKE) release_blog_text | grep -v '^make' > release_blog.md

release_blog_text: release_notes.txt
	@echo '---'
	@echo 'date: $(shell date +"%Y-%m-%d")T00:00:00.000Z'
	@echo 'title: "SNClient+ $(VERSION) was released"'
	@echo 'linkTitle: "SNClient+ $(VERSION)"'
	@echo 'tags:'
	@echo '  - windows'
	@echo '  - linux'
	@echo '  - nsclient'
	@echo '  - snclient'
	@echo '---'
	@echo 'A new version of SNClient+ was released.'
	@echo ''
	@echo '### Breaking Changes'
	@echo ''
	@echo '* none'
	@echo ''
	@echo '### Features'
	@echo ''
	@grep "^- add" release_notes.txt | \
	while read LINE; do \
		echo $$LINE | sed 's/^- /* /'; \
	done
	@echo ''
	@echo '### Changed'
	@echo ''
	@grep "^- " release_notes.txt | grep -v "^- add" | grep -v "^- fix" | \
	while read LINE; do \
		echo $$LINE | sed 's/^- /* /'; \
	done
	@echo ''
	@echo '### Bugfixes'
	@echo ''
	@grep "^- fix" release_notes.txt | \
	while read LINE; do \
		echo $$LINE | sed 's/^- /* /'; \
	done
	@echo ''
	@echo '### Download'
	@echo ''
	@echo '<https://github.com/ConSol-Monitoring/snclient/releases/tag/v$(VERSION)>'


# just skip unknown make targets
.DEFAULT:
	@if [[ "$(MAKECMDGOALS)" =~ ^testf ]]; then \
		: ; \
	else \
		echo "unknown make target(s): $(MAKECMDGOALS)"; \
		exit 1; \
	fi

client1.pem:
	openssl req -new -nodes -x509 -out client1.pem -keyout client1.key -days 20000 -subj "/C=DE/ST=Bavaria/L=Earth/O=SNClient/OU=IT"

# selfsigned certificate for code signing
sign.pfx:
	mkdir sign
	openssl genrsa -out sign/ca.key 4096
	openssl req -key sign/ca.key -new -x509 -days 20000 -sha256 -out sign/cacert.pem -subj "/C=DE/ST=Bavaria/L=Munich/O=snclient/OU=IT/CN=Root CA SNClient Code Sign"
	openssl req -newkey rsa:2048 -nodes -keyout sign/sign.key -out sign/sign.csr -subj "/CN=snclient"
	openssl x509 -req -CAcreateserial -CA sign/cacert.pem -CAkey sign/ca.key -days 20000 -in sign/sign.csr -out sign/sign.crt
	rm -f sign/sign.csr sign/sign.srl
	openssl pkcs12 -export -out sign.pfx -inkey sign/sign.key -in sign/sign.crt

sign.pfx_sha1: sign.pfx
	openssl x509 -fingerprint -in sign.pfx -noout | tr -d ':'

DOC_COMMANDS=\
	check_connections \
	check_cpu \
	check_cpu_utilization \
	check_dummy \
	check_drivesize \
	check_eventlog \
	check_files \
	check_index \
	check_kernel_stats \
	check_load \
	check_mailq \
	check_memory \
	check_mount \
	check_network \
	check_ntp_offset \
	check_omd \
	check_os_version \
	check_os_updates \
	check_pagefile \
	check_process \
	check_snclient_version \
	check_tasksched \
	check_temperature \
	check_uptime \
	check_wmi \

docs: build
	set -e; \
	for CHK in $(DOC_COMMANDS); do \
		echo "updating docs/checks/commands/$$CHK.md"; \
		./snclient -logfile stderr run $$CHK help=md > docs/checks/commands/$$CHK.md ; \
	done
	if [ $(GOOS) = "linux" ]; then \
		./snclient -logfile stderr run check_service help=md > docs/checks/commands/check_service_linux.md ; \
		sed '/^func.*Build/,/^\}/d' -i pkg/snclient/check_service_linux.go; \
		sed '1,/^func.*Build/d' -i pkg/snclient/check_service_windows.go; \
		sed '/^\}/,$$d' -i pkg/snclient/check_service_windows.go; \
		echo "func (l *CheckService) Build() *CheckData {" >> pkg/snclient/check_service_linux.go ; \
		cat pkg/snclient/check_service_windows.go >> pkg/snclient/check_service_linux.go ; \
		echo "}" >> pkg/snclient/check_service_linux.go ; \
		rm pkg/snclient/check_service_windows.go; \
		$(MAKE) build ; \
		./snclient -logfile stderr run check_service help=md > docs/checks/commands/check_service_windows.md ; \
		git checkout pkg/snclient/check_service_windows.go pkg/snclient/check_service_linux.go; \
	fi

docs_server:
	$(MAKE) -C docs server
	$(MAKE) -C docs clean

docs_test:
	$(MAKE) -C docs test
	$(MAKE) -C docs clean

# ensure docs folder is clean
gitcleandocs:
	@if [ $$(git status -s docs | wc -l) -gt 0 ]; then \
		echo "docs folder not clean:"; \
		git status -s docs; \
		exit 1; \
	fi
