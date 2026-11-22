#!/usr/bin/env bash
set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
run_root="${1:-$(mktemp -d /tmp/specter-pipeline-runtime.XXXXXX)}"
zig_root="$run_root/zig-toolchain"
artifact_root="$run_root/artifacts"
runtime_library_path="$project_dir/bin/Release/net10.0"

if [[ -n "${SPECTER_BFLAT_LIBRARY_PATH:-}" ]]; then
  runtime_library_path="$runtime_library_path:$SPECTER_BFLAT_LIBRARY_PATH"
fi

mkdir -p "$zig_root" "$artifact_root"
cp /home/deleteme/Desktop/SpecterInsight/dependencies/zig/zig "$zig_root/zig"
chmod 0700 "$zig_root/zig"
ln -s /home/deleteme/Desktop/SpecterInsight/dependencies/zig/lib "$zig_root/lib"

dotnet build "$project_dir/PipelineRuntime.csproj" --configuration Release --nologo
mkdir -p "$run_root/xdg-cache" "$run_root/xdg-config" "$run_root/xdg-data"
env \
  XDG_CACHE_HOME="$run_root/xdg-cache" \
  XDG_CONFIG_HOME="$run_root/xdg-config" \
  XDG_DATA_HOME="$run_root/xdg-data" \
  POWERSHELL_TELEMETRY_OPTOUT=1 \
  LD_LIBRARY_PATH="$runtime_library_path" \
  dotnet "$project_dir/bin/Release/net10.0/PipelineRuntime.dll" "$artifact_root" "$zig_root/zig" | tee "$run_root/stdout.json"

sha256sum "$zig_root/zig" /home/deleteme/Desktop/SpecterInsight/dependencies/bflat/bflat > "$run_root/tool-hashes.txt"
file "$artifact_root"/* > "$run_root/file-types.txt"
find "$artifact_root" -maxdepth 1 -type f -print0 | sort -z | xargs -0 sha256sum > "$run_root/artifact-hashes.txt"
uname -a > "$run_root/environment.txt"
dotnet --info >> "$run_root/environment.txt"

printf '%s\n' "$run_root"
