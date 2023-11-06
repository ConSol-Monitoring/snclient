module snclient

go 1.21

replace pkg/dump => ../../pkg/dump

replace pkg/eventlog => ../../pkg/eventlog

replace pkg/convert => ../../pkg/convert

replace pkg/humanize => ../../pkg/humanize

replace pkg/nrpe => ../../pkg/nrpe

replace pkg/snclient => ../../pkg/snclient

replace pkg/snclient/cmd => ../../pkg/snclient/cmd

replace pkg/utils => ../../pkg/utils

replace pkg/wmi => ../../pkg/wmi

replace pkg/check_dns => ../../pkg/check_dns

replace pkg/check_tcp => ../../pkg/check_tcp

require (
	github.com/capnspacehook/taskmaster v0.0.0-20210519235353-1629df7c85e9
	github.com/consol-monitoring/check_nsc_web/pkg/checknscweb v0.0.0-20230911200955-df38adef6161
	github.com/elastic/beats/v7 v7.17.14
	github.com/go-chi/chi/v5 v5.0.10
	github.com/kdar/factorlog v0.0.0-20211012144011-6ea75a169038
	github.com/otiai10/copy v1.14.0
	github.com/prometheus/client_golang v1.17.0
	github.com/sasha-s/go-deadlock v0.3.1
	github.com/sassoftware/go-rpmutils v0.2.0
	github.com/sevlyar/go-daemon v0.1.6
	github.com/shirou/gopsutil/v3 v3.23.10
	github.com/sni/check_http_go/pkg/checkhttp v0.0.0-20231029180838-9bde49d5f2ed
	github.com/stretchr/testify v1.8.4
	golang.org/x/exp v0.0.0-20231006140011-7918f672742d
	golang.org/x/sys v0.14.0
	gopkg.in/yaml.v3 v3.0.1
	pkg/check_dns v0.0.0-00010101000000-000000000000
	pkg/check_tcp v0.0.0-00010101000000-000000000000
	pkg/convert v0.0.0-00010101000000-000000000000
	pkg/eventlog v0.0.0-00010101000000-000000000000
	pkg/humanize v0.0.0-00010101000000-000000000000
	pkg/nrpe v0.0.0-00010101000000-000000000000
	pkg/utils v0.0.0-00010101000000-000000000000
	pkg/wmi v0.0.0-00010101000000-000000000000
)

require (
	github.com/DataDog/zstd v1.5.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/elastic/go-sysinfo v1.11.1 // indirect
	github.com/elastic/go-ucfg v0.8.6 // indirect
	github.com/elastic/go-windows v1.0.1 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/jessevdk/go-flags v1.5.0 // indirect
	github.com/joeshaw/multierror v0.0.0-20140124173710-69b34d4ec901 // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/klauspost/compress v1.17.2 // indirect
	github.com/lufia/plan9stats v0.0.0-20231016141302-07b5767bb0ed // indirect
	github.com/mackerelio/checkers v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/miekg/dns v1.1.56 // indirect
	github.com/namsral/flag v1.7.4-pre // indirect
	github.com/petermattis/goid v0.0.0-20230904192822-1876fd5063bc // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20221212215047-62379fc7944b // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.45.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/rickb777/date v1.20.5 // indirect
	github.com/rickb777/plural v1.4.1 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/ulikunitz/xz v0.5.11 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/yusufpapurcu/wmi v1.2.3 // indirect
	go.elastic.co/ecszap v1.0.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.26.0 // indirect
	golang.org/x/crypto v0.14.0 // indirect
	golang.org/x/mod v0.14.0 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/tools v0.14.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	howett.net/plist v1.0.0 // indirect
)
