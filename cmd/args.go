package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// placeholderRe matches <...> tokens in a cobra Use string.
var placeholderRe = regexp.MustCompile(`<[^>]+>`)

// exactArgs returns a cobra.PositionalArgs validator that requires exactly n
// arguments and produces an actionable error message when validation fails.
//
// The placeholder name (e.g. "<agent>") is extracted from cmd.Use.
// The full usage line is built from cmd.CommandPath() + the argument portion of cmd.Use.
func exactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == n {
			return nil
		}

		useLine := cmd.CommandPath()
		// Extract the argument portion from Use (everything after the first word).
		if parts := strings.SplitN(cmd.Use, " ", 2); len(parts) == 2 {
			useLine += " " + parts[1]
		}

		if len(args) < n {
			placeholder := "argument"
			if match := placeholderRe.FindString(cmd.Use); match != "" {
				placeholder = match
			}
			return fmt.Errorf("missing required argument: %s\nUsage: %s", placeholder, useLine)
		}

		// Too many arguments.
		return fmt.Errorf("expected %d argument, got %d\nUsage: %s", n, len(args), useLine)
	}
}
