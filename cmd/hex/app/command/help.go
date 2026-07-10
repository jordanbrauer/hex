package command

import (
	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex/cmd/hex/infrastructure/embedfs"
)

// helpLong returns the embedded long description for a command key.
func helpLong(key string) string {
	return embedfs.HelpLong(key)
}

// helpExample returns the embedded example script for a command key.
func helpExample(key string) string {
	return embedfs.HelpExample(key)
}

// setExample sets cmd.Example from the embedded example script for key.
func setExample(cmd *cobra.Command, key string) {
	cmd.Example = helpExample(key)
}
