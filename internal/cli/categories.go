package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/output"
)

func newCategoriesCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{Use: "categories", Short: "Transaction categories"}

	list := &cobra.Command{
		Use:   "list",
		Short: "List built-in and user-defined categories",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, userID, err := a.session()
			if err != nil {
				return err
			}
			cats, err := collect(c.Categories(cmd.Context(), userID))
			if err != nil {
				return err
			}
			return output.List(a.printer(), cats, func(c bunq.Category) bunq.Category { return c },
				[]output.Column[bunq.Category]{
					{Header: "CATEGORY", Value: func(c bunq.Category) string { return c.Category }},
					{Header: "TYPE", Value: func(c bunq.Category) string { return c.Type }},
					{Header: "STATUS", Value: func(c bunq.Category) string { return c.Status }},
					{Header: "DESCRIPTION", Value: func(c bunq.Category) string { return c.Description }},
				})
		},
	}

	var (
		color, icon string
		wf          *writeFlags
	)
	create := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a user-defined category in the next free slot (USER_DEFINED_1..9)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			color = strings.ToUpper(color)
			icon = strings.ToUpper(icon)
			if !slices.Contains(categoryColors, color) {
				return usageErr("--color must be one of " + strings.Join(categoryColors, ", "))
			}
			c, userID, err := a.session()
			if err != nil {
				return err
			}
			cats, err := collect(c.Categories(cmd.Context(), userID))
			if err != nil {
				return err
			}
			slot, err := freeCategorySlot(cats)
			if err != nil {
				return err
			}
			body := bunq.NewCategory{Category: slot, Status: "ACTIVE", Description: args[0], Color: color, Icon: icon}
			ch := change{op: "category create", body: body, summary: [][2]string{{"Name", args[0]}, {"Slot", slot}}}
			if ok, err := a.approve(ch, wf); err != nil || !ok {
				return err
			}
			if err := c.CreateCategory(cmd.Context(), userID, body); err != nil {
				return err
			}
			fmt.Fprintf(a.errOut, "created category %q as %s\n", args[0], slot)
			return nil
		},
	}
	create.Flags().StringVar(&color, "color", "BLUE", "one of "+strings.Join(categoryColors, ", "))
	create.Flags().StringVar(&icon, "icon", "GENERAL", "icon, e.g. GENERAL, HOLIDAY, SPORTS (see categories list --json)")
	wf = addWriteFlags(create, false)

	cmd.AddCommand(list, create)
	return cmd
}

// categoryColors are the colors bunq accepts for categories.
var categoryColors = []string{
	"YELLOW", "DEEP_YELLOW", "ORANGE", "HOT_PINK", "RED", "INDIGO", "PURPLE", "DEEP_PURPLE", "BLUE", "LIGHT_BLUE",
	"DEEP_BLUE", "CYAN", "TURQUOISE", "GREEN", "FUCHSIA", "DEEP_GREEN", "GRAY", "DARK_GRAY", "CALM_GRAY", "BLACK",
}

// maxCategorySlots is the number of user-defined category slots bunq offers.
const maxCategorySlots = 9

// freeCategorySlot returns the first USER_DEFINED_n that is not active.
// bunq always lists all slots; unused ones are INACTIVE.
func freeCategorySlot(cats []bunq.Category) (string, error) {
	used := map[string]bool{}
	for _, c := range cats {
		used[c.Category] = c.Status == "ACTIVE"
	}
	for n := 1; n <= maxCategorySlots; n++ {
		if slot := fmt.Sprintf("USER_DEFINED_%d", n); !used[slot] {
			return slot, nil
		}
	}
	return "", fmt.Errorf("all %d user-defined category slots are in use", maxCategorySlots)
}
