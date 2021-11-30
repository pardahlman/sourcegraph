package main

import (
	"context"
	"fmt"
	"os"

	"github.com/sourcegraph/sourcegraph/lib/output"
)

var out = output.NewOutput(os.Stdout, output.OutputOpts{
	ForceColor: true,
	ForceTTY:   true,
})

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		args = append(args, "up")
	}

	if err := rootCommand.Parse(args); err != nil {
		os.Exit(1)
	}

	if err := rootCommand.Run(context.Background()); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
}
