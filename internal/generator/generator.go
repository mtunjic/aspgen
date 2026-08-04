package generator

import (
	"errors"
	"fmt"
)

func Run(args []string) error {
	if len(args) == 0 {
		return usage()
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
	return errors.New("usage: aspgen new NAME --app webapi|wpf|blazor|fullstack [--simple] [--backend ddd] [--database sqlite|postgres] [--seed dummy] [--theme wpfui] | aspgen add context|aggregate|value-object|domain-service|repository|event|entity|module|database|service ...")
}
