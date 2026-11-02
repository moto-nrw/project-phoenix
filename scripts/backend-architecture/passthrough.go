package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/moto-nrw/project-phoenix/internal/passthrough"
)

func passthroughCommand(args []string, project string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	switch args[0] {
	case "passthrough-snapshot":
		flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
		decision := flags.String("decision", "", "reviewed ADR number; defaults to the current register")
		if err := flags.Parse(args[1:]); err != nil {
			return true, err
		}
		if flags.NArg() != 0 {
			return true, fmt.Errorf("passthrough-snapshot accepts --decision only")
		}
		return true, passthrough.WriteSnapshot(os.Stdout, project, *decision)
	case "passthrough-import":
		if len(args) != 2 {
			return true, fmt.Errorf("passthrough-import requires one reviewed snapshot filename")
		}
		return true, passthrough.ImportSnapshot(project, args[1])
	default:
		return false, nil
	}
}
