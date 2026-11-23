// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/openchami/boot-service/pkg/bootadopt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newBootAdoptCommand() *cobra.Command {
	var options bootadopt.Options
	var dryRun bool
	cmd := &cobra.Command{
		Use: "boot-adopt", Short: "Plan or execute explicit legacy boot adoption",
		Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().Changed("dry-run") && cmd.Flags().Changed("execute") {
				err := fmt.Errorf("--dry-run and --execute are mutually exclusive")
				if outputErr := writeAdoptionReceipt(cmd.OutOrStdout(), bootadopt.Receipt{Operation: "boot-adopt", Phase: "preflight", Error: err.Error()}); outputErr != nil {
					return outputErr
				}
				return err
			}
			options.Server, options.Token = viper.GetString("server"), viper.GetString("token")
			duration := viper.GetDuration("timeout")
			if duration <= 0 {
				duration = 30 * time.Second
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), duration)
			defer cancel()
			receipt, err := bootadopt.Run(ctx, options)
			if outputErr := writeAdoptionReceipt(cmd.OutOrStdout(), receipt); outputErr != nil {
				return outputErr
			}
			return err
		},
	}
	cmd.Flags().StringVar(&options.SourceUID, "source-uid", "", "Legacy BootConfiguration UID to copy")
	cmd.Flags().StringVar(&options.Series, "series", "", "Target BootConfig series name")
	cmd.Flags().StringVar(&options.Assignment, "assignment", "", "Target BootAssignment name")
	cmd.Flags().BoolVar(&options.Execute, "execute", false, "Execute the plan; safe retries reuse named resources")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the plan without writing (the default)")
	return cmd
}

func writeAdoptionReceipt(w io.Writer, receipt bootadopt.Receipt) error {
	if err := json.NewEncoder(w).Encode(receipt); err != nil {
		return fmt.Errorf("write adoption receipt: %w", err)
	}
	return nil
}

func init() { rootCmd.AddCommand(newBootAdoptCommand()) }
