package app

import (
	"context"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/types/module"
)

// upgradeName is the on-chain name of the next planned upgrade. Governance
// schedules an upgrade by passing a software-upgrade proposal whose Plan.Name
// matches this string; every validator must run a binary that registers a
// handler for it, otherwise the chain halts at the upgrade height (the intended
// upgrade-safety behavior).
const upgradeName = "v1.1.0"

// RegisterUpgradeHandlers wires upgrade handlers and store loaders. It is
// called from NewDulgiApp after the module manager is fully assembled.
//
// Upgrade-safety guarantees:
//   - The handler runs module migrations deterministically on every node.
//   - StoreUpgrades are applied at the exact upgrade height read from disk, so
//     a node started with the new binary before that height still loads the old
//     store layout until the planned height is reached.
//   - A height-matched upgrade with no registered handler halts the node
//     instead of forking.
func (app *DulgiApp) RegisterUpgradeHandlers() {
	app.UpgradeKeeper.SetUpgradeHandler(
		upgradeName,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			// The IBC-related modules (capability, ibc, transfer, 07-tendermint)
			// are wired manually via RegisterModules rather than through
			// depinject, so the consensus-version map persisted at InitGenesis
			// omits them. Without seeding their current versions here, the first
			// RunMigrations treats them as brand-new modules and re-runs their
			// InitGenesis — which panics in capability.InitializeIndex on the
			// already-initialized store ("SetIndex requires index to not be
			// set"). Seed any missing manual module so its InitGenesis is not
			// re-executed.
			for _, name := range []string{"capability", "ibc", "transfer", "07-tendermint"} {
				if _, ok := fromVM[name]; ok {
					continue
				}
				if m, ok := app.ModuleManager.Modules[name].(module.HasConsensusVersion); ok {
					fromVM[name] = m.ConsensusVersion()
				}
			}
			// For a routine upgrade with no state changes this simply records
			// the new consensus versions of each module.
			return app.ModuleManager.RunMigrations(ctx, app.Configurator(), fromVM)
		},
	)

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Errorf("failed to read upgrade info from disk: %w", err))
	}

	if upgradeInfo.Name == upgradeName && !app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		// Declare store additions/renames/deletions a future module set
		// introduces. Empty for routine upgrades.
		storeUpgrades := storetypes.StoreUpgrades{
			Added:   []string{},
			Renamed: []storetypes.StoreRename{},
			Deleted: []string{},
		}
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
	}
}
