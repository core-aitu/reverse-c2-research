#!/usr/bin/env bash
set -euo pipefail
sdk=$(realpath "$1"); dist=$(realpath "$2"); xvfb=$(realpath "$3"); build=$(realpath -m "$4")
dir=$(cd "$(dirname "$0")" && pwd)
mkdir -p "$build"/{app,home}
export DOTNET_CLI_HOME="$build/home" DOTNET_NOLOGO=1 DOTNET_CLI_TELEMETRY_OPTOUT=1
"$sdk/dotnet" build "$dir/Screen.csproj" -p:Dist="$dist" --artifacts-path "$build/artifacts" -o "$build/app" >&2
python - "$dist/client/SpecterInsight.UI.runtimeconfig.json" "$build/app/Screen.runtimeconfig.json" <<'PYCONF'
import json,sys
from pathlib import Path
source,target=map(Path,sys.argv[1:])
options=json.loads(source.read_text(encoding='utf-8-sig'))['runtimeOptions'].get('configProperties',{})
runtime=json.loads(target.read_text())
runtime['runtimeOptions'].setdefault('configProperties',{}).update(options)
target.write_text(json.dumps(runtime))
PYCONF
python -c 'import sys,json; print(json.dumps(next(x["record"] for x in map(json.loads,open(sys.argv[1])) if x["id"]=="signalr-record")))' "$dir/../loopback/loop-1.jsonl" > "$build/record.json"
files=(--dir /files)
for path in "$dist/client/"*.dll; do
 name=$(basename "$path"); case "$name" in SpecterInsight.Obfuscation.dll) continue;; esac
 files+=(--ro-bind "$path" "/files/$name")
done
bwrap --unshare-all --die-with-parent --new-session --clearenv --ro-bind /usr /usr --symlink usr/lib /lib --symlink usr/lib64 /lib64 --symlink usr/bin /bin --ro-bind /etc/fonts /etc/fonts --dev /dev --proc /proc --tmpfs /tmp \
 --setenv STUDY_SCREEN_CASE "${5:-baseline}" --setenv HOME /tmp --setenv PATH /usr/bin --setenv DISPLAY :77 --setenv FONTCONFIG_PATH /etc/fonts --setenv LD_LIBRARY_PATH /native --setenv DOTNET_EnableDiagnostics 0 --ro-bind "$sdk" /dotnet --ro-bind "$build/app" /app --ro-bind "$xvfb" /Xvfb --ro-bind "$build/record.json" /record.json --ro-bind "$dist/client/runtimes/linux-x64/native" /native "${files[@]}" --chdir /tmp /bin/bash -euc '
 /Xvfb :77 -screen 0 1280x800x24 -nolisten tcp >/tmp/xlog 2>&1 &
 xp=$!; trap "kill $xp 2>/dev/null || true" EXIT
 for ((i=0;i<40;i++)); do xdpyinfo >/dev/null 2>&1 && break; sleep 0.1; done
 xdpyinfo >/dev/null || { cat /tmp/xlog >&2; exit 1; }
 timeout 40 /dotnet/dotnet /app/Screen.dll
 '
