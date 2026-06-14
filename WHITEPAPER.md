# Dulgi

### A Fun-First Rest Stop for Blockchain Networks

**Whitepaper · v1.0**
**Chain: Dulgi · Token: Dulgi Coin (DUL)**

---

## Abstract

The multi-chain era is not a destination — it is a journey. Value, messages, and
users move endlessly between sovereign networks along the highways of the
Inter-Blockchain Communication protocol (IBC). What that journey lacks is a good
place to *stop*: a fast, friendly, dependable waypoint where assets can land,
rest, and continue — without paying the tax of a bloated, over-featured chain.

**Dulgi** (둘기 — the carrier pigeon, the original cross-network messenger) is
that rest stop. It is a lightweight, IBC-native Cosmos SDK Layer-1 blockchain
that does a small number of things exceptionally well: fast token transfers,
clean staking, transparent governance, and first-class cross-chain
interoperability. Every non-essential feature has been deliberately removed.
There are no smart contracts, no virtual machine, and no surface for the kinds
of complexity that turn blockchains into something heavy, slow, and joyless.

Dulgi's thesis is simple and slightly contrarian: **a blockchain can be fun.**
Fun is not a gimmick layered on top — it is the direct, felt consequence of a
chain that is fast (~1-second blocks), legible (a handful of well-understood
modules), safe (a minimal attack surface), and welcoming (low friction for
validators, relayers, and users). This document explains the philosophy, the
architecture, the economics, and the roadmap behind that idea.

---

## 1. The Rest Stop Thesis

### 1.1 The highway and the waystation

On any real highway, the rest stop is not the least important building — it is
the one travelers actually remember. It is where you refuel, stretch, grab
something good, and decide where to go next. It is small on purpose. It does not
try to be a city. Its entire value is that it is *reliably there*, *quick to get
in and out of*, and *pleasant to pass through*.

The interchain has highways — IBC connects hundreds of sovereign chains — but it
is short on great rest stops. Many general-purpose L1s try to be the whole city:
execution layer, DeFi hub, NFT marketplace, gaming platform, and settlement
network all at once. That ambition carries a cost. More features mean more
state, more dependencies, more consensus-critical code, a larger attack surface,
and a slower, more intimidating experience for everyone who just wants to *move
value and keep going*.

Dulgi makes the opposite bet. It is the rest stop, not the city:

- **You arrive quickly.** ~1-second finality means an asset that lands on Dulgi
  is usable almost immediately.
- **You can always leave.** Native ICS-20 transfers and standard relayer support
  mean Dulgi is a hub you pass *through*, never a roach motel you get stuck in.
- **Nothing surprises you.** A small, audited-by-construction module set means
  the chain behaves the way the documentation says it does.
- **It's a nice place to be.** Cheap fees, predictable rewards, and simple
  mental models make Dulgi pleasant for users, validators, and relayers alike.

### 1.2 Why "fun" is an engineering target, not a slogan

"Fun" sounds soft. In Dulgi it is a precise design constraint that decomposes
into measurable properties:

| Felt experience | Engineering property |
|---|---|
| "It's snappy." | ~1s block time; tuned CometBFT timeouts. |
| "I understand it." | ~16 modules, no VM, no contract semantics to reason about. |
| "It feels safe." | Minimized attack surface; non-compiled modules cannot be exploited. |
| "It's cheap to use." | Low gas, predictable fixed inflation, simple fee flow. |
| "I'm not locked in." | IBC-native; leaving is as easy as arriving. |
| "I can run one." | Lightweight binary, clear validator scripts, low hardware bar. |

Joy, in software, is mostly the absence of friction and the presence of
predictability. Dulgi engineers for both.

### 1.3 The pigeon

The project's name is its mascot and its mission statement. The carrier pigeon
(둘기) is the oldest cross-network messenger humanity ever trusted: small,
unglamorous, astonishingly reliable, and built for exactly one job — *getting a
message from here to there.* Dulgi the chain is the carrier pigeon of the
interchain. It does not want to be the message, the sender, or the empire. It
wants to fly the route, every second, without fail — and to make the trip feel
light.

---

## 2. Design Philosophy

### 2.1 Subtraction as a feature

