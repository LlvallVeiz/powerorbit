// Command powerorbit is the CubeSat power system simulator CLI.
//
// It answers one question before a CubeSat flies: "will your satellite
// survive the eclipse?" — by simulating solar generation, battery state of
// charge, and subsystem loads orbit by orbit.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags "-X main.version=...".
// It defaults to the development version when built without ldflags.
var version = "v0.1.0-dev"

func main() {
	rootCmd := &cobra.Command{
		Use:   "powerorbit",
		Short: "CubeSat power system simulator — will your satellite survive the eclipse?",
		Long: `powerorbit simulates a CubeSat's electrical power system (EPS) in low Earth
orbit: solar panels generate power in sunlight, the battery carries the
spacecraft through eclipse, and subsystem loads drain it. It reports the
energy balance per orbit, the minimum state of charge, and a PASS/FAIL
verdict against the engineering rules (DoD <= 30% per orbit, SoC_min >= 20%).

See 'powerorbit simulate --help' once the simulation engine lands (Day 6).
Until then, 'powerorbit version' confirms the binary works.`,
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the powerorbit version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "powerorbit %s\n", version)
		},
	}
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
