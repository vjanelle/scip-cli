package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if err := runCLI(args, stdout, stderr); err != nil {
		if exit, ok := err.(cliExit); ok {
			return exit.code
		}

		_, _ = fmt.Fprintf(stderr, "scip-cli: %v\n", err)
		return 1
	}

	return 0
}

func runCLI(args []string, stdout, stderr io.Writer) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if exit, ok := recovered.(cliExit); ok {
				err = exit
				return
			}
			panic(recovered)
		}
	}()

	state := newCLIState()
	program := newCLIProgram(stdout, stderr, &state)
	command, parseErr := program.app.Parse(normalizeArgs(args))
	if parseErr != nil {
		return parseErr
	}
	if command == "" {
		return nil
	}

	handler, ok := program.handlers[command]
	if !ok {
		return fmt.Errorf("unknown command %q", command)
	}

	return handler(stdout)
}