Most blockchains are defined by what they add. Dulgi is defined by what it
refuses to add. The following are **not compiled into the binary** and are
therefore impossible to deploy or invoke by construction:

- **No CosmWasm** — no WebAssembly smart contracts.
- **No EVM** — no Ethereum-compatible execution layer.
- **No smart contracts of any kind.**
- **No NFT module.**
- **No Interchain Accounts (ICA) / Interchain Queries (ICQ).**
- **No feegrant, group, circuit-breaker, or authz modules.**

This is not laziness; it is the core security and performance argument. Code
that does not exist cannot be exploited, cannot bloat state, cannot slow down
block production, and cannot confuse a user about what the chain will do.
Subtraction shrinks the attack surface, the dependency tree, the binary size,
and the cognitive load — all at once.

### 2.2 Legibility over generality

A general-purpose execution layer can do anything, which means no one — not
users, not auditors, not validators — can fully predict what it *will* do.
Dulgi chooses legibility. Every state transition on the chain comes from one of
a small, fixed, well-understood set of Cosmos SDK modules. There is no
user-supplied code path. The chain you read about in this document is exactly
the chain you run.

### 2.3 Sovereignty and exit

A good rest stop never traps its visitors. Dulgi is IBC-native and relayer-
agnostic. Assets enter and leave over the same open, standardized rails. There
is no proprietary bridge, no privileged exit, and no lock-in. Dulgi earns its
traffic by being good, not by being inescapable.

---

## 3. Architecture

### 3.1 Stack

Dulgi is built on a mature, battle-tested foundation:

| Layer | Technology |
|---|---|
| Application framework | Cosmos SDK v0.50.x (`runtime.App` + depinject) |
| Consensus engine | CometBFT v0.38.x (BFT, instant finality) |
| Interoperability | IBC-go v8.x (ICS-20 fungible token transfer) |
| Language | Go 1.23+ |

### 3.2 Module set (minimal by design)

Dulgi compiles only the modules required to be an excellent transfer, staking,
governance, and interoperability chain:

| Module | Purpose |
|---|---|
| `auth` (+ `vesting`) | Accounts, signatures, vesting allocations. |
| `bank` | Balances, transfers, denom metadata. |
| `staking` | Delegated proof-of-stake; up to 300 validators. |
| `distribution` | Block-reward and fee distribution; community pool. |
| `slashing` | Downtime and double-sign penalties. |
| `mint` | Fixed 10% annual inflation. |
| `gov` | On-chain governance (Msg-based + legacy param-change). |
| `upgrade` | Coordinated, height-based binary upgrades. |
| `evidence` | Equivocation evidence handling. |
| `consensus` | Consensus-parameter governance (required in SDK v0.50). |
| `params` | Legacy param subspaces (IBC migration support). |
| `genutil` | Genesis and gentx tooling. |
| `ibc` (core) | IBC clients, connections, and channels. |
| `transfer` (ICS-20) | Fungible cross-chain token transfers. |
| `capability` | Object-capability keys for IBC ports/channels. |
| `07-tendermint` | IBC Tendermint light client. |

Everything outside this list is intentionally absent.

### 3.3 Wiring

The SDK modules are declared with **depinject** via the modern `runtime.App`
approach. Because IBC-go v8 is not yet depinject-native, the IBC keepers
(`capability`, `ibc`, `transfer`) are wired manually after the runtime app is
built: their stores are registered explicitly, and the modules are spliced into
the module manager. Module execution ordering — begin/end-blockers, init/export
genesis, and preblockers — is declared on the runtime configuration and applied
at load time, including the IBC modules. This hybrid wiring keeps the codebase
both modern and explicit, with no hidden magic in the consensus-critical path.

### 3.4 Block production (~1 second)

CometBFT's block cadence is governed primarily by `timeout_commit`. Dulgi pins
this to **1 second** and keeps the propose/prevote/precommit timeouts small, so
a healthy consensus round finishes comfortably inside the commit window — making
1-second blocks the binding constraint rather than an aspirational target. The
result is a chain that *feels* instant: the practical experience of arriving at
the rest stop and being immediately ready to move on.

---

## 4. Tokenomics

### 4.1 The token

