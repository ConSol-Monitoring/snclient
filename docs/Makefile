#!/usr/bin/make -f


server: .omd-consol-de
	docker compose up --build

test: .bin/muffet .omd-consol-de
	docker compose --progress=quiet --ansi=never up -d --build --wait --no-color --no-recreate --quiet-pull
	docker compose exec site chmod -R 777 public
	./.bin/muffet \
		http://localhost:1313/docs/snclient/ \
		-i 'http://localhost:1313/docs/snclient/' \
		--max-connections=50 \
		--max-connections-per-host=50 \
		--timeout=30

.omd-consol-de:
	git clone --depth=1 https://github.com/ConSol-Monitoring/omd-consol-de .omd-consol-de

.bin/muffet:
	mkdir -p .bin
	GOBIN=$(shell pwd)/.bin go install github.com/raviqqe/muffet/v2@latest

clean:
	docker compose down --rmi all
	docker compose kill -s INT
	docker compose kill
	docker compose rm -f
	rm -rf .omd-consol-de
	rm -rf .bin
