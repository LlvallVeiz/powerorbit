// Command powerorbit is the CubeSat power system simulator CLI.
//
// It answers one question before a CubeSat flies: "will your satellite
// survive the eclipse?" — by simulating solar generation, battery state of
// charge, and subsystem loads orbit by orbit.
package main

import (
	"fmt"
	"os"

	"github.com/LlvallVeiz/powerorbit/internal/config"
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

	var configPath string
	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Load and validate a mission YAML file",
		Long: `Load a mission config file, parse it strictly (unknown fields are
rejected), and check every value against the engineering rules:
LEO altitude 200–2000 km, panel efficiency in (0, 1], DoD <= 30% per
orbit, SoC_min >= 20%, and so on. Exits non-zero and lists every
problem if the config is invalid.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := config.Load(configPath)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "config %q is valid: mission %q, %d load mode(s), orbit-average load %.2f W\n",
				configPath, m.Name, len(m.Loads), m.AverageLoadW())
			return nil
		},
	}
	validateCmd.Flags().StringVarP(&configPath, "config", "c", "configs/1u-reference.yaml", "path to the mission YAML file")
	rootCmd.AddCommand(validateCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
