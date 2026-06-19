package cmd

import (
	"errors"
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"cosmossdk.io/log"
	confixcmd "cosmossdk.io/tools/confix/cmd"

	dbm "github.com/cosmos/cosmos-db"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/debug"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/client/pruning"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/client/snapshot"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"

	capability "github.com/cosmos/ibc-go/modules/capability"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	ibctransfer "github.com/cosmos/ibc-go/v8/modules/apps/transfer"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v8/modules/core"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibctm "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"

	"github.com/dulgi/dulgi/app"
)

// addIBCBasics splices the manually-wired IBC module basics into the
// depinject-provided BasicManager (which only knows about SDK modules). This
// keeps `init` and `genesis` commands aware of IBC genesis state.
func addIBCBasics(bm module.BasicManager) module.BasicManager {
	bm[capabilitytypes.ModuleName] = capability.AppModuleBasic{}
	bm[ibcexported.ModuleName] = ibc.AppModuleBasic{}
	bm[ibctransfertypes.ModuleName] = ibctransfer.AppModuleBasic{}
	bm[ibctm.ModuleName] = ibctm.AppModuleBasic{}
	return bm
}

// initRootCmd registers all top-level subcommands. Module-specific tx and query
// commands are added automatically by AutoCLI in NewRootCmd, so only the
// chain-management commands are wired here.
func initRootCmd(rootCmd *cobra.Command, txConfig client.TxConfig, basicManager module.BasicManager) {
	cfg := sdk.GetConfig()
	cfg.Seal()

	basicManager = addIBCBasics(basicManager)

	rootCmd.AddCommand(
		genutilcli.InitCmd(basicManager, app.DefaultNodeHome),
		debug.Cmd(),
		confixcmd.ConfigCommand(),
		pruning.Cmd(newApp, app.DefaultNodeHome),
		snapshot.Cmd(newApp),
	)

	// start, export, version, rollback, etc.
	server.AddCommands(rootCmd, app.DefaultNodeHome, newApp, appExport, addModuleInitFlags)

	rootCmd.AddCommand(
		server.StatusCommand(),
		genesisCommand(txConfig, basicManager),
		queryCommand(),
		txCommand(),
		keys.Commands(),
	)
}

// addModuleInitFlags is a no-op: Dulgi removes x/crisis (the only module that
// historically added start-time flags here), so there is nothing to register.
func addModuleInitFlags(_ *cobra.Command) {}

// genesisCommand builds the "genesis" parent command (gentx, collect-gentxs,
// add-genesis-account, validate-genesis, migrate).
func genesisCommand(txConfig client.TxConfig, basicManager module.BasicManager, cmds ...*cobra.Command) *cobra.Command {
	cmd := genutilcli.Commands(txConfig, basicManager, app.DefaultNodeHome)
	for _, subCmd := range cmds {
		cmd.AddCommand(subCmd)
	}
	return cmd
}

func queryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		rpc.QueryEventForTxCmd(),
		rpc.ValidatorCommand(),
		server.QueryBlockCmd(),
		authcmd.QueryTxsByEventsCmd(),
		server.QueryBlocksCmd(),
		authcmd.QueryTxCmd(),
		server.QueryBlockResultsCmd(),
		// IBC query commands are not AutoCLI-enabled in ibc-go v8, so we add
		// them explicitly. Yields `query ibc {client,connection,channel}` and
		// `query ibc-transfer ...`.
		ibc.AppModuleBasic{}.GetQueryCmd(),
		ibctransfer.AppModuleBasic{}.GetQueryCmd(),
	)

	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")
	return cmd
}

func txCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		authcmd.GetSignCommand(),
		authcmd.GetSignBatchCommand(),
		authcmd.GetMultiSignCommand(),
		authcmd.GetMultiSignBatchCmd(),
		authcmd.GetValidateSignaturesCommand(),
		authcmd.GetBroadcastCommand(),
		authcmd.GetEncodeCommand(),
		authcmd.GetDecodeCommand(),
		// IBC tx commands (client update/upgrade, channel, ICS-20 transfer).
		ibc.AppModuleBasic{}.GetTxCmd(),
		ibctransfer.AppModuleBasic{}.GetTxCmd(),
	)

	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")
	return cmd
}

// newApp builds a fully-loaded DulgiApp for `dulgid start`.
func newApp(logger log.Logger, db dbm.DB, traceStore io.Writer, appOpts servertypes.AppOptions) servertypes.Application {
	baseappOptions := server.DefaultBaseappOptions(appOpts)
	return app.NewDulgiApp(logger, db, traceStore, true, appOpts, baseappOptions...)
}

// appExport exports chain state to a genesis file for `dulgid export`.
func appExport(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	height int64,
	forZeroHeight bool,
	jailAllowedAddrs []string,
	appOpts servertypes.AppOptions,
	modulesToExport []string,
) (servertypes.ExportedApp, error) {
	var dulgiApp *app.DulgiApp

	homePath, ok := appOpts.Get(flags.FlagHome).(string)
	if !ok || homePath == "" {
		return servertypes.ExportedApp{}, errors.New("application home not set")
	}

	viperAppOpts, ok := appOpts.(*viper.Viper)
	if !ok {
		return servertypes.ExportedApp{}, errors.New("appOpts is not viper.Viper")
	}
	appOpts = viperAppOpts

	if height != -1 {
		dulgiApp = app.NewDulgiApp(logger, db, traceStore, false, appOpts)
		if err := dulgiApp.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	} else {
		dulgiApp = app.NewDulgiApp(logger, db, traceStore, true, appOpts)
	}

	return dulgiApp.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs, modulesToExport)
}
