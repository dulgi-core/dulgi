package app

import (
	"io"
	"os"
	"path/filepath"

	"cosmossdk.io/depinject"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"

	dbm "github.com/cosmos/cosmos-db"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	consensuskeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"

	capability "github.com/cosmos/ibc-go/modules/capability"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	ibctransfer "github.com/cosmos/ibc-go/v8/modules/apps/transfer"
	ibctransferkeeper "github.com/cosmos/ibc-go/v8/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v8/modules/core"
	porttypes "github.com/cosmos/ibc-go/v8/modules/core/05-port/types"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"
	ibctm "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"
)

const (
	// Name is the application / chain identifier used by depinject.
	Name = "dulgi"

	// AccountAddressPrefix is the Bech32 prefix for all account addresses.
	AccountAddressPrefix = "dulgi"
)

// Bech32 HRP (human-readable prefix) constants derived from the account prefix.
const (
	Bech32PrefixAccAddr  = AccountAddressPrefix
	Bech32PrefixAccPub   = AccountAddressPrefix + "pub"
	Bech32PrefixValAddr  = AccountAddressPrefix + "valoper"
	Bech32PrefixValPub   = AccountAddressPrefix + "valoperpub"
	Bech32PrefixConsAddr = AccountAddressPrefix + "valcons"
	Bech32PrefixConsPub  = AccountAddressPrefix + "valconspub"
)

// DefaultNodeHome is the default home directory for the dulgid binary.
var DefaultNodeHome string

func init() {
	// Configure the GLOBAL SDK Bech32 prefixes. This must happen before any
	// module address (e.g. the gov authority used by bank/staking/ibc keepers)
	// is constructed, otherwise depinject builds keepers with 'cosmos'-prefixed
	// authority strings that fail validation against our 'dulgi' codec.
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount(Bech32PrefixAccAddr, Bech32PrefixAccPub)
	cfg.SetBech32PrefixForValidator(Bech32PrefixValAddr, Bech32PrefixValPub)
	cfg.SetBech32PrefixForConsensusNode(Bech32PrefixConsAddr, Bech32PrefixConsPub)
	// Note: not sealed here; the CLI seals the config in initRootCmd.

	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	DefaultNodeHome = filepath.Join(userHomeDir, "."+Name)
}

var (
	_ runtime.AppI            = (*DulgiApp)(nil)
	_ servertypes.Application = (*DulgiApp)(nil)
)

// DulgiApp is the Dulgi Layer-1 application. It embeds *runtime.App (the
// depinject-managed core) and adds the manually-wired IBC v8 stack.
type DulgiApp struct {
	*runtime.App

	legacyAmino       *codec.LegacyAmino
	appCodec          codec.Codec
	txConfig          client.TxConfig
	interfaceRegistry codectypes.InterfaceRegistry

	// Keepers populated by depinject.
	AccountKeeper         authkeeper.AccountKeeper
	BankKeeper            bankkeeper.Keeper
	StakingKeeper         *stakingkeeper.Keeper
	SlashingKeeper        slashingkeeper.Keeper
	MintKeeper            mintkeeper.Keeper
	DistrKeeper           distrkeeper.Keeper
	GovKeeper             *govkeeper.Keeper
	UpgradeKeeper         *upgradekeeper.Keeper
	ParamsKeeper          paramskeeper.Keeper
	ConsensusParamsKeeper consensuskeeper.Keeper
	EvidenceKeeper        evidencekeeper.Keeper

	// IBC keepers (wired manually; ibc-go v8 is not yet depinject-native).
	CapabilityKeeper *capabilitykeeper.Keeper
	IBCKeeper        *ibckeeper.Keeper
	TransferKeeper   ibctransferkeeper.Keeper

	ScopedIBCKeeper      capabilitykeeper.ScopedKeeper
	ScopedTransferKeeper capabilitykeeper.ScopedKeeper
}

