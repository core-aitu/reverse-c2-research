#!/usr/bin/env bash
# Build the alternate desktop observation from the existing screen and run the existing isolated loopback experiment.
set -euo pipefail
sdk=$(realpath "$1")
dist=$(realpath "$2")
pg=$(realpath "$3")
xvfb=$(realpath "$4")
build=$(realpath -m "$5")
dir=$(cd "$(dirname "$0")" && pwd)
stage=$(realpath "$dir/../../validation/continuation")
mkdir -p "$build"/{src,screen,home}
cp "$stage/desktop/"{Screen.csproj,DocumentationOrder.cs} "$dir/Observe.cs" "$build/src/"
python3 - "$stage/desktop/Program.cs" "$build/src/Program.cs" <<'PY'
import sys
from pathlib import Path
source, dest = map(Path, sys.argv[1:])
text = source.read_text()
start = text.index('            var control=(Control)', text.index('if(live!=null)'))
end = text.index('        }\n        foreach(var mode', start)
dest.write_text(text[:start] + '            await Observe.Run(ui,client,live);\n            return;\n' + text[end:])
PY
export DOTNET_CLI_HOME="$build/home" DOTNET_NOLOGO=1 DOTNET_CLI_TELEMETRY_OPTOUT=1
"$sdk/dotnet" build "$build/src/Screen.csproj" -p:Dist="$dist" --artifacts-path "$build/artifacts" -o "$build/screen" >&2
python3 - "$dist/client/SpecterInsight.UI.runtimeconfig.json" "$build/screen/Screen.runtimeconfig.json" <<'PY'
from pathlib import Path
import sys,json
source,target = map(Path,sys.argv[1:])
runtime=json.loads(target.read_text())
runtime['runtimeOptions'].setdefault('configProperties',{}).update(json.loads(source.read_text(encoding='utf-8-sig'))['runtimeOptions']['configProperties'])
target.write_text(json.dumps(runtime))
PY
"$stage/loopback/repeat.sh" "$sdk" "$dist" "$pg" "$build/loop" "$build/screen" "$xvfb"
