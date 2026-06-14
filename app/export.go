package app

import (
	"encoding/json"
	"fmt"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// ExportAppStateAndValidators exports the application state for a genesis file.
// When forZeroHeight is true, state is prepared for a fresh chain start at
// height 0 (rewards withdrawn, validator heights reset, etc.).
func (app *DulgiApp) ExportAppStateAndValidators(
	forZeroHeight bool,
	jailAllowedAddrs []string,
	modulesToExport []string,
) (servertypes.ExportedApp, error) {
	ctx := app.NewContextLegacy(true, cmtproto.Header{Height: app.LastBlockHeight()})

	height := app.LastBlockHeight() + 1
	if forZeroHeight {
		height = 0
		app.prepForZeroHeightGenesis(ctx, jailAllowedAddrs)
	}

	genState, err := app.ModuleManager.ExportGenesisForModules(ctx, app.appCodec, modulesToExport)
	if err != nil {
		return servertypes.ExportedApp{}, err
	}

	appState, err := json.MarshalIndent(genState, "", "  ")
	if err != nil {
		return servertypes.ExportedApp{}, err
	}

	validators, err := staking.WriteValidators(ctx, app.StakingKeeper)
	return servertypes.ExportedApp{
		AppState:        appState,
		Validators:      validators,
		Height:          height,
		ConsensusParams: app.BaseApp.GetConsensusParams(ctx),
	}, err
}

// prepForZeroHeightGenesis prepares state for a zero-height (fresh-start)
// genesis export. It withdraws all rewards, resets validator/slashing heights,
// and re-initializes the distribution accumulators so the exported genesis is a
// valid block-0 state.
func (app *DulgiApp) prepForZeroHeightGenesis(ctx sdk.Context, jailAllowedAddrs []string) {
	applyAllowedAddrs := len(jailAllowedAddrs) > 0

	allowedAddrsMap := make(map[string]bool)
	for _, addr := range jailAllowedAddrs {
		if _, err := sdk.ValAddressFromBech32(addr); err != nil {
			panic(fmt.Errorf("invalid validator address in jail-allowed list: %w", err))
		}
		allowedAddrsMap[addr] = true
	}

	// Withdraw all validator commission.
	app.StakingKeeper.IterateValidators(ctx, func(_ int64, val stakingtypes.ValidatorI) (stop bool) {
		valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
		if err != nil {
			panic(err)
		}
		_, _ = app.DistrKeeper.WithdrawValidatorCommission(ctx, valAddr)
		return false
	})

	// Withdraw all delegator rewards.
	dels, err := app.StakingKeeper.GetAllDelegations(ctx)
	if err != nil {
		panic(err)
	}
	for _, delegation := range dels {
		valAddr, err := sdk.ValAddressFromBech32(delegation.ValidatorAddress)
		if err != nil {
			panic(err)
		}
		delAddr, err := sdk.AccAddressFromBech32(delegation.DelegatorAddress)
		if err != nil {
			panic(err)
		}
		if _, err := app.DistrKeeper.WithdrawDelegationRewards(ctx, delAddr, valAddr); err != nil {
			panic(err)
		}
	}

	// Clear validator slash events and historical rewards.
	app.DistrKeeper.DeleteAllValidatorSlashEvents(ctx)
	app.DistrKeeper.DeleteAllValidatorHistoricalRewards(ctx)

	// Reinitialize all validators' distribution records.
	app.StakingKeeper.IterateValidators(ctx, func(_ int64, val stakingtypes.ValidatorI) (stop bool) {
		valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
		if err != nil {
			panic(err)
		}
		scraps, err := app.DistrKeeper.GetValidatorOutstandingRewardsCoins(ctx, valAddr)
		if err != nil {
			panic(err)
		}
		feePool, err := app.DistrKeeper.FeePool.Get(ctx)
		if err != nil {
			panic(err)
		}
		feePool.CommunityPool = feePool.CommunityPool.Add(scraps...)
		if err := app.DistrKeeper.FeePool.Set(ctx, feePool); err != nil {
			panic(err)
		}
		if err := app.DistrKeeper.Hooks().AfterValidatorCreated(ctx, valAddr); err != nil {
			panic(err)
		}
		return false
	})

	// Reinitialize all delegations.
	for _, del := range dels {
		valAddr, err := sdk.ValAddressFromBech32(del.ValidatorAddress)
		if err != nil {
			panic(err)
		}
		delAddr, err := sdk.AccAddressFromBech32(del.DelegatorAddress)
		if err != nil {
			panic(err)
		}
		if err := app.DistrKeeper.Hooks().BeforeDelegationCreated(ctx, delAddr, valAddr); err != nil {
			// new delegation records may not exist yet; ignore not-found
			_ = err
		}
		if err := app.DistrKeeper.Hooks().AfterDelegationModified(ctx, delAddr, valAddr); err != nil {
			_ = err
		}
	}

	// Reset block height on the context for the staking iterations below.
	ctx = ctx.WithBlockHeight(0)

	// Reset validator signing-info start heights and re-set bond heights.
	app.StakingKeeper.IterateValidators(ctx, func(_ int64, val stakingtypes.ValidatorI) (stop bool) {
		valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
		if err != nil {
			panic(err)
		}
		validator, err := app.StakingKeeper.GetValidator(ctx, valAddr)
		if err != nil {
			return false
		}
		validator.UnbondingHeight = 0
		if applyAllowedAddrs && !allowedAddrsMap[valAddr.String()] {
			validator.Jailed = true
		}
		if err := app.StakingKeeper.SetValidator(ctx, validator); err != nil {
			panic(err)
		}
		return false
	})

	if _, err := app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(ctx); err != nil {
		panic(err)
	}
}