| Property | Value |
|---|---|
| Token name | Dulgi Coin |
| Symbol | DUL |
| Base denom | `udul` |
| Precision | 6 decimals (1 DUL = 1,000,000 udul) |
| Address prefix | `dulgi` |
| Genesis supply | 100,000,000 DUL |

DUL is the chain's gas token, staking token, and governance token. It pays the
fees for the transfers that pass through the rest stop, secures the network
through delegated proof-of-stake, and confers a voice in on-chain governance.

### 4.2 Genesis allocation

The full genesis supply of **100,000,000 DUL** is allocated as follows:

| Allocation | Share | DUL |
|---|---:|---:|
| Community | 40% | 40,000,000 |
| Ecosystem | 25% | 25,000,000 |
| Treasury | 15% | 15,000,000 |
| Team | 10% | 10,000,000 |
| Liquidity | 10% | 10,000,000 |

- **Community (40%)** — the largest single allocation, reflecting that a rest
  stop exists for its travelers. Reserved for distribution, incentives, and
  community programs.
- **Ecosystem (25%)** — grants and support for relayers, tooling, integrations,
  and partner chains that make Dulgi a better hub.
- **Treasury (15%)** — long-term operational reserve, governed transparently.
- **Team (10%)** — contributors, subject to vesting to align long-term
  incentives (vesting allocations are supported natively at genesis).
- **Liquidity (10%)** — initial market and cross-chain liquidity provisioning.

### 4.3 Inflation and rewards

Dulgi uses a **fixed 10% annual inflation** rate. By configuring the `mint`
module with `inflation_min == inflation_max == 0.10` and a zero rate-of-change,
inflation is constant regardless of the bonded ratio — predictable, easy to
reason about, and free of the reflexive dynamics that complicate variable-
inflation chains.

The `blocks_per_year` parameter (31,536,000) is matched to 1-second blocks, so
the per-block mint equals the intended annual rate. Newly minted `udul` flows to
the fee collector; the `distribution` module then pays validators (net of their
commission) and their delegators every block. A **2% community tax** funds the
community pool, giving governance an ongoing, on-chain budget without requiring
new token issuance decisions.

### 4.4 Fees

Fees are paid in DUL at a low, predictable minimum gas price (default
`0.025 udul`). Low fees are not a marketing promise — they are the natural
consequence of a chain that does little per transaction and never has to price
in the risk of arbitrary contract execution.

---

## 5. Consensus and Security

### 5.1 Consensus

Dulgi runs on **CometBFT**, a production-grade Byzantine Fault Tolerant
consensus engine offering single-slot, instant finality. There are no
probabilistic reorganizations: once a block is committed, it is final. The
active validator set holds up to **300 validators**, secured by delegated
proof-of-stake.

### 5.2 Slashing and validator discipline

| Event | Penalty |
|---|---|
| Double sign | 5% of stake slashed + permanent tombstone. |
| Downtime | 0.1% of stake slashed + jail. |
| Downtime window | 10,000 blocks; ≥5% of blocks must be signed. |

Validators are expected to protect their signing keys rigorously — never running
two nodes with the same `priv_validator_key.json`, and using a sentry
architecture and/or a remote signer (e.g. tmkms) in production.

### 5.3 The minimal-surface security argument

Dulgi's strongest security property is structural. The classes of exploit that
dominate blockchain incident reports — reentrancy, malicious or buggy smart
contracts, VM edge cases, bridge-contract drains, and unbounded user-supplied
execution — **simply have no foothold here**, because the code that would enable
them is not in the binary. Security audits can therefore concentrate on a small,
stable, well-understood module set rather than an open-ended execution
environment.

### 5.4 Operational hardening

- **Anti-spam:** non-zero minimum gas price on every node as the primary
  control.
- **Mempool bounds:** transaction size capped at 1 MiB; mempool capped at
  128 MiB / 5,000 transactions.
- **Block bounds:** `max_bytes = 5 MiB`, `max_gas = 100,000,000`.
- **Pruning:** sensible defaults to limit disk growth, with archive mode
  available.
- **Network exposure:** only the p2p port is intended to be public; RPC, gRPC,
  and REST endpoints sit behind authentication or a reverse proxy.
- **Process isolation:** a hardened systemd unit runs the node as a non-root
  user with restricted filesystem access.

### 5.5 Safe upgrades

