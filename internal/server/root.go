package server

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "server",
	Short: "Eventing server with WAL support",
	Long:  `A Go server using standard library, chi for routing, and cobra for CLI`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return startServer()
	},
}
