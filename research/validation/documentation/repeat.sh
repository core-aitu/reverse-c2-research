#!/usr/bin/env bash
set -euo pipefail
# Arguments: isolated SDK directory, working copy of the distribution, build directory.
sdk=$(realpath "$1")
copy=$(realpath "$2")
build=$(realpath -m "$3")
here=$(cd "$(dirname "$0")" && pwd)
mkdir -p "$build/app" "$build/home"
export DOTNET_CLI_HOME="$build/home" DOTNET_CLI_TELEMETRY_OPTOUT=1 DOTNET_NOLOGO=1 DOTNET_SKIP_FIRST_TIME_EXPERIENCE=1
"$sdk/dotnet" build "$here/Probe.csproj" --artifacts-path "$build/artifacts" -o "$build/app" >&2
client_files=()
for name in SpecterInsight.UI ReactiveUI System.Reactive Splat DynamicData Avalonia.Base ReactiveUI.Fody.Helpers; do
  client_files+=(--ro-bind "$copy/client/$name.dll" "/client/$name.dll")
done
bwrap --unshare-all --die-with-parent --new-session --clearenv \
  --ro-bind /usr /usr --symlink usr/lib /lib --symlink usr/lib64 /lib64 --proc /proc --dev /dev \
  --tmpfs /tmp --ro-bind "$sdk" /dotnet --ro-bind "$build/app" /app \
  --dir /server --dir /client --ro-bind "$copy/server/SpecterInsight.Server.dll" /server/SpecterInsight.Server.dll \
  --ro-bind "$copy/client/SpecterInsight.ApiClient.dll" /client/SpecterInsight.ApiClient.dll \
  --ro-bind "$copy/client/Newtonsoft.Json.dll" /client/Newtonsoft.Json.dll \
  "${client_files[@]}" \
  --setenv DOTNET_NOLOGO 1 --setenv HOME /tmp --setenv DOTNET_EnableDiagnostics 0 \
  --chdir /tmp /dotnet/dotnet /app/Probe.dll
