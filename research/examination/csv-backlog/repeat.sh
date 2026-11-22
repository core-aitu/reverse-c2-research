#!/usr/bin/env bash
set -euo pipefail
sdk=$(realpath "$1")
product=$(realpath "$2")
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
source_dir=$(cd "$(dirname "$0")" && pwd)
export DOTNET_CLI_HOME="$work/home" DOTNET_CLI_TELEMETRY_OPTOUT=1 DOTNET_NOLOGO=1
"$sdk/dotnet" build "$source_dir/ReadVendors.csproj" --artifacts-path "$work/build" -o "$work/app" >&2
bwrap --unshare-all --die-with-parent --new-session --clearenv --ro-bind /usr /usr \
  --symlink usr/lib /lib --symlink usr/lib64 /lib64 --proc /proc --dev /dev --tmpfs /tmp \
  --ro-bind "$sdk" /dotnet --ro-bind "$work/app" /app --dir /app-data \
  --ro-bind "$product/server/SpecterInsight.Server.dll" /app-data/SpecterInsight.Server.dll \
  --ro-bind "$product/resources/mac-vendors-export.csv" /app-data/vendors.csv \
  --setenv HOME /tmp --setenv DOTNET_EnableDiagnostics 0 --chdir /tmp /dotnet/dotnet /app/ReadVendors.dll
