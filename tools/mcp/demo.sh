#!/bin/bash
# Demo do servidor MCP: pipeline completo de
# criação de um domain Widget + service Create usando exclusivamente tool
# calls MCP. Cada chamada sobe sua própria sessão (init → tools/call → EOF),
# replicando a serialização de um cliente MCP real.
#
# Pré-requisitos (rodar dentro do container dev-tools com /app montado):
#   podman compose --profile tools run --rm dev-tools bash tools/mcp/demo.sh

set -e

REPO=/tmp/demo-repo

# 1. Clone do /app pra um sandbox descartável
rm -rf "$REPO"
cp -r /app "$REPO"
rm -rf "$REPO/.git" "$REPO/.scaffold" "$REPO/bin"

# Reset do HCL pra evitar conflito com tabela manual existente
cat > "$REPO/schemas/schema.my.hcl" <<EOF
schema "zord" {}

EOF

go build -o /tmp/mcp-bin ./cmd/mcp

REQ_INIT='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}'
REQ_NOTIF='{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}'

# call_one <id> <tool_name> <arguments_json> — sessão completa em 1 process
call_one() {
  local id=$1 name=$2 args=$3
  local req
  req=$(jq -nc --arg n "$name" --argjson a "$args" --arg id "$id" \
    '{jsonrpc:"2.0", id: ($id | tonumber), method:"tools/call", params:{name:$n, arguments:$a}}')
  { printf '%s\n%s\n%s\n' "$REQ_INIT" "$REQ_NOTIF" "$req"; sleep 0.4; } \
    | /tmp/mcp-bin --repo "$REPO" 2>/dev/null \
    | jq -c "select(.id==$id)"
}

DOMAIN="Widget"
SERVICE="Create"

CALLS=(
  "10|scaffold_domain_create|$(jq -nc --arg d $DOMAIN '{name:$d}')"
  "11|scaffold_field_add|$(jq -nc --arg d $DOMAIN '{domain:$d, field_name:"Name", type:"string", tags:["db:name","json:name","validate:required","db_type:varchar","db_size:120"]}')"
  "12|scaffold_repository_port|$(jq -nc --arg d $DOMAIN '{domain:$d}')"
  "13|scaffold_repository_create|$(jq -nc --arg d $DOMAIN '{domain:$d}')"
  "14|scaffold_repository_register|$(jq -nc --arg d $DOMAIN '{domain:$d}')"
  "15|scaffold_service_create|$(jq -nc --arg d $DOMAIN --arg v $SERVICE '{domain:$d, verb:$v}')"
  "16|scaffold_service_register|$(jq -nc --arg d $DOMAIN --arg v $SERVICE '{domain:$d, verb:$v}')"
  "17|scaffold_handler_create|$(jq -nc --arg d $DOMAIN --arg v $SERVICE '{domain:$d, service:$v}')"
  "18|scaffold_handler_register|$(jq -nc --arg d $DOMAIN --arg v $SERVICE '{domain:$d, service:$v}')"
  "19|scaffold_route_create|$(jq -nc --arg d $DOMAIN '{domain:$d}')"
  "20|scaffold_route_add|$(jq -nc --arg d $DOMAIN --arg v $SERVICE '{domain:$d, service:$v, method:"POST"}')"
  "21|scaffold_route_register|$(jq -nc --arg d $DOMAIN '{domain:$d}')"
  "22|scaffold_derive_schema|$(jq -nc --arg d $DOMAIN '{domain:$d}')"
)

echo
echo "================================================================="
echo " PIPELINE DEMO: cria domain '$DOMAIN' + service '$SERVICE'"
echo "================================================================="
printf "%-3s %-32s %-8s %s\n" "id" "tool" "result" "files"
echo "-----------------------------------------------------------------"
for c in "${CALLS[@]}"; do
  IFS='|' read -r id name args <<< "$c"
  resp=$(call_one "$id" "$name" "$args")
  is_err=$(echo "$resp" | jq -r '.result.isError // false')
  if [ "$is_err" = "true" ]; then
    msg=$(echo "$resp" | jq -r '.result.content[0].text' | head -1)
    printf "%-3s %-32s %-8s %s\n" "$id" "$name" "ERROR" "$msg"
  else
    files=$(echo "$resp" | jq -rc '.result.structuredContent | (.created // []) + (.modified // []) | join(",")')
    printf "%-3s %-32s %-8s %s\n" "$id" "$name" "OK" "$files"
  fi
