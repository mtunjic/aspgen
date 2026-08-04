package generator

import (
	"errors"
	"fmt"
)

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
	case "templates":
		return templateCommand(args[1:])
	case "version":
		fmt.Println("aspgen dev")
		return nil
	default:
		return usage()
	}
}

func usage() error {
	return errors.New("usage: aspgen new NAME --app webapi|wpf|blazor|fullstack [flags] | aspgen add KIND NAME [flags]; run 'aspgen --help' for details")
}
