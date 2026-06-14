#!/usr/bin/env bash
#
# validator-setup.sh — prepare a node to join an EXISTING Dulgi chain and
# (optionally) promote it to a validator.
#
# Required env:
#   CHAIN_ID      e.g. dulgi-1
#   GENESIS_URL   URL to the network's genesis.json
#   SEEDS         comma-separated CometBFT seed list (id@host:port)
# Optional env:
#   PERSISTENT_PEERS, MONIKER, HOME_DIR, BIN, MIN_GAS_PRICE,
#   STATE_SYNC_RPC1, STATE_SYNC_RPC2  (enable state-sync if both set)
set -euo pipefail

: "${CHAIN_ID:?set CHAIN_ID}"
: "${GENESIS_URL:?set GENESIS_URL}"
: "${SEEDS:?set SEEDS}"
MONIKER="${MONIKER:-$(hostname)}"
HOME_DIR="${HOME_DIR:-$HOME/.dulgi}"
BIN="${BIN:-dulgid}"
MIN_GAS_PRICE="${MIN_GAS_PRICE:-0.025udul}"
PERSISTENT_PEERS="${PERSISTENT_PEERS:-}"

echo ">> Initializing node home at $HOME_DIR"
$BIN init "$MONIKER" --chain-id "$CHAIN_ID" --home "$HOME_DIR" --default-denom udul >/dev/null 2>&1 || true

echo ">> Fetching genesis"
curl -fsSL "$GENESIS_URL" -o "$HOME_DIR/config/genesis.json"
$BIN genesis validate-genesis --home "$HOME_DIR"

CONF="$HOME_DIR/config/config.toml"
APP="$HOME_DIR/config/app.toml"

echo ">> Configuring peers + gas price"
sed -i "s#^seeds = \".*\"#seeds = \"$SEEDS\"#" "$CONF"
sed -i "s#^persistent_peers = \".*\"#persistent_peers = \"$PERSISTENT_PEERS\"#" "$CONF"
sed -i "s#^minimum-gas-prices = \".*\"#minimum-gas-prices = \"$MIN_GAS_PRICE\"#" "$APP"

# Optional state-sync for fast bootstrap.
if [ -n "${STATE_SYNC_RPC1:-}" ] && [ -n "${STATE_SYNC_RPC2:-}" ]; then
  echo ">> Enabling state-sync"
  LATEST=$(curl -fsSL "$STATE_SYNC_RPC1/block" | sed -n 's/.*"height":"\([0-9]*\)".*/\1/p' | head -1)
  TRUST_HEIGHT=$((LATEST - 2000)); [ "$TRUST_HEIGHT" -lt 1 ] && TRUST_HEIGHT=1
  TRUST_HASH=$(curl -fsSL "$STATE_SYNC_RPC1/block?height=$TRUST_HEIGHT" | sed -n 's/.*"hash":"\([0-9A-Fa-f]*\)".*/\1/p' | head -1)
  sed -i "s#^enable = false#enable = true#" "$CONF"
  sed -i "s#^rpc_servers = \".*\"#rpc_servers = \"$STATE_SYNC_RPC1,$STATE_SYNC_RPC2\"#" "$CONF"
  sed -i "s#^trust_height = .*#trust_height = $TRUST_HEIGHT#" "$CONF"
  sed -i "s#^trust_hash = \".*\"#trust_hash = \"$TRUST_HASH\"#" "$CONF"
fi

cat <<EOF

✓ Node configured to join $CHAIN_ID.

Next:
  1. Start and sync the node:
       $BIN start --home $HOME_DIR --minimum-gas-prices $MIN_GAS_PRICE
  2. Create/import the validator operator key:
       $BIN keys add validator --home $HOME_DIR
       (fund it with DUL, then ensure the node is fully synced)
  3. Promote to validator once synced (catching_up == false):
       cat > validator.json <<JSON
{
  "pubkey": $($BIN comet show-validator --home $HOME_DIR),
  "amount": "1000000000000udul",
  "moniker": "$MONIKER",
  "commission-rate": "0.05",
  "commission-max-rate": "0.20",
  "commission-max-change-rate": "0.01",
  "min-self-delegation": "1"
}
JSON
       $BIN tx staking create-validator validator.json \\
         --from validator --chain-id $CHAIN_ID --home $HOME_DIR \\
         --gas auto --gas-adjustment 1.4 --fees 5000udul
EOF