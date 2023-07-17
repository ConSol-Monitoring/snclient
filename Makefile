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
REVISION:=$(shell ./buildtools/get_version | awk -F . '{ print $$3 }')
# see https://github.com/go-modules-by-example/index/blob/master/010_tools/README.md
# and https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
TOOLSFOLDER=$(shell pwd)/tools
export GOBIN := $(TOOLSFOLDER)
export PATH := $(GOBIN):$(PATH)

VERSION ?= $(shell ./buildtools/get_version)
ARCH    ?= $(shell go env GOARCH)
DEBFILE ?= snclient-$(VERSION)-$(BUILD)-$(ARCH).deb
DEB_ARCH=$(ARCH)
ifeq ($(DEB_ARCH),386)
	DEB_ARCH=i386
endif
RPM_TOPDIR=$(shell pwd)/rpm.top
RPM_ARCH=$(ARCH)
ifeq ($(RPM_ARCH),386)
	RPM_ARCH=i386
endif
ifeq ($(RPM_ARCH),amd64)
	RPM_ARCH=x86_64
endif
ifeq ($(RPM_ARCH),arm64)
	RPM_ARCH=aarch64
endif

BUILD_FLAGS=-ldflags "-s -w -X pkg/snclient.Build=$(BUILD) -X pkg/snclient.Revision=$(REVISION)"
TEST_FLAGS=-timeout=3m $(BUILD_FLAGS)

all: build

CMDS = $(shell cd ./cmd && ls -1)

tools: versioncheck vendor go.work
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

vendor: go.work
	go mod download
	go mod tidy
	go mod vendor

go.work: pkg/*
	echo "go $(MINGOVERSIONSTR)" > go.work
	go work use . pkg/* pkg/*/*/.

build: vendor go.work snclient.ini server.crt server.key
	set -xe; for CMD in $(CMDS); do \
		( cd ./cmd/$$CMD && CGO_ENABLED=0 go build -trimpath $(BUILD_FLAGS) -o ../../$$CMD ) ; \
	done

# run build watch, ex. with tracing: make build-watch -- -vv
build-watch: vendor
	ls cmd/*/*.go pkg/*/*.go pkg/*/*/*.go snclient.ini | entr -sr "$(MAKE) && ./snclient $(filter-out $@,$(MAKECMDGOALS))"

# run build watch with other build targets, ex.: make build-watch-make -- build-windows-amd64
build-watch-make: vendor
	ls cmd/*/*.go pkg/*/*.go pkg/*/*/*.go snclient.ini | entr -sr "$(MAKE) $(filter-out $@,$(MAKECMDGOALS))"

build-linux-amd64: vendor
	set -e; for CMD in $(CMDS); do \
		( cd ./cmd/$$CMD && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath $(BUILD_FLAGS) -o ../../$$CMD.linux.amd64 ) ; \
	done

build-linux-i386: vendor
	set -e; for CMD in $(CMDS); do \
		( cd ./cmd/$$CMD && GOOS=linux GOARCH=386 CGO_ENABLED=0 go build -trimpath $(BUILD_FLAGS) -o ../../$$CMD.linux.i386 ) ; \
	done

build-windows-i386: vendor
	set -e; for CMD in $(CMDS); do \
		( cd ./cmd/$$CMD && GOOS=windows GOARCH=386 CGO_ENABLED=0 go build -trimpath $(BUILD_FLAGS) -o ../../$$CMD.windows.i386.exe ) ; \
	done

build-windows-amd64: vendor
	set -e; for CMD in $(CMDS); do \
		( cd ./cmd/$$CMD && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath $(BUILD_FLAGS) -o ../../$$CMD.windows.amd64.exe ) ; \
	done

build-freebsd-i386: vendor
	set -e; for CMD in $(CMDS); do \
		( cd ./cmd/$$CMD && GOOS=freebsd GOARCH=386 CGO_ENABLED=0 go build -trimpath $(BUILD_FLAGS) -o ../../$$CMD.freebsd.i386 ) ; \
	done

