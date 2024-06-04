module github.com/consol-monitoring/snclient

go 1.22.0

replace pkg/dump => ./pkg/dump

replace github.com/consol-monitoring/snclient/pkg/eventlog => ./pkg/eventlog

replace github.com/consol-monitoring/snclient/pkg/convert => ./pkg/convert

replace pkg/counter => ./pkg/counter

replace github.com/consol-monitoring/snclient/pkg/humanize => ./pkg/humanize

replace github.com/consol-monitoring/snclient/pkg/nrpe => ./pkg/nrpe

replace pkg/snclient => ./pkg/snclient

replace pkg/snclient/commands => ./pkg/snclient/commands

replace github.com/consol-monitoring/snclient/pkg/utils => ./pkg/utils

replace github.com/consol-monitoring/snclient/pkg/wmi => ./pkg/wmi

replace pkg/check_dns => ./pkg/check_dns

replace pkg/check_tcp => ./pkg/check_tcp

// use fork with pulled patches
replace github.com/shirou/gopsutil/v3 => github.com/sni/gopsutil/v3 v3.0.0-20240506201943-915d2ad98f31

require (
	github.com/consol-monitoring/snclient/pkg/utils v0.0.0-20240514201752-4ef91b19d850
	github.com/stretchr/testify v1.9.0
	pkg/snclient/commands v0.0.0-00010101000000-000000000000
)

require (
	github.com/DataDog/zstd v1.5.5 // indirect
	github.com/ProtonMail/go-crypto v1.0.0 // indirect
	github.com/beevik/ntp v1.4.3 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/capnspacehook/taskmaster v0.0.0-20210519235353-1629df7c85e9 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudflare/circl v1.3.8 // indirect
	github.com/consol-monitoring/check_nsc_web/pkg/checknscweb v0.0.0-20240321161425-fd9209e96e1f // indirect
	github.com/consol-monitoring/snclient/pkg/convert v0.0.0-20240514201752-4ef91b19d850 // indirect
	github.com/consol-monitoring/snclient/pkg/eventlog v0.0.0-20240514201752-4ef91b19d850 // indirect
	github.com/consol-monitoring/snclient/pkg/humanize v0.0.0-20240514201752-4ef91b19d850 // indirect
	github.com/consol-monitoring/snclient/pkg/nrpe v0.0.0-20240514201752-4ef91b19d850 // indirect
	github.com/consol-monitoring/snclient/pkg/wmi v0.0.0-20240514201752-4ef91b19d850 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-chi/chi/v5 v5.0.12 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jessevdk/go-flags v1.5.0 // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/kdar/factorlog v0.0.0-20211012144011-6ea75a169038 // indirect
	github.com/klauspost/compress v1.17.8 // indirect
	github.com/lufia/plan9stats v0.0.0-20240513124658-fba389f38bae // indirect
	github.com/mackerelio/checkers v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/miekg/dns v1.1.59 // indirect
	github.com/petermattis/goid v0.0.0-20240503122002-4b96552b8156 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_golang v1.19.1 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.54.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/reeflective/readline v1.0.14 // indirect
	github.com/rickb777/date v1.20.6 // indirect
	github.com/rickb777/plural v1.4.2 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/sasha-s/go-deadlock v0.3.1 // indirect
	github.com/sassoftware/go-rpmutils v0.4.0 // indirect
	github.com/sevlyar/go-daemon v0.1.6 // indirect
	github.com/shirou/gopsutil/v3 v3.24.5 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/sni/check_http_go/pkg/checkhttp v0.0.0-20231227232912-71c069b10aae // indirect
	github.com/sni/shelltoken v0.0.0-20240314123449-84b0a0c05450 // indirect
	github.com/spf13/cobra v1.8.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	github.com/tklauser/numcpus v0.8.0 // indirect
	github.com/ulikunitz/xz v0.5.12 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/crypto v0.23.0 // indirect
	golang.org/x/exp v0.0.0-20240531132922-fd00a4e0eefc // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/term v0.20.0 // indirect
	golang.org/x/tools v0.21.0 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	pkg/check_dns v0.0.0-00010101000000-000000000000 // indirect
	pkg/check_tcp v0.0.0-00010101000000-000000000000 // indirect
	pkg/counter v0.0.0-00010101000000-000000000000 // indirect
	pkg/snclient v0.0.0-00010101000000-000000000000 // indirect
)
