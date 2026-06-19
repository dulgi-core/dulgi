#!/usr/bin/env bash
#
# configure-genesis.sh — apply Dulgi's canonical economic & consensus genesis
# parameters to an already-initialized genesis.json.
#
# Usage:  scripts/configure-genesis.sh [HOME_DIR]
#   HOME_DIR defaults to ~/.dulgi
#
# Requires: jq
#
# This script is the single source of truth for Dulgi's on-chain parameters and
# is invoked by single-node.sh, testnet.sh and mainnet.sh.
set -euo pipefail

HOME_DIR="${1:-$HOME/.dulgi}"
GENESIS="$HOME_DIR/config/genesis.json"
DENOM="udul"

command -v jq >/dev/null || { echo "error: jq is required"; exit 1; }
[ -f "$GENESIS" ] || { echo "error: genesis not found at $GENESIS (run 'dulgid init' first)"; exit 1; }

# --- Time constants (seconds) ------------------------------------------------
UNBONDING_TIME="1814400s"   # 21 days
DOWNTIME_JAIL="600s"        # 10 minutes
MAX_DEPOSIT_PERIOD="172800s" # 2 days
VOTING_PERIOD="259200s"     # 3 days
EXPEDITED_VOTING="86400s"   # 1 day

# --- Inflation: fixed ~10% / year --------------------------------------------
# With 1-second blocks there are 60*60*24*365 = 31,536,000 blocks per year.
# Pinning inflation_min == inflation_max and rate_change = 0 yields a constant
# 10% annual inflation regardless of the bonded ratio.
BLOCKS_PER_YEAR="31536000"
INFLATION="0.100000000000000000"

tmp="$(mktemp)"

jq \
  --arg denom "$DENOM" \
  --arg unbonding "$UNBONDING_TIME" \
  --arg inflation "$INFLATION" \
  --arg bpy "$BLOCKS_PER_YEAR" \
  --arg downtime "$DOWNTIME_JAIL" \
  --arg maxdep "$MAX_DEPOSIT_PERIOD" \
  --arg voting "$VOTING_PERIOD" \
  --arg exvoting "$EXPEDITED_VOTING" \
'
# ---- staking ----------------------------------------------------------------
.app_state.staking.params.bond_denom = $denom
| .app_state.staking.params.unbonding_time = $unbonding
| .app_state.staking.params.max_validators = 300
| .app_state.staking.params.max_entries = 7
| .app_state.staking.params.historical_entries = 10000
| .app_state.staking.params.min_commission_rate = "0.050000000000000000"

# ---- mint (fixed 10% inflation, minted every block) -------------------------
| .app_state.mint.params.mint_denom = $denom
| .app_state.mint.params.inflation_min = $inflation
| .app_state.mint.params.inflation_max = $inflation
| .app_state.mint.params.inflation_rate_change = "0.000000000000000000"
| .app_state.mint.params.goal_bonded = "0.670000000000000000"
| .app_state.mint.params.blocks_per_year = $bpy
| .app_state.mint.minter.inflation = $inflation

# ---- distribution -----------------------------------------------------------
| .app_state.distribution.params.community_tax = "0.020000000000000000"
| .app_state.distribution.params.withdraw_addr_enabled = true

# ---- slashing ---------------------------------------------------------------
| .app_state.slashing.params.signed_blocks_window = "10000"
| .app_state.slashing.params.min_signed_per_window = "0.050000000000000000"
| .app_state.slashing.params.downtime_jail_duration = $downtime
| .app_state.slashing.params.slash_fraction_double_sign = "0.050000000000000000"
| .app_state.slashing.params.slash_fraction_downtime = "0.001000000000000000"

# ---- gov (lightweight) ------------------------------------------------------
| .app_state.gov.params.min_deposit = [{ "denom": $denom, "amount": "1000000000" }]
| .app_state.gov.params.expedited_min_deposit = [{ "denom": $denom, "amount": "5000000000" }]
| .app_state.gov.params.max_deposit_period = $maxdep
| .app_state.gov.params.voting_period = $voting
| .app_state.gov.params.expedited_voting_period = $exvoting
| .app_state.gov.params.quorum = "0.334000000000000000"
| .app_state.gov.params.threshold = "0.500000000000000000"
| .app_state.gov.params.expedited_threshold = "0.667000000000000000"
| .app_state.gov.params.veto_threshold = "0.334000000000000000"
| .app_state.gov.params.min_initial_deposit_ratio = "0.100000000000000000"
| .app_state.gov.params.burn_vote_quorum = false
| .app_state.gov.params.burn_proposal_deposit_prevote = false
| .app_state.gov.params.burn_vote_veto = true

# ---- crisis removed: ensure no dangling state -------------------------------
| del(.app_state.crisis)

# ---- bank: register DUL denom metadata --------------------------------------
| .app_state.bank.denom_metadata = [{
    "description": "The native staking and governance token of the Dulgi network.",
    "denom_units": [
      { "denom": $denom, "exponent": 0, "aliases": ["microdul"] },
      { "denom": "mdul", "exponent": 3, "aliases": ["millidul"] },
      { "denom": "DUL", "exponent": 6, "aliases": [] }
    ],
    "base": $denom,
    "display": "DUL",
    "name": "Dulgi Coin",
    "symbol": "DUL",
    "uri": "",
    "uri_hash": ""
  }]

# ---- consensus block limits (anti-spam / safety) ----------------------------
| .consensus.params.block.max_bytes = "5242880"
| .consensus.params.block.max_gas = "100000000"
' "$GENESIS" > "$tmp" && mv "$tmp" "$GENESIS"

echo "✓ Dulgi genesis parameters applied to $GENESIS"
echo "  staking: max_validators=300 unbonding=21d min_commission=5%"
echo "  mint:    fixed 10% annual inflation, blocks_per_year=$BLOCKS_PER_YEAR"
echo "  slashing: double_sign=5% downtime=0.1%"
echo "  gov:     voting_period=3d quorum=33.4% threshold=50% veto=33.4%"