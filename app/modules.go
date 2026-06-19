package app

// Blank imports of each SDK module's root package. These trigger the module's
// init() which registers its `*module.v1.Module` protobuf type with depinject's
// appconfig registry. Without them, AppConfig() fails at runtime with
// "no module registered for type URL ...". We import the keeper/types
// subpackages elsewhere, but only the root package performs registration.
import (
	_ "cosmossdk.io/x/evidence"                       // cosmos.evidence.module.v1.Module
	_ "cosmossdk.io/x/upgrade"                        // cosmos.upgrade.module.v1.Module
	_ "github.com/cosmos/cosmos-sdk/x/auth"           // cosmos.auth.module.v1.Module
	_ "github.com/cosmos/cosmos-sdk/x/auth/tx/config" // cosmos.tx.config.v1.Config
	_ "github.com/cosmos/cosmos-sdk/x/auth/vesting"   // cosmos.vesting.module.v1.Module
	_ "github.com/cosmos/cosmos-sdk/x/bank"           // cosmos.bank.module.v1.Module
	_ "github.com/cosmos/cosmos-sdk/x/consensus"      // cosmos.consensus.module.v1.Module
	_ "github.com/cosmos/cosmos-sdk/x/distribution"   // cosmos.distribution.module.v1.Module
	_ "github.com/cosmos/cosmos-sdk/x/genutil"        // cosmos.genutil.module.v1.Module
	_ "github.com/cosmos/cosmos-sdk/x/gov"            // cosmos.gov.module.v1.Module
	_ "github.com/cosmos/cosmos-sdk/x/mint"           // cosmos.mint.module.v1.Module
	_ "github.com/cosmos/cosmos-sdk/x/params"         // cosmos.params.module.v1.Module
	_ "github.com/cosmos/cosmos-sdk/x/slashing"       // cosmos.slashing.module.v1.Module
	_ "github.com/cosmos/cosmos-sdk/x/staking"        // cosmos.staking.module.v1.Module
)
