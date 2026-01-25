package main

import (
	_ "embed"

	"huseynovvusal/gitai/cmd"
)

// Version is injected at build time.
//
//go:embed VERSION
var Version string

func main() {
	cmd.Execute(Version)
}
