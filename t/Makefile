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


testca:
	mkdir -p ./dist
	openssl genrsa -out dist/ca.key 4096
	openssl req -key dist/ca.key -new -x509 -days 20000 -sha256 -extensions v3_ca -out dist/cacert.pem -subj "/C=DE/ST=Bavaria/L=Earth/O=snclient/OU=IT/CN=Root CA SNClient"
	openssl req -newkey rsa:2048 -nodes -keyout dist/server.key -out dist/server.csr -subj "/CN=snclient" -reqexts SAN -extensions SAN -config <(echo -e "[req]\ndistinguished_name=req\n[SAN]\nsubjectAltName=DNS:snclient")
	openssl x509 -req -CAcreateserial -CA dist/cacert.pem -CAkey dist/ca.key -days 20000 -in dist/server.csr -out dist/server.crt
	rm -f dist/server.csr dist/cacert.srl

clean-testca:
	rm -rf dist