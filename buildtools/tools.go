//go:build tools
// +build tools

package tools

/*
 * list of modules required to build and run all tests
 * see: https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
 */

import (
	_ "pkg/dump"

	_ "github.com/daixiang0/gci"
	_ "github.com/florentsolt/colorgo"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/tc-hib/go-winres"
	_ "golang.org/x/tools/cmd/goimports"
	_ "golang.org/x/vuln/cmd/govulncheck"
	_ "mvdan.cc/gofumpt"
)
