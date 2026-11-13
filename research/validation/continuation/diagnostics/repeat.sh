#!/usr/bin/env bash
set -euo pipefail
sdk=$(realpath "$1"); copy=$(realpath "$2"); build=$(realpath -m "$3")
dir=$(cd "$(dirname "$0")" && pwd)
mkdir -p "$build"/{home,app}
export DOTNET_CLI_HOME="$build/home" DOTNET_NOLOGO=1 DOTNET_CLI_TELEMETRY_OPTOUT=1
"$sdk/dotnet" build "$dir/Probe.csproj" --artifacts-path "$build/artifacts" -o "$build/app" >&2
while IFS=$'\t' read -r hash path; do
 extra=()
 if [[ "$path" == server/SpecterInsight.exe ]]; then extra+=(--ro-bind "$copy/server/SpecterInsight.pdb" /symbols.pdb --ro-bind "$copy/templates/source/SpecterInsight/Logging/LogEntry.cs" /LogEntry.cs); fi
 bwrap --unshare-all --die-with-parent --new-session --clearenv --ro-bind /usr /usr \
  --symlink usr/lib /lib --symlink usr/lib64 /lib64 --dev /dev --proc /proc --tmpfs /tmp --chdir /tmp \
  --ro-bind "$sdk" /dotnet --ro-bind "$build/app" /app --ro-bind "$copy/$path" /sample.exe \
  "${extra[@]}" --setenv HOME /tmp --setenv DOTNET_EnableDiagnostics 0 /dotnet/dotnet /app/Probe.dll "$hash"
done < <(python -c 'import json,sys;print("\n".join(x["sha256"]+"\t"+x["paths"][0] for x in json.load(open(sys.argv[1]))))' "$dir/../diagnostic-variants.json")
