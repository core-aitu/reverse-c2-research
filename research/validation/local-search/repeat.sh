#!/usr/bin/env bash
set -euo pipefail
sdk=$(realpath "$1"); copy=$(realpath "$2"); build=$(realpath -m "$3")
dir=$(cd "$(dirname "$0")" && pwd)
mkdir -p "$build/home" "$build/app"
export DOTNET_CLI_HOME="$build/home" DOTNET_CLI_TELEMETRY_OPTOUT=1 DOTNET_NOLOGO=1
"$sdk/dotnet" build "$dir/Model.csproj" -p:FixtureDir="$copy/server" --artifacts-path "$build/artifacts" -o "$build/app" >&2
mkdir -p "$build/app/LocalEmbeddingsModel"
touch "$build/app/libonnxruntime.so"
bwrap --unshare-all --die-with-parent --new-session --clearenv --ro-bind /usr /usr \
 --symlink usr/lib /lib --symlink usr/lib64 /lib64 --proc /proc --dev /dev --tmpfs /tmp --dir /server \
 --ro-bind "$copy/server/SpecterInsight.Server.dll" /server/SpecterInsight.Server.dll \
 --ro-bind "$sdk" /dotnet --ro-bind "$build/app" /app \
 --ro-bind "$copy/server/LocalEmbeddingsModel" /app/LocalEmbeddingsModel \
 --ro-bind "$copy/server/runtimes/linux-x64/native/libonnxruntime.so" /app/libonnxruntime.so \
 --setenv HOME /tmp --setenv LD_LIBRARY_PATH /app --setenv DOTNET_EnableDiagnostics 0 --chdir /app \
 /dotnet/dotnet /app/Model.dll
