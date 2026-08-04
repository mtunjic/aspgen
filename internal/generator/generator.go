package generator

import (
	"errors"
	"fmt"
)

// Version is set at build time via -ldflags "-X aspgen/internal/generator.Version=...".
var Version = "dev"

func Run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	if isHelp(args[0]) {
		fmt.Print(topLevelHelp)
		return nil
	}
	switch args[0] {
	case "new":
		return newProject(args[1:])
	case "add":
		return add(args[1:])
	case "import-db":
		return importDBCmd(args[1:])
	case "templates":
		return templateCommand(args[1:])
	case "version":
		fmt.Println("aspgen " + Version)
		return nil
	default:
		return usage()
	}
}

func usage() error {
	return errors.New("usage: aspgen new NAME --app webapi|wpf|blazor|fullstack [flags] | aspgen add KIND NAME [flags] | aspgen import-db --project PATH [flags]; run 'aspgen --help' for details")
}
