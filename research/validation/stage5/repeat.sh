#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 5 ]]; then
  echo "usage: $0 SDK DIST PG16 XVFB NEW_OUTPUT_DIR" >&2
  exit 2
fi
sdk=$(realpath "$1")
dist=$(realpath "$2")
pg=$(realpath "$3")
xvfb=$(realpath "$4")
out=$(realpath -m "$5")
dir=$(cd "$(dirname "$0")" && pwd)
if [[ -e "$out" ]] && [[ -n "$(find "$out" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]]; then
  echo "output directory must be new or empty: $out" >&2
  exit 2
fi
mkdir -p "$out"/{home,catalog-app,catalog-socket,workflow-app,workflow-output}
export DOTNET_CLI_HOME="$out/home" DOTNET_NOLOGO=1 DOTNET_CLI_TELEMETRY_OPTOUT=1

"$sdk/dotnet" build "$dir/Catalog.csproj" -p:Dist="$dist" \
  --artifacts-path "$out/catalog-artifacts" -o "$out/catalog-app" >&2
export LD_LIBRARY_PATH="$pg/lib"
"$pg/bin/initdb" --no-locale --encoding=UTF8 --auth-local=trust --username=fixture \
  -D "$out/pgdata" >/dev/null
"$pg/bin/pg_ctl" -D "$out/pgdata" -l "$out/postgres.log" \
  -o "-k $out/catalog-socket -c listen_addresses= -c fsync=off" -w start >/dev/null
trap '"$pg/bin/pg_ctl" -D "$out/pgdata" -m immediate -w stop >/dev/null 2>&1 || true' EXIT
STAGE5_DIST="$dist" STAGE5_DATABASE="Host=$out/catalog-socket;Username=fixture;Database=postgres;Pooling=false" \
  "$sdk/dotnet" "$out/catalog-app/Catalog.dll" > "$out/postgres-catalog.json"
"$pg/bin/pg_ctl" -D "$out/pgdata" -m immediate -w stop >/dev/null
trap - EXIT

"$sdk/dotnet" build "$dir/WorkflowSmoke.csproj" -p:Dist="$dist" \
  --artifacts-path "$out/workflow-artifacts" -o "$out/workflow-app" >&2
python - "$dist/client/SpecterInsight.UI.runtimeconfig.json" "$out/workflow-app/WorkflowSmoke.runtimeconfig.json" <<'PY'
import json,sys
from pathlib import Path
source,target=map(Path,sys.argv[1:])
options=json.loads(source.read_text(encoding='utf-8-sig'))['runtimeOptions'].get('configProperties',{})
runtime=json.loads(target.read_text())
runtime['runtimeOptions'].setdefault('configProperties',{}).update(options)
target.write_text(json.dumps(runtime))
PY
bwrap --unshare-all --die-with-parent --new-session --clearenv \
  --ro-bind /usr /usr --symlink usr/lib /lib --symlink usr/lib64 /lib64 --symlink usr/bin /bin \
  --ro-bind /etc/fonts /etc/fonts --dev /dev --proc /proc --tmpfs /tmp \
  --bind "$out/workflow-output" /out --setenv HOME /tmp --setenv PATH /usr/bin \
  --setenv DISPLAY :88 --setenv FONTCONFIG_PATH /etc/fonts \
  --setenv LD_LIBRARY_PATH /files/runtimes/linux-x64/native --setenv DOTNET_EnableDiagnostics 0 \
  --setenv STAGE5_UI_OUTPUT /out/workflows --ro-bind "$sdk" /dotnet --ro-bind "$xvfb" /Xvfb \
  --ro-bind "$out/workflow-app" /app --ro-bind "$dist/client" /files --chdir /tmp /bin/bash -euc '
    /Xvfb :88 -screen 0 1280x800x24 -nolisten tcp >/tmp/xlog 2>&1 & xp=$!
    trap "kill $xp 2>/dev/null || true" EXIT
    for ((i=0;i<40;i++)); do xdpyinfo >/dev/null 2>&1 && break; sleep 0.1; done
    timeout 90 /dotnet/dotnet /app/WorkflowSmoke.dll > /out/workflow-smoke.json
  '

"$dir/../continuation/desktop/repeat.sh" "$sdk" "$dist" "$xvfb" "$out/screen" \
  > "$out/desktop.jsonl"
"$dir/../continuation/loopback/repeat.sh" "$sdk" "$dist" "$pg" "$out/chain" \
  "$out/screen/app" "$xvfb" > "$out/correlated-chain.jsonl"

python - "$out/postgres-catalog.json" "$out/workflow-output/workflow-smoke.json" "$out/correlated-chain.jsonl" <<'PY'
import json,sys
catalog=json.load(open(sys.argv[1]))
workflow=json.load(open(sys.argv[2]))
chain=[json.loads(line) for line in open(sys.argv[3])]
assert all(x['applied'] for x in catalog['create_methods'])
assert len(catalog['catalog']['tables']) == 36
assert len(catalog['catalog']['enums']) == 10
assert sum(len(t['indexes']) for t in catalog['catalog']['tables']) == 81
assert workflow['expected'] == workflow['passed'] == 24 and workflow['complete']
passed={x['id']:x.get('passed') for x in chain if 'id' in x}
for key in ('stock-window-visible','stock-window-exit','same-id-in-database'):
    assert passed[key] is True
print('stage5 runtime checks passed')
PY
