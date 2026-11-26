// telemetry-audit is a read-only analyzer for synthetic laboratory event files.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("telemetry-audit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("input", "-", "synthetic JSON event file, or - for stdin")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	reader := stdin
	if *input != "-" {
		file, err := os.Open(*input)
		if err != nil {
			return err
		}
		defer file.Close()
		reader = file
	}
	report, err := analyze(reader)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		if err == flag.ErrHelp {
			return
		}
		fmt.Fprintln(os.Stderr, "telemetry-audit:", err)
		os.Exit(1)
	}
}
