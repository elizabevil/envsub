package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elizabevil/envsub/internal/version"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   commandVersion,
		Short: "Print build information as JSON",
		RunE: func(cmd *cobra.Command, _ []string) error {
			info, err := version.JSON()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(info))
			return err
		},
	}
}
