#!/usr/bin/env bash
#
# testnet.sh — launch a local multi-validator Dulgi testnet on a single host.
# Each validator gets its own home dir and a distinct port block.
#
# Env overrides: N (validators, default 4), CHAIN_ID, BIN, BASE_DIR
#
# After running, start each node in a separate terminal using the printed
# commands (or wrap them in your process manager of choice).
set -euo pipefail

N="${N:-4}"
CHAIN_ID="${CHAIN_ID:-dulgi-testnet-1}"
BIN="${BIN:-dulgid}"
BASE_DIR="${BASE_DIR:-$HOME/.dulgi-testnet}"
KEYRING="test"
DENOM="udul"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Per-validator stake / funding.
FUND="10000000000000${DENOM}"   # 10,000,000 DUL
STAKE="1000000000000${DENOM}"   #  1,000,000 DUL

echo ">> Resetting $BASE_DIR (N=$N validators)"
rm -rf "$BASE_DIR"; mkdir -p "$BASE_DIR"

node_home() { echo "$BASE_DIR/node$1"; }

# 1) init every node + create that node's own validator key in its own keyring
for i in $(seq 0 $((N-1))); do
  H="$(node_home "$i")"
  $BIN init "node$i" --chain-id "$CHAIN_ID" --home "$H" --default-denom "$DENOM" >/dev/null 2>&1
  $BIN keys add "val$i" --keyring-backend "$KEYRING" --home "$H" >/dev/null 2>&1
done

# 2) fund all validator accounts in node0's genesis
for i in $(seq 0 $((N-1))); do
  ADDR=$($BIN keys show "val$i" -a --keyring-backend "$KEYRING" --home "$(node_home "$i")")
  $BIN genesis add-genesis-account "$ADDR" "$FUND" --home "$(node_home 0)" >/dev/null 2>&1
done

# 3) apply Dulgi economic params to node0's genesis
bash "$SCRIPT_DIR/configure-genesis.sh" "$(node_home 0)"

# 4) each node creates a gentx against the shared genesis, collected into node0
GENTX_DIR="$(node_home 0)/config/gentx"
mkdir -p "$GENTX_DIR"
for i in $(seq 0 $((N-1))); do
  H="$(node_home "$i")"
  # copy node0's funded+configured genesis to node i so the gentx validates
  if [ "$i" -ne 0 ]; then
    cp "$(node_home 0)/config/genesis.json" "$H/config/genesis.json"
  fi
  $BIN genesis gentx "val$i" "$STAKE" --chain-id "$CHAIN_ID" \
    --keyring-backend "$KEYRING" --home "$H" \
    --commission-rate 0.05 --commission-max-rate 0.20 --commission-max-change-rate 0.01 \
    --output-document "$GENTX_DIR/gentx-node$i.json" >/dev/null 2>&1
done

$BIN genesis collect-gentxs --home "$(node_home 0)" >/dev/null 2>&1
$BIN genesis validate-genesis --home "$(node_home 0)"

# 5) build persistent_peers list and distribute final genesis + config
PEERS=""
for i in $(seq 0 $((N-1))); do
  ID=$($BIN comet show-node-id --home "$(node_home "$i")")
  P2P_PORT=$((26656 + i*100))
  PEERS="${PEERS}${ID}@127.0.0.1:${P2P_PORT},"
done
PEERS="${PEERS%,}"

for i in $(seq 0 $((N-1))); do
  H="$(node_home "$i")"
  if [ "$i" -ne 0 ]; then
    cp "$(node_home 0)/config/genesis.json" "$H/config/genesis.json"
  fi
  RPC=$((26657 + i*100)); P2P=$((26656 + i*100)); GRPC=$((9090 + i*10)); API=$((1317 + i*10))
  C="$H/config/config.toml"; A="$H/config/app.toml"
  sed -i "s#^laddr = \"tcp://127.0.0.1:26657\"#laddr = \"tcp://0.0.0.0:${RPC}\"#" "$C"
  sed -i "s#^laddr = \"tcp://0.0.0.0:26656\"#laddr = \"tcp://0.0.0.0:${P2P}\"#" "$C"
  sed -i "s#^persistent_peers = \"\"#persistent_peers = \"${PEERS}\"#" "$C"
  sed -i "s#^allow_duplicate_ip = false#allow_duplicate_ip = true#" "$C"
  sed -i "s#^address = \"tcp://localhost:9090\"#address = \"localhost:${GRPC}\"#" "$A"
  sed -i "s#^address = \"tcp://localhost:1317\"#address = \"tcp://localhost:${API}\"#" "$A"
done

echo
echo "✓ ${N}-validator testnet prepared under $BASE_DIR"
echo "  Start each node in its own terminal:"
for i in $(seq 0 $((N-1))); do
  echo "    $BIN start --home $(node_home "$i") --minimum-gas-prices 0.025udul"
done