package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"gofu.dev/gofu/internal/cli"
)

var version = ""

func init() {
	if version != "" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	} else {
		version = "dev"
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: gofu <command> [args]")
		os.Exit(2)
	}

	switch os.Args[1] {
	case "--version":
		fmt.Println("gofu " + version)
	case "build":
		os.Exit(cli.Build(os.Args[2:], os.Stderr))
	case "run":
		os.Exit(cli.Run(os.Args[2:], os.Stdout, os.Stderr))
	case "init":
		os.Exit(cli.Init(os.Args[2:], os.Stderr))
	case "test":
		os.Exit(cli.Test(os.Args[2:], os.Stdout, os.Stderr))
	default:
		fmt.Fprintf(os.Stderr, "gofu: unknown command %q\n", os.Args[1])
		os.Exit(2)
	}
}
