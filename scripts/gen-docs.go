package main

import (
	"log"
	"os"

	"ifc-cli/internal/cli"

	"github.com/spf13/cobra/doc"
)

func main() {
	if err := os.MkdirAll("./docs/man/", 0o755); err != nil {
		log.Fatal(err)
	}
	cmd := cli.RootCmd()
	if err := doc.GenManTree(cmd, nil, "./docs/man/"); err != nil {
		log.Fatal(err)
	}
}
