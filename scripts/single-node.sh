#!/usr/bin/env bash
#
# single-node.sh — spin up a local single-validator Dulgi devnet.
#
# Env overrides: CHAIN_ID, MONIKER, HOME_DIR, BIN, KEYRING
set -euo pipefail

CHAIN_ID="${CHAIN_ID:-dulgi-local-1}"
MONIKER="${MONIKER:-dulgi-local}"
HOME_DIR="${HOME_DIR:-$HOME/.dulgi}"
BIN="${BIN:-dulgid}"
KEYRING="${KEYRING:-test}"
DENOM="udul"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo ">> Resetting $HOME_DIR"
rm -rf "$HOME_DIR"

echo ">> dulgid init"
$BIN init "$MONIKER" --chain-id "$CHAIN_ID" --home "$HOME_DIR" --default-denom "$DENOM" >/dev/null 2>&1

echo ">> Creating validator key"
$BIN keys add validator --keyring-backend "$KEYRING" --home "$HOME_DIR"

echo ">> Funding validator (10,000,000 DUL)"
$BIN genesis add-genesis-account validator 10000000000000"$DENOM" \
  --keyring-backend "$KEYRING" --home "$HOME_DIR"

echo ">> Applying Dulgi genesis parameters"
bash "$SCRIPT_DIR/configure-genesis.sh" "$HOME_DIR"

echo ">> Creating gentx (1,000,000 DUL self-delegation)"
$BIN genesis gentx validator 1000000000000"$DENOM" \
  --chain-id "$CHAIN_ID" --keyring-backend "$KEYRING" --home "$HOME_DIR" \
  --commission-rate 0.05 --commission-max-rate 0.20 --commission-max-change-rate 0.01 >/dev/null 2>&1

echo ">> Collecting gentxs"
$BIN genesis collect-gentxs --home "$HOME_DIR" >/dev/null 2>&1

echo ">> Validating genesis"
$BIN genesis validate-genesis --home "$HOME_DIR"

cat <<EOF

✓ Devnet ready. Start the node with:

  $BIN start --home $HOME_DIR --minimum-gas-prices 0.025udul

RPC:  http://localhost:26657
API:  http://localhost:1317
gRPC: localhost:9090
EOF