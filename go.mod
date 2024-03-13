module github.com/consol-monitoring/snclient

go 1.21

replace pkg/dump => ./pkg/dump

replace pkg/eventlog => ./pkg/eventlog

replace pkg/convert => ./pkg/convert

replace pkg/humanize => ./pkg/humanize

replace pkg/nrpe => ./pkg/nrpe

replace pkg/snclient => ./pkg/snclient

replace pkg/snclient/cmd => ./pkg/snclient/cmd

replace pkg/utils => ./pkg/utils

replace pkg/wmi => ./pkg/wmi

replace pkg/check_dns => ./pkg/check_dns

replace pkg/check_tcp => ./pkg/check_tcp

// use fork with pulled patches
replace github.com/shirou/gopsutil/v3 => github.com/sni/gopsutil/v3 v3.0.0-20240129124248-a5f3e5722a21

require (
	github.com/stretchr/testify v1.9.0
	pkg/snclient/cmd v0.0.0-00010101000000-000000000000
	pkg/utils v0.0.0-00010101000000-000000000000
)

require (
	github.com/DataDog/zstd v1.5.5 // indirect
	github.com/beevik/ntp v1.3.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/capnspacehook/taskmaster v0.0.0-20210519235353-1629df7c85e9 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/consol-monitoring/check_nsc_web/pkg/checknscweb v0.0.0-20240313214236-5e1cfe7a3e3f // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-chi/chi/v5 v5.0.12 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jessevdk/go-flags v1.5.0 // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/kdar/factorlog v0.0.0-20211012144011-6ea75a169038 // indirect
	github.com/klauspost/compress v1.17.7 // indirect
	github.com/lufia/plan9stats v0.0.0-20240226150601-1dcf7310316a // indirect
	github.com/mackerelio/checkers v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/miekg/dns v1.1.58 // indirect
	github.com/onsi/gomega v1.31.1 // indirect
	github.com/petermattis/goid v0.0.0-20231207134359-e60b3f734c67 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_golang v1.19.0 // indirect
	github.com/prometheus/client_model v0.6.0 // indirect
	github.com/prometheus/common v0.50.0 // indirect
	github.com/prometheus/procfs v0.13.0 // indirect
	github.com/reeflective/readline v1.0.13 // indirect
	github.com/rickb777/date v1.20.6 // indirect
	github.com/rickb777/plural v1.4.1 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/sasha-s/go-deadlock v0.3.1 // indirect
	github.com/sassoftware/go-rpmutils v0.3.0 // indirect
	github.com/sevlyar/go-daemon v0.1.6 // indirect
	github.com/shirou/gopsutil/v3 v3.24.2 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/sni/check_http_go/pkg/checkhttp v0.0.0-20231227232912-71c069b10aae // indirect
	github.com/sni/shelltoken v0.0.0-20240305201340-d67cf5c19d23 // indirect
	github.com/spf13/cobra v1.8.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tklauser/go-sysconf v0.3.13 // indirect
	github.com/tklauser/numcpus v0.7.0 // indirect
	github.com/ulikunitz/xz v0.5.11 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/exp v0.0.0-20240222234643-814bf88cf225 // indirect
	golang.org/x/mod v0.16.0 // indirect
	golang.org/x/net v0.22.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/term v0.18.0 // indirect
	golang.org/x/tools v0.19.0 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	pkg/check_dns v0.0.0-00010101000000-000000000000 // indirect
	pkg/check_tcp v0.0.0-00010101000000-000000000000 // indirect
	pkg/convert v0.0.0-00010101000000-000000000000 // indirect
	pkg/eventlog v0.0.0-00010101000000-000000000000 // indirect
	pkg/humanize v0.0.0-00010101000000-000000000000 // indirect
	pkg/nrpe v0.0.0-00010101000000-000000000000 // indirect
	pkg/snclient v0.0.0-00010101000000-000000000000 // indirect
	pkg/wmi v0.0.0-00010101000000-000000000000 // indirect
)
