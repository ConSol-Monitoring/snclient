#!/usr/bin/make -f


server: .omd-consol-de
	docker compose up --build

test: .omd-consol-de
	docker compose up -d --build --wait
	go install github.com/raviqqe/muffet/v2@latest
	muffet http://localhost:1313/docs/snclient/ -i '/docs/snclient/*' -e '_print'

.omd-consol-de:
	git clone --depth=1 https://github.com/ConSol-Monitoring/omd-consol-de .omd-consol-de

clean:
	docker compose down --rmi all
	docker compose kill -s INT
	docker compose kill
	docker compose rm -f
	rm -rf .omd-consol-de
