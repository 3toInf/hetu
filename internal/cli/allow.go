package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/3toInf/hetu/internal/config"
	"github.com/3toInf/hetu/internal/policy"
	"github.com/spf13/cobra"
)

func newAllowCmd() *cobra.Command {
	root := &cobra.Command{Use: "allow", Short: "Manage approval rules"}
	root.AddCommand(newAllowListCmd(), newAllowAddCmd(), newAllowRmCmd())
	return root
}

func newAllowListCmd() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "Show approval rules",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rs, err := policy.Load(configRulesPath())
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "allow:")
			for _, r := range rs.Allow {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", r)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "deny:")
			for _, r := range rs.Deny {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", r)
			}
			return nil
		}}
}

func newAllowAddCmd() *cobra.Command {
	isDeny := false
	c := &cobra.Command{Use: "add <rule>", Args: cobra.ExactArgs(1),
		Short: "Add an allow rule (e.g. \"Shell(git status:*)\")",
		RunE: func(cmd *cobra.Command, args []string) error {
			return mutateRules(cmd, func(rs *policy.Rules) error {
				rule := args[0]
				if _, err := policy.Parse(rule); err != nil {
					return err
				}
				if isDeny {
					rs.Deny = append(rs.Deny, rule)
				} else {
					rs.Allow = append(rs.Allow, rule)
				}
				return nil
			})
		}}
	c.Flags().BoolVar(&isDeny, "deny", false, "add to the deny list")
	return c
}

func newAllowRmCmd() *cobra.Command {
	isDeny := false
	c := &cobra.Command{Use: "rm <rule>", Args: cobra.ExactArgs(1),
		Short: "Remove a rule by its text",
		RunE: func(cmd *cobra.Command, args []string) error {
			return mutateRules(cmd, func(rs *policy.Rules) error {
				target := args[0]
				list := &rs.Allow
				if isDeny {
					list = &rs.Deny
				}
				for i, r := range *list {
					if r == target {
						*list = append((*list)[:i], (*list)[i+1:]...)
						return nil
					}
				}
				return fmt.Errorf("rule not found: %s", target)
			})
		}}
	c.Flags().BoolVar(&isDeny, "deny", false, "remove from the deny list")
	return c
}

func configRulesPath() string { return config.RulesPath() }

func mutateRules(cmd *cobra.Command, fn func(*policy.Rules) error) error {
	path := configRulesPath()
	rs, err := policy.Load(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		rs = policy.DefaultRules()
	}
	if err := fn(&rs); err != nil {
		return err
	}
	if err := policy.Save(path, rs); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "saved")
	return nil
}
