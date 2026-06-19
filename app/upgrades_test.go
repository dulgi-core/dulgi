package app

import (
	"encoding/json"
	"testing"

	coreheader "cosmossdk.io/core/header"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/require"
)

const testChainID = "dulgi-upgrade-test-1"

// TestFirstUpgradeDoesNotReinitializeIBCModules is a regression test for the
// mainnet-blocking panic discovered during the v1.1.0 upgrade rehearsal:
//
//	applying upgrade "v1.1.0" → adding a new module: capability
//	panic: SetIndex requires index to not be set (capability.InitializeIndex)
//
// Root cause: the IBC stack (capability, ibc, transfer, 07-tendermint) is wired
// manually via RegisterModules in app.go rather than through depinject. The
// consensus-version map persisted at InitGenesis omitted them, so the first
// RunMigrations treated them as brand-new modules and re-ran their InitGenesis —
// which panics in capability.InitializeIndex because the capability index was
// already set at genesis.
//
// The fix (app/upgrades.go) seeds the current consensus version of every
// manually-wired module into fromVM before RunMigrations, so their InitGenesis
// is not re-executed. This test drives the real genesis → upgrade flow and
// asserts the upgrade applies cleanly; reverting the fix makes it panic again.
func TestFirstUpgradeDoesNotReinitializeIBCModules(t *testing.T) {
	a := NewDulgiApp(log.NewNopLogger(), dbm.NewMemDB(), nil, true, sims.EmptyAppOptions{}, baseapp.SetChainID(testChainID))

	// Build a one-validator genesis so InitChain returns a non-empty validator
	// set (baseapp rejects an empty set after InitGenesis).
	valPubKey := ed25519.GenPrivKey().PubKey()
	cmtVal := cmttypes.NewValidator(valPubKey, 1)
	valSet := cmttypes.NewValidatorSet([]*cmttypes.Validator{cmtVal})

	accPubKey := secp256k1.GenPrivKey().PubKey()
	acc := authtypes.NewBaseAccount(accPubKey.Address().Bytes(), accPubKey, 0, 0)
	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100_000_000_000_000))),
	}

	genState, err := sims.GenesisStateWithValSet(
		a.AppCodec(), a.DefaultGenesis(), valSet, []authtypes.GenesisAccount{acc}, balance,
	)
	require.NoError(t, err)

	genBytes, err := json.Marshal(genState)
	require.NoError(t, err)

	// Run InitGenesis exactly as a real node does. This is what persists the
	// consensus-version map (and initializes the capability index) against which
	// the upgrade handler later runs.
	_, err = a.InitChain(&abci.RequestInitChain{
		ChainId:         testChainID,
		ConsensusParams: sims.DefaultConsensusParams,
		AppStateBytes:   genBytes,
		InitialHeight:   1,
	})
	require.NoError(t, err)

	// Finalize + commit block 1 so the genesis state — including the
	// already-initialized capability index and the persisted consensus-version
	// map (which omits the manually-wired IBC modules) — is durable and visible
	// to the upgrade below. Without this commit the bug does not reproduce.
	_, err = a.FinalizeBlock(&abci.RequestFinalizeBlock{Height: 1})
	require.NoError(t, err)
	_, err = a.Commit()
	require.NoError(t, err)

	const upgradeHeight = 3

	// Apply the v1.1.0 upgrade against the genesis-initialized state. The keeper
	// reads the persisted fromVM and invokes the registered handler; before the
	// fix this re-ran capability.InitGenesis and panicked.
	ctx := a.NewUncachedContext(false, cmtproto.Header{Height: upgradeHeight}).
		WithHeaderInfo(coreheader.Info{Height: upgradeHeight})
	require.NotPanics(t, func() {
		err = a.UpgradeKeeper.ApplyUpgrade(ctx, upgradetypes.Plan{
			Name:   upgradeName,
			Height: upgradeHeight,
		})
	})
	require.NoError(t, err)

	// The upgrade must be recorded as applied and its plan cleared.
	doneHeight, err := a.UpgradeKeeper.GetDoneHeight(ctx, upgradeName)
	require.NoError(t, err)
	require.Equal(t, int64(upgradeHeight), doneHeight)
	_, err = a.UpgradeKeeper.GetUpgradePlan(ctx)
	require.ErrorIs(t, err, upgradetypes.ErrNoUpgradePlanFound)
}