// NewDulgiApp constructs the application. SDK modules are wired via depinject
// from AppConfig(); the IBC v8 stack is then bolted on manually and spliced
// into the module manager.
func NewDulgiApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	appOpts servertypes.AppOptions,
	baseAppOptions ...func(*baseapp.BaseApp),
) *DulgiApp {
	var (
		app        = &DulgiApp{}
		appBuilder *runtime.AppBuilder
	)

	if err := depinject.Inject(
		depinject.Configs(
			AppConfig(),
			depinject.Supply(logger, appOpts),
		),
		&appBuilder,
		&app.appCodec,
		&app.legacyAmino,
		&app.txConfig,
		&app.interfaceRegistry,
		&app.AccountKeeper,
		&app.BankKeeper,
		&app.StakingKeeper,
		&app.SlashingKeeper,
		&app.MintKeeper,
		&app.DistrKeeper,
		&app.GovKeeper,
		&app.UpgradeKeeper,
		&app.ParamsKeeper,
		&app.ConsensusParamsKeeper,
		&app.EvidenceKeeper,
	); err != nil {
		panic(err)
	}

	app.App = appBuilder.Build(db, traceStore, baseAppOptions...)

	// -------------------------------------------------------------------------
	// Manual IBC v8 wiring
	// -------------------------------------------------------------------------

	// Mount the store keys that are not owned by a depinject module, keeping
	// references so we can hand them directly to the manually-wired keepers
	// (runtime.App does not expose a GetKey accessor in SDK v0.50).
	capStoreKey := storetypes.NewKVStoreKey(capabilitytypes.StoreKey)
	capMemKey := storetypes.NewMemoryStoreKey(capabilitytypes.MemStoreKey)
	ibcStoreKey := storetypes.NewKVStoreKey(ibcexported.StoreKey)
	transferStoreKey := storetypes.NewKVStoreKey(ibctransfertypes.StoreKey)

	if err := app.RegisterStores(capStoreKey, capMemKey, ibcStoreKey, transferStoreKey); err != nil {
		panic(err)
	}

	// Legacy param subspaces required by the IBC keepers for migration support.
	app.ParamsKeeper.Subspace(ibcexported.ModuleName)
	app.ParamsKeeper.Subspace(ibctransfertypes.ModuleName)

	govAuthority := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	// Capability keeper + scoped sub-keepers, sealed so no module can scope
	// after initialization.
	app.CapabilityKeeper = capabilitykeeper.NewKeeper(
		app.appCodec,
		capStoreKey,
		capMemKey,
	)
	app.ScopedIBCKeeper = app.CapabilityKeeper.ScopeToModule(ibcexported.ModuleName)
	app.ScopedTransferKeeper = app.CapabilityKeeper.ScopeToModule(ibctransfertypes.ModuleName)
	app.CapabilityKeeper.Seal()

	// IBC core keeper.
	app.IBCKeeper = ibckeeper.NewKeeper(
		app.appCodec,
		ibcStoreKey,
		app.GetSubspace(ibcexported.ModuleName),
		app.StakingKeeper,
		app.UpgradeKeeper,
		app.ScopedIBCKeeper,
		govAuthority,
	)

	// ICS-20 fungible token transfer keeper.
	app.TransferKeeper = ibctransferkeeper.NewKeeper(
		app.appCodec,
		transferStoreKey,
		app.GetSubspace(ibctransfertypes.ModuleName),
		app.IBCKeeper.ChannelKeeper, // ICS4Wrapper
		app.IBCKeeper.ChannelKeeper,
		app.IBCKeeper.PortKeeper,
		app.AccountKeeper,
		app.BankKeeper,
		app.ScopedTransferKeeper,
		govAuthority,
	)

	// IBC port router: route the "transfer" port to the ICS-20 app module.
	transferIBCModule := ibctransfer.NewIBCModule(app.TransferKeeper)
	ibcRouter := porttypes.NewRouter()
	ibcRouter.AddRoute(ibctransfertypes.ModuleName, transferIBCModule)
	app.IBCKeeper.SetRouter(ibcRouter)

	// Register the manually-wired modules into the depinject module manager so
	// their genesis, gRPC services and ABCI hooks run.
	if err := app.RegisterModules(
		capability.NewAppModule(app.appCodec, *app.CapabilityKeeper, false),
		ibc.NewAppModule(app.IBCKeeper),
		ibctransfer.NewAppModule(app.TransferKeeper),
		ibctm.NewAppModule(),
	); err != nil {
		panic(err)
	}

	// Module ordering (begin/end blockers, init/export genesis, preblockers) is
	// declared on the runtime module config in app_config.go and applied by
	// app.Load() below — including the IBC modules registered just above. We do
	// NOT call SetOrder* here: Load() would overwrite it with the config orders.

	app.RegisterUpgradeHandlers()

	// Load the latest version (mounts every store, including IBC keys), applies
	// the module orderings from the app config, and runs store/upgrade checks.
	if loadLatest {
		if err := app.Load(true); err != nil {
			panic(err)
		}
	}

	return app
}

// LegacyAmino returns the app's legacy amino codec.
func (app *DulgiApp) LegacyAmino() *codec.LegacyAmino { return app.legacyAmino }

// AppCodec returns the app's protobuf codec.
func (app *DulgiApp) AppCodec() codec.Codec { return app.appCodec }

// InterfaceRegistry returns the app's interface registry.
func (app *DulgiApp) InterfaceRegistry() codectypes.InterfaceRegistry {
	return app.interfaceRegistry
}

// TxConfig returns the app's transaction config.
func (app *DulgiApp) TxConfig() client.TxConfig { return app.txConfig }

// GetSubspace returns the param subspace registered for the given module.
func (app *DulgiApp) GetSubspace(moduleName string) paramstypes.Subspace {
	subspace, _ := app.ParamsKeeper.GetSubspace(moduleName)
	return subspace
}

// SimulationManager implements the SimulationApp interface. Dulgi ships without
// the simulation suite to stay lightweight, so this is intentionally nil.
func (app *DulgiApp) SimulationManager() *module.SimulationManager { return nil }
