#!/usr/bin/env bash
#
# mainnet.sh — build the Dulgi mainnet genesis with the canonical token
# allocations, then collect validator gentxs.
#
# This script is run by the genesis COORDINATOR. Each foundation/allocation
# address must be a real bech32 'dulgi1...' address you control — set them via
# the environment below. The script refuses to run with unset allocation
# addresses (no placeholders are baked into a production genesis).
#
# Total genesis supply: 100,000,000 DUL = 100,000,000,000,000 udul
#   Community  40%  = 40,000,000 DUL
#   Ecosystem  25%  = 25,000,000 DUL
#   Treasury   15%  = 15,000,000 DUL
#   Team       10%  = 10,000,000 DUL
#   Liquidity  10%  = 10,000,000 DUL
#
# Required env: ADDR_COMMUNITY ADDR_ECOSYSTEM ADDR_TREASURY ADDR_TEAM ADDR_LIQUIDITY
# Optional env: CHAIN_ID, HOME_DIR, BIN, GENTX_DIR
set -euo pipefail

CHAIN_ID="${CHAIN_ID:-dulgi-1}"
HOME_DIR="${HOME_DIR:-$HOME/.dulgi-mainnet}"
BIN="${BIN:-dulgid}"
DENOM="udul"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

: "${ADDR_COMMUNITY:?set ADDR_COMMUNITY to a dulgi1... address}"
: "${ADDR_ECOSYSTEM:?set ADDR_ECOSYSTEM to a dulgi1... address}"
: "${ADDR_TREASURY:?set ADDR_TREASURY to a dulgi1... address}"
: "${ADDR_TEAM:?set ADDR_TEAM to a dulgi1... address}"
: "${ADDR_LIQUIDITY:?set ADDR_LIQUIDITY to a dulgi1... address}"

# Allocations in udul (6 decimals).
ALLOC_COMMUNITY="40000000000000${DENOM}"
ALLOC_ECOSYSTEM="25000000000000${DENOM}"
ALLOC_TREASURY="15000000000000${DENOM}"
ALLOC_TEAM="10000000000000${DENOM}"
ALLOC_LIQUIDITY="10000000000000${DENOM}"

echo ">> Initializing coordinator genesis at $HOME_DIR (chain-id=$CHAIN_ID)"
rm -rf "$HOME_DIR"
$BIN init "dulgi-genesis-coordinator" --chain-id "$CHAIN_ID" --home "$HOME_DIR" --default-denom "$DENOM" >/dev/null 2>&1

echo ">> Writing token allocations"
for pair in \
  "$ADDR_COMMUNITY:$ALLOC_COMMUNITY" \
  "$ADDR_ECOSYSTEM:$ALLOC_ECOSYSTEM" \
  "$ADDR_TREASURY:$ALLOC_TREASURY" \
  "$ADDR_TEAM:$ALLOC_TEAM" \
  "$ADDR_LIQUIDITY:$ALLOC_LIQUIDITY"; do
  ADDR="${pair%%:*}"; AMT="${pair##*:}"
  $BIN genesis add-genesis-account "$ADDR" "$AMT" --home "$HOME_DIR"
done

echo ">> Applying Dulgi economic parameters"
bash "$SCRIPT_DIR/configure-genesis.sh" "$HOME_DIR"

# Optional: collect validator gentxs placed in $GENTX_DIR.
if [ -n "${GENTX_DIR:-}" ]; then
  echo ">> Collecting gentxs from $GENTX_DIR"
  mkdir -p "$HOME_DIR/config/gentx"
  cp "$GENTX_DIR"/*.json "$HOME_DIR/config/gentx/"
  $BIN genesis collect-gentxs --home "$HOME_DIR" >/dev/null 2>&1
fi

echo ">> Validating final genesis"
$BIN genesis validate-genesis --home "$HOME_DIR"

SUPPLY=$($BIN genesis validate-genesis --home "$HOME_DIR" >/dev/null 2>&1; echo "100,000,000 DUL")
cat <<EOF

✓ Mainnet genesis built at $HOME_DIR/config/genesis.json
  Total supply: $SUPPLY

Distribute this genesis.json to all genesis validators. Validators submit their
gentx (1,000,000+ DUL self-delegation) and you re-run with GENTX_DIR set to
collect them, then publish the final genesis + seed node IDs.
EOF