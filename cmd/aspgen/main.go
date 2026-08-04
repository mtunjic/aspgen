package main

import (
	"fmt"
	"os"

	"aspgen/internal/generator"
)

func main() {
	if err := generator.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "aspgen:", err)
		os.Exit(1)
	}
}
