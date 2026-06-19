package main

import (
	"os"

	clog "cosmossdk.io/log"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"

	"github.com/dulgi/dulgi/app"
	"github.com/dulgi/dulgi/cmd/dulgid/cmd"
)

func main() {
	rootCmd := cmd.NewRootCmd()

	if err := svrcmd.Execute(rootCmd, "DULGID", app.DefaultNodeHome); err != nil {
		clog.NewLogger(rootCmd.OutOrStderr()).Error("failure when running app", "err", err)
		os.Exit(1)
	}
}
