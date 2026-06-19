package app

import (
	runtimev1alpha1 "cosmossdk.io/api/cosmos/app/runtime/v1alpha1"
	appv1alpha1 "cosmossdk.io/api/cosmos/app/v1alpha1"
	authmodulev1 "cosmossdk.io/api/cosmos/auth/module/v1"
	bankmodulev1 "cosmossdk.io/api/cosmos/bank/module/v1"
	consensusmodulev1 "cosmossdk.io/api/cosmos/consensus/module/v1"
	distrmodulev1 "cosmossdk.io/api/cosmos/distribution/module/v1"
	evidencemodulev1 "cosmossdk.io/api/cosmos/evidence/module/v1"
	genutilmodulev1 "cosmossdk.io/api/cosmos/genutil/module/v1"
	govmodulev1 "cosmossdk.io/api/cosmos/gov/module/v1"
	mintmodulev1 "cosmossdk.io/api/cosmos/mint/module/v1"
	paramsmodulev1 "cosmossdk.io/api/cosmos/params/module/v1"
	slashingmodulev1 "cosmossdk.io/api/cosmos/slashing/module/v1"
	stakingmodulev1 "cosmossdk.io/api/cosmos/staking/module/v1"
	txconfigv1 "cosmossdk.io/api/cosmos/tx/config/v1"
	upgrademodulev1 "cosmossdk.io/api/cosmos/upgrade/module/v1"
	vestingmodulev1 "cosmossdk.io/api/cosmos/vesting/module/v1"
	"cosmossdk.io/core/appconfig"
	"cosmossdk.io/depinject"
	evidencetypes "cosmossdk.io/x/evidence/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensustypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// -----------------------------------------------------------------------------
// Module account permissions
//
// We keep the module-account set as small as possible: only the modules that
// actually hold, mint, or burn coins are listed here. Removing unused module
// accounts directly reduces auth state and genesis size.
// -----------------------------------------------------------------------------

var moduleAccPerms = []*authmodulev1.ModuleAccountPermission{
	{Account: authtypes.FeeCollectorName}, // collects tx fees + block rewards before distribution
	{Account: distrtypes.ModuleName},      // holds the community pool + outstanding rewards
	{Account: minttypes.ModuleName, Permissions: []string{authtypes.Minter}},
	{Account: stakingtypes.BondedPoolName, Permissions: []string{authtypes.Burner, stakingtypes.ModuleName}},
	{Account: stakingtypes.NotBondedPoolName, Permissions: []string{authtypes.Burner, stakingtypes.ModuleName}},
	{Account: govtypes.ModuleName, Permissions: []string{authtypes.Burner}},
	// IBC transfer escrow account. ICS-20 mints/burns vouchers for fungible
	// token transfers, so it requires both Minter and Burner permissions.
	{Account: ibcTransferModuleName, Permissions: []string{authtypes.Minter, authtypes.Burner}},
}

// blockAccAddrs are accounts that may never receive externally-sent funds.
// Blocking the bonded/not-bonded pools and fee collector is a standard
// anti-foot-gun measure that also prevents accidental balance corruption.
var blockAccAddrs = []string{
	authtypes.FeeCollectorName,
	distrtypes.ModuleName,
	minttypes.ModuleName,
	stakingtypes.BondedPoolName,
	stakingtypes.NotBondedPoolName,
}

// IBC module names, declared as literals here so the declarative config file
// does not need to import ibc-go (which is wired manually in app.go, not via
// depinject). These must match the corresponding ibc-go ModuleName constants.
const (
	capabilityModuleName  = "capability"    // == capabilitytypes.ModuleName
	ibcModuleName         = "ibc"           // == ibcexported.ModuleName
	ibcTransferModuleName = "transfer"      // == ibctransfertypes.ModuleName
	ibcTendermintName     = "07-tendermint" // == ibctm.ModuleName (light client)
)

// -----------------------------------------------------------------------------
// Module ordering
//
// These slices are consumed by the runtime module below. IBC, capability and
// transfer are wired manually (ibc-go v8 is not yet depinject-native) and are
// spliced into these orders inside app.go via SetOrder*.
// -----------------------------------------------------------------------------

var (
	// beginBlockers: ordering matters for correctness.
	// - mint before distribution so freshly minted rewards are available to distribute.
	// - staking after slashing/evidence so jailing is reflected in the validator set.
	// (upgrade is a PreBlocker, declared separately on the runtime module.)
	//
	// The IBC modules are appended because runtime.App.Load() re-applies these
	// exact lists to the module manager AFTER our manual RegisterModules call —
	// so every manager module (including the IBC ones) must be present here.
	// Modules that implement no begin/end-block hook are simply skipped at
	// execution time, so listing them is harmless.
	beginBlockers = []string{
		minttypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName,
		stakingtypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		govtypes.ModuleName,
		genutiltypes.ModuleName,
		paramstypes.ModuleName,
		vestingtypes.ModuleName,
		consensustypes.ModuleName,
		capabilityModuleName,
		ibcModuleName,
		ibcTransferModuleName,
	}

	// endBlockers: gov before staking so passed proposals affecting params apply,
	// then staking writes the validator-set updates returned to CometBFT.
	endBlockers = []string{
		govtypes.ModuleName,
		stakingtypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		minttypes.ModuleName,
		genutiltypes.ModuleName,
		evidencetypes.ModuleName,
		paramstypes.ModuleName,
		upgradetypes.ModuleName,
		vestingtypes.ModuleName,
		consensustypes.ModuleName,
		capabilityModuleName,
		ibcModuleName,
		ibcTransferModuleName,
	}

	// initGenesis: capability first so other modules can claim capabilities;
	// auth before any module that creates accounts; genutil (gentxs) after
	// staking so delegations validate against staking state; bank after auth.
	genesisModuleOrder = []string{
		capabilityModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		stakingtypes.ModuleName,
		slashingtypes.ModuleName,
		govtypes.ModuleName,
		minttypes.ModuleName,
		ibcModuleName,
		genutiltypes.ModuleName,
		evidencetypes.ModuleName,
		ibcTransferModuleName,
		paramstypes.ModuleName,
		upgradetypes.ModuleName,
		vestingtypes.ModuleName,
		consensustypes.ModuleName,
		ibcTendermintName,
	}
)

// AppConfig is the declarative (depinject) wiring for every SDK module used by
// Dulgi. Anything not listed here is simply not compiled into the binary —
// this is how we guarantee "no CosmWasm / no EVM / no NFT / no ICA / no group /
// no feegrant / no circuit / no authz" rather than merely disabling them at
// runtime.
func AppConfig() depinject.Config {
	return depinject.Configs(
		appConfig,
		// Supply concrete module basics for modules whose CLI requires the
		// concrete type rather than depinject's generic adaptor:
		//   - genutil: gentx/collect-gentxs need its GenTxValidator.
		//   - gov: legacy param-change proposal CLI handler.
		depinject.Supply(
			map[string]module.AppModuleBasic{
				genutiltypes.ModuleName: genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
				govtypes.ModuleName: gov.NewAppModuleBasic([]govclient.ProposalHandler{
					paramsclient.ProposalHandler,
				}),
			},
		),
	)
}

var appConfig = appconfig.Compose(&appv1alpha1.Config{
	Modules: []*appv1alpha1.ModuleConfig{
		{
			Name: runtime.ModuleName,
			Config: appconfig.WrapAny(&runtimev1alpha1.Module{
				AppName: Name,
				// upgrade runs in the PreBlocker (SDK v0.50) so a scheduled
				// upgrade halts the node before any module BeginBlock executes.
				PreBlockers:   []string{upgradetypes.ModuleName},
				BeginBlockers: beginBlockers,
				EndBlockers:   endBlockers,
				InitGenesis:   genesisModuleOrder,
				// auth historically used the "acc" store key; keeping the
				// override preserves compatibility with existing tooling.
				OverrideStoreKeys: []*runtimev1alpha1.StoreKeyConfig{
					{ModuleName: authtypes.ModuleName, KvStoreKey: "acc"},
				},
			}),
		},
		{
			Name: authtypes.ModuleName,
			Config: appconfig.WrapAny(&authmodulev1.Module{
				Bech32Prefix:             AccountAddressPrefix,
				ModuleAccountPermissions: moduleAccPerms,
			}),
		},
		{Name: vestingtypes.ModuleName, Config: appconfig.WrapAny(&vestingmodulev1.Module{})},
		{
			Name: banktypes.ModuleName,
			Config: appconfig.WrapAny(&bankmodulev1.Module{
				BlockedModuleAccountsOverride: blockAccAddrs,
			}),
		},
		{Name: stakingtypes.ModuleName, Config: appconfig.WrapAny(&stakingmodulev1.Module{})},
		{Name: slashingtypes.ModuleName, Config: appconfig.WrapAny(&slashingmodulev1.Module{})},
		{Name: "tx", Config: appconfig.WrapAny(&txconfigv1.Config{})},
		{Name: genutiltypes.ModuleName, Config: appconfig.WrapAny(&genutilmodulev1.Module{})},
		{Name: minttypes.ModuleName, Config: appconfig.WrapAny(&mintmodulev1.Module{})},
		{Name: distrtypes.ModuleName, Config: appconfig.WrapAny(&distrmodulev1.Module{})},
		{Name: govtypes.ModuleName, Config: appconfig.WrapAny(&govmodulev1.Module{})},
		{Name: paramstypes.ModuleName, Config: appconfig.WrapAny(&paramsmodulev1.Module{})},
		{Name: consensustypes.ModuleName, Config: appconfig.WrapAny(&consensusmodulev1.Module{})},
		{Name: upgradetypes.ModuleName, Config: appconfig.WrapAny(&upgrademodulev1.Module{})},
		{Name: evidencetypes.ModuleName, Config: appconfig.WrapAny(&evidencemodulev1.Module{})},
	},
})