done
echo

echo "================================================================="
echo " BUILD pós-demo"
echo "================================================================="
(cd "$REPO" && go build ./... 2>&1 | head -20) && echo "BUILD_OK"

echo "================================================================="
echo " HCL gerado pra $DOMAIN"
echo "================================================================="
grep -A20 'scaffold:generated' "$REPO/schemas/schema.my.hcl" | head -25

# Pipeline reverso: desfaz tudo na ordem inversa de dependências
# (route_remove → route_unregister/route_delete → handler_unregister/
# handler_delete → service_unregister/service_delete → derive_schema_remove
# → repository_unregister/repository_delete → repository_unport →
# domain_delete). Cada step verifica que o sandbox volta ao estado limpo.
REVERSE_CALLS=(
  "30|scaffold_route_remove|$(jq -nc --arg d $DOMAIN --arg v $SERVICE '{domain:$d, service:$v}')"
  "31|scaffold_route_unregister|$(jq -nc --arg d $DOMAIN '{domain:$d}')"
  "32|scaffold_route_delete|$(jq -nc --arg d $DOMAIN '{domain:$d}')"
  "33|scaffold_handler_unregister|$(jq -nc --arg d $DOMAIN --arg v $SERVICE '{domain:$d, service:$v}')"
  "34|scaffold_handler_delete|$(jq -nc --arg d $DOMAIN --arg v $SERVICE '{domain:$d, service:$v}')"
  "35|scaffold_service_unregister|$(jq -nc --arg d $DOMAIN --arg v $SERVICE '{domain:$d, verb:$v}')"
  "36|scaffold_service_delete|$(jq -nc --arg d $DOMAIN --arg v $SERVICE '{domain:$d, verb:$v}')"
  "37|scaffold_derive_schema_remove|$(jq -nc --arg d $DOMAIN '{domain:$d}')"
  "38|scaffold_repository_unregister|$(jq -nc --arg d $DOMAIN '{domain:$d}')"
  "39|scaffold_repository_delete|$(jq -nc --arg d $DOMAIN '{domain:$d}')"
  "40|scaffold_repository_unport|$(jq -nc --arg d $DOMAIN '{domain:$d}')"
  "41|scaffold_domain_delete|$(jq -nc --arg d $DOMAIN '{domain:$d}')"
)

echo
echo "================================================================="
echo " PIPELINE REVERSE: desfaz domain '$DOMAIN' + service '$SERVICE'"
echo "================================================================="
printf "%-3s %-32s %-8s %s\n" "id" "tool" "result" "files"
echo "-----------------------------------------------------------------"
for c in "${REVERSE_CALLS[@]}"; do
  IFS='|' read -r id name args <<< "$c"
  resp=$(call_one "$id" "$name" "$args")
  is_err=$(echo "$resp" | jq -r '.result.isError // false')
  if [ "$is_err" = "true" ]; then
    msg=$(echo "$resp" | jq -r '.result.content[0].text' | head -1)
    printf "%-3s %-32s %-8s %s\n" "$id" "$name" "ERROR" "$msg"
  else
    files=$(echo "$resp" | jq -rc '.result.structuredContent | (.created // []) + (.modified // []) + (.deleted // []) | join(",")')
    printf "%-3s %-32s %-8s %s\n" "$id" "$name" "OK" "$files"
  fi
done
echo

echo "================================================================="
echo " BUILD pós-reverse"
echo "================================================================="
(cd "$REPO" && go build ./... 2>&1 | head -20) && echo "BUILD_OK"
