#!/usr/bin/env bash
set -euo pipefail
sdk=$(realpath "$1")
copy=$(realpath "$2")
build=$(realpath -m "$3")
dir=$(cd "$(dirname "$0")" && pwd)
storage_log=$(realpath "${4:-$dir/../postgres/run-1.jsonl}")
mkdir -p "$build/app" "$build/home"
export DOTNET_CLI_HOME="$build/home" DOTNET_CLI_TELEMETRY_OPTOUT=1 DOTNET_NOLOGO=1
"$sdk/dotnet" build "$dir/Probe.csproj" --artifacts-path "$build/artifacts" -o "$build/app" >&2
files=(--dir /files)
for name in SpecterInsight.UI SpecterInsight.ApiClient Avalonia.Base Avalonia.Controls Avalonia.Controls.DataGrid ReactiveUI Splat System.Reactive DynamicData Newtonsoft.Json Material.Icons; do
  files+=(--ro-bind "$copy/client/$name.dll" "/files/$name.dll")
done
files+=(--ro-bind "$copy/server/SpecterInsight.Server.dll" /files/SpecterInsight.Server.dll)
bwrap --unshare-all --die-with-parent --new-session --clearenv --ro-bind /usr /usr \
  --symlink usr/lib /lib --symlink usr/lib64 /lib64 --proc /proc --dev /dev --tmpfs /tmp \
  --ro-bind "$sdk" /dotnet --ro-bind "$build/app" /app "${files[@]}" \
  --ro-bind "$copy/resources/mac-vendors-export.csv" /vendors.csv --ro-bind "$storage_log" /storage.jsonl --setenv HOME /tmp \
  --setenv DOTNET_EnableDiagnostics 0 --chdir /tmp /dotnet/dotnet /app/Probe.dll
