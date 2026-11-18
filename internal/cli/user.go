package cli

import (
	"strconv"

	"github.com/spf13/cobra"
)

func newUserCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "user",
		Short: "Show the logged-in user",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, userID, err := a.session()
			if err != nil {
				return err
			}
			u, err := c.User(cmd.Context(), userID)
			if err != nil {
				return err
			}
			return a.printer().Fields(u, [][2]string{
				{"ID", strconv.Itoa(u.ID)},
				{"Name", u.DisplayName},
			})
		},
	}
}