build-darwin-aarch64: vendor
	set -e; for CMD in $(CMDS); do \
		( cd ./cmd/$$CMD && GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath $(BUILD_FLAGS) -o ../../$$CMD.darwin.aarch64 ) ; \
	done

test: vendor
	go test -short -v $(TEST_FLAGS) pkg/* pkg/*/cmd
	if grep -rn TODO: ./cmd/ ./pkg/ ./packaging/ ; then exit 1; fi
	if grep -rn Dump ./cmd/ ./pkg/ | grep -v dump.go | grep -v DumpRe | grep -v ThreadDump; then exit 1; fi

# test with filter
testf: vendor
	go test -short -v $(TEST_FLAGS) pkg/* pkg/*/cmd -run "$(filter-out $@,$(MAKECMDGOALS))" 2>&1 | grep -v "no test files" | grep -v "no tests to run" | grep -v "^PASS" | grep -v "^FAIL"

longtest: vendor
	go test -v $(TEST_FLAGS) pkg/* pkg/*/cmd

citest: vendor
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
	if grep -rn TODO: ./cmd/ ./pkg/ ./packaging/ ; then exit 1; fi
	#
	# Checking remaining debug calls
	#
	if grep -rn Dump ./cmd/ ./pkg/ | grep -v dump.go | grep -v DumpRe | grep -v ThreadDump; then exit 1; fi
	#
	# Run other subtests
	#
	$(MAKE) golangci
	#-$(MAKE) govulncheck
	$(MAKE) fmt
	#
	# Normal test cases
	#
	$(MAKE) test
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
	# All CI tests successful
	#

benchmark:
	go test $(TEST_FLAGS) -v -bench=B\* -run=^$$ -benchmem ./pkg/* pkg/*/cmd

racetest:
	go test -race $(TEST_FLAGS) -coverprofile=coverage.txt -covermode=atomic ./pkg/* pkg/*/cmd

covertest:
	go test -v $(TEST_FLAGS) -coverprofile=cover.out ./pkg/* pkg/*/cmd
	go tool cover -func=cover.out
	go tool cover -html=cover.out -o coverage.html

coverweb:
	go test -v $(TEST_FLAGS) -coverprofile=cover.out ./pkg/* pkg/*/cmd
	go tool cover -html=cover.out

clean:
	set -e; for CMD in $(CMDS); do \
		rm -f ./cmd/$$CMD/$$CMD; \
	done
	rm -f $(CMDS)
	rm -f *.windows.*.exe
	rm -f *.linux.*
	rm -f *.darwin.*
	rm -f *.freebsd.*
	rm -f cover.out
	rm -f coverage.html
	rm -f coverage.txt
	rm -rf vendor/
	rm -rf $(TOOLSFOLDER)
	rm -rf dist/
	rm -rf build-deb/
	rm -rf build-rpm/
	rm -f release_notes.txt