Upgrades are coordinated entirely on-chain through the `upgrade` module. A
governance proposal names an upgrade and a target height; on approval, every
node halts at that height and validators swap to the new binary (manually or via
cosmovisor). Crucially, a node *without* the matching upgrade handler halts
rather than forking — a built-in safety guarantee against accidental chain
splits.

---

## 6. Interoperability — The Heart of the Rest Stop

Interoperability is not a feature of Dulgi; it is Dulgi's reason to exist. A rest
stop with no roads in or out is just a building in a field.

Dulgi is **IBC-native** and compatible with the standard Cosmos relayer
ecosystem (Hermes, `rly`). ICS-20 fungible token transfers let any IBC-connected
chain send assets to Dulgi and receive them back over the same open standard.
There is no custom bridge to trust and no privileged operator in the path — only
the audited, light-client-secured IBC protocol that the broader interchain
already relies on.

This is the literal mechanism behind the rest stop metaphor: assets traveling
across the interchain *pull in* to Dulgi, where ~1-second finality makes them
immediately usable, and *pull out* whenever their journey continues. Dulgi adds
value by being a fast, cheap, dependable place along the route — and by never
standing between a traveler and the exit.

---

## 7. Governance

Dulgi is governed on-chain by its stakeholders through the `gov` module,
supporting both modern Msg-based proposals and legacy parameter-change
proposals. DUL holders — directly or through their chosen validators — can:

- adjust chain parameters within the bounds the modules permit,
- direct the community pool funded by the 2% community tax,
- approve coordinated software upgrades, and
- steer the long-term direction of the network.

Governance embodies the same philosophy as the rest of the chain: a small number
of clear, consequential levers rather than an unbounded surface of knobs. Easy to
understand, hard to misuse.

---

## 8. Running a Node

Dulgi is deliberately easy to operate — a low barrier to entry is part of being
welcoming. The single binary `dulgid` handles initialization, node lifecycle,
keys, transactions, queries, and IBC operations. Provided scripts cover the full
operator lifecycle:

- `single-node.sh` — a local single-validator devnet.
- `testnet.sh` — a local multi-validator testnet on one host.
- `validator-setup.sh` — join an existing chain as a validator, with optional
  state-sync fast bootstrap.
- `mainnet.sh` — the coordinated mainnet genesis ceremony with canonical token
  allocations.

A minimal feature set means a lightweight binary, a modest hardware footprint,
and a node that is genuinely pleasant to run — closing the loop on the "fun"
thesis for operators, not just end users.

---

## 9. Roadmap

Dulgi's roadmap deliberately resists feature creep. Progress is measured by
reach, reliability, and delight — never by surface area.

**Phase 1 — Open the rest stop (Mainnet).**
Coordinated genesis ceremony, validator onboarding, and a stable, IBC-native
mainnet with ~1s blocks.

**Phase 2 — Connect the highways.**
Establish IBC channels to major hub and partner chains; grow a healthy, well-
incentivized relayer set so traffic flows freely in every direction.

**Phase 3 — Make the trip delightful.**
First-class wallet and explorer experiences, transparent dashboards for fees,
rewards, and channel health, and ecosystem grants for tools that reduce friction
for travelers.

**Phase 4 — Hand over the keys.**
Progressive decentralization of parameters and treasury to on-chain governance,
so the community that travels through the rest stop is the community that runs
it.

Throughout every phase, the constraint is constant: **Dulgi stays small, stays
fast, and stays fun.**

---

## 10. Conclusion

The interchain does not need another city. It needs more great rest stops —
fast, friendly, trustworthy places to land, transact, and continue. Dulgi is
built to be exactly that: a lightweight, IBC-native Layer-1 that pursues fun as a
first-class engineering goal and earns its traffic by being a genuine pleasure to
pass through.

By subtracting everything non-essential, Dulgi gains everything that matters —
speed, legibility, safety, and joy. It is the carrier pigeon of the interchain:
small, reliable, and built for one beautiful purpose — *getting value from here
to there, and making the trip feel light.*

Welcome to the rest stop.

---

*This document describes the design and intent of the Dulgi network. It is
informational and does not constitute financial, investment, or legal advice.
Network parameters described herein are governed on-chain and may be changed by
the community through the governance process.*