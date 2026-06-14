# Dulgi

**Dulgi** is a lightweight, IBC-native Cosmos SDK Layer-1 blockchain focused on
fast token transfers, staking, governance, and cross-chain interoperability —
with every non-essential feature removed.

| Property | Value |
|---|---|
| Chain name | Dulgi |
| Token | Dulgi Coin (**DUL**) |
| Base denom | `udul` (6 decimals; 1 DUL = 1,000,000 udul) |
| Address prefix | `dulgi` |
| Binary | `dulgid` |
| Block time | ~1 second |
| Max validators | 300 |
| Inflation | Fixed 10% / year |
| Unbonding | 21 days |
| Consensus | CometBFT (BFT) |
| Stack | Cosmos SDK v0.50.x · CometBFT v0.38.x · IBC-go v8.x |

**No** CosmWasm · **No** EVM · **No** smart contracts · **No** NFT · **No** ICA/ICQ ·
**No** feegrant/group/circuit/authz. Smart-contract deployment is impossible by
construction — those modules are not compiled into the binary.

---

## Table of Contents

1. [Project structure](#1-project-structure)
2. [Architecture](#2-architecture)
3. [Build instructions](#3-build-instructions)
4. [Validator guide](#4-validator-guide)
5. [Testnet launch guide](#5-testnet-launch-guide)
6. [Mainnet launch guide](#6-mainnet-launch-guide)
7. [IBC relayer setup](#7-ibc-relayer-setup)
8. [Upgrade guide](#8-upgrade-guide)
9. [Security recommendations](#9-security-recommendations)
10. [CLI reference](#10-cli-reference)

---

## 1. Project structure

```
Dulgi/
├── go.mod / go.sum            # Module: github.com/dulgi/dulgi
├── Makefile                   # build / install / docker / devnet targets
├── Dockerfile                 # multi-stage minimal node image
├── docker-compose.yml         # single-node quick deploy
├── app/
│   ├── app.go                 # DulgiApp: runtime.App + manual IBC v8 wiring
│   ├── app_config.go          # depinject module config, account perms, orderings
│   ├── modules.go             # blank imports that register SDK modules w/ depinject
│   ├── upgrades.go            # governance upgrade handlers + store loader
│   └── export.go              # genesis export (incl. zero-height prep)
├── cmd/dulgid/
│   ├── main.go
│   └── cmd/
│       ├── root.go            # NewRootCmd + AutoCLI + ProvideClientContext
│       ├── commands.go        # command tree, newApp, appExport, IBC CLI
│       └── config.go          # tuned CometBFT + app.toml defaults (1s blocks)
├── scripts/
│   ├── configure-genesis.sh   # canonical economic/consensus genesis params
│   ├── single-node.sh         # local single-validator devnet
│   ├── testnet.sh             # local N-validator testnet
│   ├── validator-setup.sh     # join an existing chain as a validator
│   └── mainnet.sh             # mainnet genesis ceremony (token allocations)
├── systemd/dulgid.service     # production service unit (+ cosmovisor notes)
└── README.md
```

## 2. Architecture

### Module set (minimal by design)

Dulgi compiles only the modules required for a transfer/staking/governance chain
with IBC:

| Module | Purpose |
|---|---|
| `auth` (+ `vesting`) | accounts, signatures, vesting allocations |
| `bank` | balances, transfers, denom metadata |
| `staking` | delegated proof-of-stake, 300-validator set |
| `distribution` | block-reward and fee distribution, community pool |
| `slashing` | downtime/double-sign penalties |
| `mint` | fixed 10% annual inflation |
| `gov` | on-chain governance (Msg-based + legacy param-change) |
| `upgrade` | coordinated, height-based binary upgrades |
| `evidence` | equivocation evidence handling |
| `consensus` | consensus-param governance (required in SDK v0.50) |
| `params` | legacy param subspaces (IBC migration support) |
| `genutil` | gentx / genesis tooling |
| `ibc` (core) | IBC clients/connections/channels |
| `transfer` (ICS-20) | fungible cross-chain token transfers |
| `capability` | object-capability keys for IBC ports/channels |
| `07-tendermint` | IBC Tendermint light client |

Everything else (CosmWasm, EVM, NFT, ICA, ICQ, feegrant, group, circuit, authz,
crisis, simulation) is **not imported**, so the attack surface, dependency
count, binary size, and state footprint are all minimized.

### Wiring

`app_config.go` declares the SDK modules with **depinject** (the modern
`runtime.App` approach). IBC-go v8 is not yet depinject-native, so the IBC
keepers (`capability`, `ibc`, `transfer`) are wired manually in `app.go` after
the runtime app is built, their stores registered via `RegisterStores`, and the
modules spliced into the manager via `RegisterModules`. The module execution
order (begin/end-blockers, init/export genesis, preblockers) is declared on the
runtime module config and applied by `app.Load()` — including the IBC modules,
which is why their names appear in the order slices in `app_config.go`.

### Block production (~1s)

CometBFT's block cadence is governed primarily by `timeout_commit`. We pin it to
**1s** and keep the propose/prevote/precommit timeouts small so a healthy round
finishes well inside the commit window, making 1s the binding constraint. See
`cmd/dulgid/cmd/config.go` for every tuned value and the rationale.

### Inflation & rewards

`mint` mints new `udul` every block. By setting `inflation_min == inflation_max
== 0.10` and `inflation_rate_change = 0`, inflation is a constant **10%/year**
regardless of bonded ratio. `blocks_per_year = 31,536,000` matches 1s blocks so
the per-block mint equals the intended annual rate. Minted coins flow to the
fee-collector, then `distribution` pays validators (with commission) and their
delegators each block; `community_tax` (2%) funds the community pool.

## 3. Build instructions

### Prerequisites
- Go **1.23+**
- `make`, `git`, a C toolchain (for cgo/leveldb)
- `jq` (runtime dependency for the genesis scripts)

### Build
```bash
git clone https://github.com/dulgi/dulgi.git
cd dulgi
make install          # installs dulgid into $(go env GOPATH)/bin
# or
make build            # outputs ./build/dulgid
dulgid version --long
```

### Docker
```bash
make docker-build
docker compose up -d        # initializes + starts a single node
docker compose logs -f
```

## 4. Validator guide

### Quick local validator (devnet)
```bash
bash scripts/single-node.sh
dulgid start --home ~/.dulgi --minimum-gas-prices 0.025udul
```

### Join an existing network
```bash
export CHAIN_ID=dulgi-1
export GENESIS_URL=https://.../genesis.json
export SEEDS="<nodeid>@seed1:26656,<nodeid>@seed2:26656"
# Optional fast bootstrap:
export STATE_SYNC_RPC1=https://rpc1:443 STATE_SYNC_RPC2=https://rpc2:443
bash scripts/validator-setup.sh
```
Then start and fully sync the node, fund your operator key, and run the
`create-validator` step printed by the script. Recommended commission settings
respect the **5% minimum commission rate**:
```
--commission-rate 0.05 --commission-max-rate 0.20 --commission-max-change-rate 0.01
```

### Key validator operations
```bash
# delegate / redelegate
dulgid tx staking delegate   <valoper> 1000000udul --from <key> --chain-id dulgi-1
dulgid tx staking redelegate <src-valoper> <dst-valoper> 1000000udul --from <key> --chain-id dulgi-1
# unjail after downtime
dulgid tx slashing unjail --from <key> --chain-id dulgi-1
# withdraw rewards + commission
dulgid tx distribution withdraw-rewards <valoper> --commission --from <key> --chain-id dulgi-1
```

### Slashing parameters
| Event | Penalty |
|---|---|
| Double sign | 5% of stake (`slash_fraction_double_sign = 0.05`) + permanent tombstone |
| Downtime | 0.1% of stake (`slash_fraction_downtime = 0.001`) + jail |
| Downtime window | 10,000 blocks, ≥5% signed required |

**Protect against double-signing**: never run two validators with the same
`priv_validator_key.json`. Use a sentry architecture and/or a remote signer
(tmkms) in production.

## 5. Testnet launch guide

Local multi-validator testnet on one host:
```bash
N=4 CHAIN_ID=dulgi-testnet-1 bash scripts/testnet.sh
# then start each node (separate terminals), as printed:
dulgid start --home ~/.dulgi-testnet/node0 --minimum-gas-prices 0.025udul
dulgid start --home ~/.dulgi-testnet/node1 --minimum-gas-prices 0.025udul
# ...
```
The script funds each validator, applies the canonical genesis params, gathers
all gentxs, and wires `persistent_peers` + distinct port blocks per node.

## 6. Mainnet launch guide

Mainnet uses a coordinated genesis ceremony.

1. **Coordinator** builds the genesis with the canonical allocations:
   ```bash
   export CHAIN_ID=dulgi-1
   export ADDR_COMMUNITY=dulgi1...   # 40,000,000 DUL
   export ADDR_ECOSYSTEM=dulgi1...   # 25,000,000 DUL
   export ADDR_TREASURY=dulgi1...    # 15,000,000 DUL
   export ADDR_TEAM=dulgi1...        # 10,000,000 DUL
   export ADDR_LIQUIDITY=dulgi1...   # 10,000,000 DUL
   bash scripts/mainnet.sh
   ```
   **Total genesis supply: 100,000,000 DUL.**

   | Allocation | % | DUL | udul |
   |---|---:|---:|---:|
   | Community | 40% | 40,000,000 | 40,000,000,000,000 |
   | Ecosystem | 25% | 25,000,000 | 25,000,000,000,000 |
   | Treasury | 15% | 15,000,000 | 15,000,000,000,000 |
   | Team | 10% | 10,000,000 | 10,000,000,000,000 |
   | Liquidity | 10% | 10,000,000 | 10,000,000,000,000 |

2. **Distribute** the `genesis.json` to all genesis validators.
3. **Each validator** funds its operator key from one of the allocation
   accounts (or is pre-funded) and submits a gentx (≥1,000,000 DUL
   self-delegation).
4. **Coordinator** collects gentxs and publishes the final genesis:
   ```bash
   GENTX_DIR=/path/to/collected/gentxs bash scripts/mainnet.sh
   ```
5. Publish the final `genesis.json` (with its sha256) and seed node IDs.
6. Validators start at the agreed genesis time.

> Vesting allocations (e.g. Team) can be created with
> `dulgid genesis add-genesis-account <addr> <amt> --vesting-amount <amt> --vesting-end-time <unix>`.

## 7. IBC relayer setup

Dulgi is compatible with standard Cosmos relayers (Hermes, `rly`). Example with
**Hermes**:

```toml
# ~/.hermes/config.toml (Dulgi chain entry)
[[chains]]
id = "dulgi-1"
type = "CosmosSdk"
rpc_addr = "http://<rpc>:26657"
grpc_addr = "http://<grpc>:9090"
event_source = { mode = "push", url = "ws://<rpc>:26657/websocket", batch_delay = "200ms" }
account_prefix = "dulgi"
key_name = "relayer"
gas_price = { price = 0.025, denom = "udul" }
gas_multiplier = 1.3
trusting_period = "14days"     # < 21d unbonding
clock_drift = "10s"
max_block_time = "5s"
```

```bash
hermes keys add --chain dulgi-1 --mnemonic-file relayer.mnemonic
hermes create channel --a-chain dulgi-1 --b-chain <counterparty> \
  --a-port transfer --b-port transfer --new-client-connection
hermes start
```

Send an ICS-20 transfer from Dulgi:
```bash
dulgid tx ibc-transfer transfer transfer channel-0 <recipient> 1000000udul \
  --from <key> --chain-id dulgi-1
dulgid query ibc channel channels
```

## 8. Upgrade guide

Upgrades are coordinated on-chain via `x/upgrade`:

1. Implement the new binary; register a handler for the upgrade name in
   `app/upgrades.go` (`upgradeName`, plus any `StoreUpgrades`).
2. Submit a software-upgrade governance proposal naming that upgrade and a
   target height.
3. On approval, every node halts at the height. Validators swap to the new
   binary (or let **cosmovisor** do it — see `systemd/dulgid.service`) and
   restart. A node without a matching handler halts instead of forking — this
   is the safety guarantee.

```bash
# submit (Msg-based) software upgrade proposal
dulgid tx gov submit-proposal <proposal.json> --from <key> --chain-id dulgi-1
dulgid query gov proposals
dulgid tx gov vote <id> yes --from <key> --chain-id dulgi-1
```

## 9. Security recommendations

- **Minimum gas price**: keep `minimum-gas-prices` non-zero (default
  `0.025udul`) on every node — the primary anti-spam control.
- **Mempool limits**: tx size capped at 1 MiB, mempool at 128 MiB / 5,000 txs
  (`config.go`) to bound memory and resist flooding.
- **Block limits**: `max_bytes = 5 MiB`, `max_gas = 100,000,000` in genesis.
- **Validator key isolation**: sentry nodes + remote signer (tmkms); never
  duplicate `priv_validator_key.json`.
- **Pruning**: default `custom` (keep 100, interval 10) to limit disk growth;
  use `nothing` only for archive nodes.
- **State validation**: the upgrade store loader applies migrations at the exact
  governed height; `validate-genesis` is run by every launch script.
- **Firewall**: expose only `26656` (p2p) publicly; keep `26657/9090/1317`
  behind auth/reverse-proxy or localhost.
- **Run as non-root** under the hardened systemd unit (`ProtectSystem`,
  `NoNewPrivileges`, restricted `ReadWritePaths`).

## 10. CLI reference

```bash
# Node lifecycle
dulgid init <moniker> --chain-id dulgi-1 --default-denom udul
dulgid start --minimum-gas-prices 0.025udul
dulgid status

# Keys
dulgid keys add <name>
dulgid keys list

# Bank
dulgid tx bank send <from> <to> 1000000udul --chain-id dulgi-1
dulgid query bank balances <addr>

# Staking
dulgid tx staking delegate <valoper> 1000000udul --from <key> --chain-id dulgi-1
dulgid tx staking redelegate <src> <dst> 1000000udul --from <key> --chain-id dulgi-1
dulgid query staking validators

# Governance
dulgid tx gov vote <id> yes --from <key> --chain-id dulgi-1
dulgid query gov proposals

# IBC
dulgid query ibc channel channels
dulgid tx ibc-transfer transfer transfer channel-0 <recipient> 1000000udul --from <key> --chain-id dulgi-1
```

---

### License

Apache-2.0 (recommended for Cosmos SDK chains). Add a `LICENSE` file before
public release.