package cmd

import (
	"time"

	cmtcfg "github.com/cometbft/cometbft/config"

	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
)

// initCometBFTConfig returns Dulgi's tuned CometBFT (config.toml) defaults.
//
// Block-time strategy (target: stable 1-second blocks)
// ----------------------------------------------------
// CometBFT's effective block interval is dominated by `timeout_commit`, which
// is the minimum time the proposer waits after committing a block before
// proposing the next one. We pin it to 1s and keep every other consensus
// timeout small so that, under healthy network conditions, a height completes
// well within the commit window and the 1s floor is the binding constraint —
// giving predictable, jitter-free 1s blocks. The *Delta values add a little
// slack per round so the network still converges safely if a round fails
// (liveness over latency on bad rounds), without affecting the happy path.
func initCometBFTConfig() *cmtcfg.Config {
	cfg := cmtcfg.DefaultConfig()

	// --- Consensus timeouts ---------------------------------------------------
	cfg.Consensus.TimeoutPropose = 600 * time.Millisecond        // time to wait for a proposal
	cfg.Consensus.TimeoutProposeDelta = 100 * time.Millisecond   // per-round increment
	cfg.Consensus.TimeoutPrevote = 200 * time.Millisecond        // time to wait for +2/3 prevotes
	cfg.Consensus.TimeoutPrevoteDelta = 100 * time.Millisecond   // per-round increment
	cfg.Consensus.TimeoutPrecommit = 200 * time.Millisecond      // time to wait for +2/3 precommits
	cfg.Consensus.TimeoutPrecommitDelta = 100 * time.Millisecond // per-round increment
	cfg.Consensus.TimeoutCommit = 1000 * time.Millisecond        // <-- 1s block interval floor

	// Empty blocks are still produced (no CreateEmptyBlocksInterval delay) so
	// IBC/relayer timeouts and light clients always have fresh headers.
	cfg.Consensus.CreateEmptyBlocks = true
	cfg.Consensus.CreateEmptyBlocksInterval = 0

	// --- Mempool --------------------------------------------------------------
	// Bounded mempool to cap RAM and provide anti-spam back-pressure.
	cfg.Mempool.Size = 5000                     // max txs queued
	cfg.Mempool.MaxTxsBytes = 128 * 1024 * 1024 // 128 MiB total mempool cap
	cfg.Mempool.MaxTxBytes = 1024 * 1024        // 1 MiB per-tx ceiling (DoS guard)
	cfg.Mempool.CacheSize = 10000               // dedupe cache for already-seen txs
	cfg.Mempool.Recheck = true                  // re-validate txs after each block
	cfg.Mempool.KeepInvalidTxsInCache = false

	// --- P2P networking -------------------------------------------------------
	// Modest peer counts keep gossip/CPU overhead low for a lightweight node
	// while remaining well-connected enough for a 300-validator set.
	cfg.P2P.MaxNumInboundPeers = 40
	cfg.P2P.MaxNumOutboundPeers = 15
	cfg.P2P.FlushThrottleTimeout = 10 * time.Millisecond // faster gossip flush for 1s blocks
	cfg.P2P.SendRate = 20_480_000                        // 20 MB/s
	cfg.P2P.RecvRate = 20_480_000                        // 20 MB/s

	// --- Storage / RPC --------------------------------------------------------
	// Discard the tx index by default to slash disk growth; operators that need
	// historical tx queries (explorers) flip this to "kv" in config.toml.
	cfg.TxIndex.Indexer = "null"

	return cfg
}

// initAppConfig returns Dulgi's tuned app.toml defaults and template.
func initAppConfig() (string, interface{}) {
	// Embed the standard server config; Dulgi adds no custom app sections, so
	// the squashed base config is sufficient.
	type CustomAppConfig struct {
		serverconfig.Config `mapstructure:",squash"`
	}

	srvCfg := serverconfig.DefaultConfig()

	// --- Anti-spam: minimum gas price -----------------------------------------
	// Every node rejects txs whose offered gas price is below this floor. A
	// non-zero default is critical spam protection; operators may raise it.
	srvCfg.MinGasPrices = "0.025udul"

	// --- Storage growth: aggressive pruning -----------------------------------
	// Keep only recent state to minimize disk usage. Archive nodes should set
	// pruning = "nothing" instead.
	srvCfg.Pruning = "custom"
	srvCfg.PruningKeepRecent = "100"
	srvCfg.PruningInterval = "10"

	// --- Fast sync: state-sync snapshots --------------------------------------
	// Periodic snapshots let new nodes join via state-sync in minutes instead
	// of replaying the full chain.
	srvCfg.StateSync.SnapshotInterval = 1000
	srvCfg.StateSync.SnapshotKeepRecent = 2

	// --- Low RAM: IAVL fast node ----------------------------------------------
	srvCfg.IAVLDisableFastNode = false // fast-node index speeds reads; keep on
	srvCfg.IAVLCacheSize = 781250      // ~ default; bounded IAVL cache

	// --- Service endpoints (relayers / explorers) -----------------------------
	srvCfg.API.Enable = true
	srvCfg.API.Swagger = false // drop swagger UI to keep the binary/footprint lean
	srvCfg.GRPC.Enable = true
	srvCfg.GRPCWeb.Enable = false // not needed for standard relayers

	// --- Mempool back-pressure (app side) -------------------------------------
	// Cap the app-side prioritized mempool to bound memory.
	srvCfg.Mempool.MaxTxs = 5000

	return serverconfig.DefaultConfigTemplate, CustomAppConfig{Config: *srvCfg}
}
