module github.com/consol-monitoring/snclient/pkg/utils

go 1.22.0

require (
	github.com/kdar/factorlog v0.0.0-20211012144011-6ea75a169038
	github.com/stretchr/testify v1.9.0
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842
	pkg/convert v0.0.0-00010101000000-000000000000
)

replace pkg/convert => ../../pkg/convert

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