GOVET=go vet -all
fmt: tools
	set -e; for CMD in $(CMDS); do \
		$(GOVET) ./cmd/$$CMD; \
	done
	set -e; for dir in $(shell ls -d1 pkg/*); do \
		$(GOVET) ./$$dir; \
	done
	gofmt -w -s ./cmd/ ./pkg/
	./tools/gofumpt -w ./cmd/ ./pkg/
	./tools/gci write ./cmd/. ./pkg/.  --skip-generated
	goimports -w ./cmd/ ./pkg/

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
	set -e; for dir in $$(ls -1d pkg/*); do \
		echo $$dir; \
		if [ $$dir != "pkg/eventlog" ]; then \
			echo "  - GOOS=linux"; \
			( cd $$dir && GOOS=linux golangci-lint run ./... ); \
		fi; \
		echo "  - GOOS=windows"; \
		( cd $$dir && GOOS=windows golangci-lint run ./... ); \
	done

govulncheck: tools
	govulncheck ./...

version:
	OLDVERSION="$(shell grep "VERSION =" ./pkg/snclient/snclient.go | awk '{print $$3}' | tr -d '"')"; \
	NEWVERSION=$$(dialog --stdout --inputbox "New Version:" 0 0 "v$$OLDVERSION") && \
		NEWVERSION=$$(echo $$NEWVERSION | sed "s/^v//g"); \
		if [ "v$$OLDVERSION" = "v$$NEWVERSION" -o "x$$NEWVERSION" = "x" ]; then echo "no changes"; exit 1; fi; \
		sed -i -e 's/VERSION =.*/VERSION = "'$$NEWVERSION'"/g' cmd/*/*.go pkg/snclient/*.go

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
	rm dist/snclient
	sed -i dist/snclient.ini \
		-e 's/\/etc\/snclient/${exe-path}/g' \
		-e 's/^file name =.*/file name = $${shared-path}\/snclient.log/g' \
		-e 's/^max size =.*/max size = 10MiB/g'


snclient: build snclient.ini

snclient.ini:
	cp packaging/snclient.ini .

server.crt: | dist
	cp dist/server.crt .

server.key: | dist
	cp dist/server.key .

deb: | dist
	mkdir -p \
		build-deb/etc/snclient \
		build-deb/usr/bin \
		build-deb/lib/systemd/system \
		build-deb/etc/logrotate.d \
		build-deb/usr/share/doc/snclient \
		build-deb/usr/share/doc/snclient \
		build-deb/usr/share/man/man1 \
		build-deb/usr/share/man/man8

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

	sed -i build-deb/DEBIAN/control -e 's|^Architecture: .*|Architecture: $(DEB_ARCH)|'
	sed -i build-deb/DEBIAN/control -e 's|^Version: .*|Version: $(VERSION)|'

	chmod 644 build-deb/etc/snclient/*

	cp -p dist/snclient.1 build-deb/usr/share/man/man1/snclient.1
	gzip -n -9 build-deb/usr/share/man/man1/snclient.1
	cp -p dist/snclient.8 build-deb/usr/share/man/man8/snclient.8
	gzip -n -9 build-deb/usr/share/man/man8/snclient.8

	dpkg-deb --build --root-owner-group ./build-deb ./$(DEBFILE)
	rm -rf ./build-deb
	-lintian ./$(DEBFILE)

rpm: | dist
	rm -rf snclient-$(VERSION)
	cp ./packaging/snclient.service dist/
	cp ./packaging/snclient.spec dist/
	sed -i dist/snclient.spec -e 's|^Version: .*|Version: $(VERSION)|'
	sed -i dist/snclient.spec -e 's|^BuildArch: .*|BuildArch: $(RPM_ARCH)|'
	cp -rp dist snclient-$(VERSION)
	rm -f snclient-$(VERSION)/ca.key
	tar cfz snclient-$(VERSION).tar.gz snclient-$(VERSION)
	rm -rf snclient-$(VERSION)
	mkdir -p $(RPM_TOPDIR)/{SOURCES,BUILD,RPMS,SRPMS,SPECS}
	mv snclient-$(VERSION).tar.gz $(RPM_TOPDIR)/SOURCES
	rpmbuild \
		--target $(RPM_ARCH) \
		--define "_topdir $(RPM_TOPDIR)" \
		--buildroot=$(shell pwd)/build-rpm \
		-bb dist/snclient.spec
	mv $(RPM_TOPDIR)/RPMS/*/snclient-*.rpm snclient-$(VERSION)-$(BUILD)-$(RPM_ARCH).rpm
	rm -rf $(RPM_TOPDIR) build-rpm
	-rpmlint -f packaging/rpmlintrc snclient-$(VERSION)-$(BUILD)-$(RPM_ARCH).rpm

osx: | dist
	rm -rf build-pkg

	mkdir -p \
		build-pkg/Library/LaunchDaemons \
		build-pkg/usr/local/bin \
		build-pkg/etc/snclient \
		build-pkg/usr/local/share/man/man1 \
		build-pkg/usr/local/share/man/man8

	cp packaging/osx/com.snclient.snclient.plist build-pkg/Library/LaunchDaemons/

	cp dist/snclient build-pkg/usr/local/bin/

	cp dist/snclient.ini dist/server.crt dist/server.key dist/cacert.pem build-pkg/etc/snclient

	sed -i "" \
		-e 's/^max size =.*/max size = 10MiB/g' \
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
