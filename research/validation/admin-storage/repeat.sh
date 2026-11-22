#!/usr/bin/env bash
# Run the existing storage fixture in a fresh cluster, then inspect both tables.
set -euo pipefail
sdk=$(realpath "$1")
dist=$(realpath "$2")
pg=$(realpath "$3")
build=$(realpath -m "$4")
dir=$(cd "$(dirname "$0")" && pwd)
mkdir -p "$build/home" "$build/app"
export DOTNET_CLI_HOME="$build/home" DOTNET_CLI_TELEMETRY_OPTOUT=1 DOTNET_NOLOGO=1
"$sdk/dotnet" build "$dir/../postgres/Store.csproj" -p:FixtureDir="$dist/server" \
  --artifacts-path "$build/artifacts" -o "$build/app" >&2
printf 'fixture:x:%s:%s:fixture:/tmp:/bin/bash\n' "$(id -u)" "$(id -g)" > "$build/passwd"
bwrap --unshare-all --die-with-parent --new-session --clearenv --ro-bind /usr /usr \
  --symlink usr/lib /lib --symlink usr/lib64 /lib64 --symlink usr/bin /bin --dir /etc \
  --ro-bind "$build/passwd" /etc/passwd --proc /proc --dev /dev --tmpfs /tmp \
  --setenv HOME /tmp --setenv PATH /pg/bin:/usr/bin --setenv LC_ALL C \
  --setenv DOTNET_EnableDiagnostics 0 --setenv LD_LIBRARY_PATH /pg/lib \
  --ro-bind "$sdk" /dotnet --ro-bind "$pg" /pg --ro-bind "$build/app" /app \
  --ro-bind "$dir/schema.sql" /schema.sql --dir /files \
  --ro-bind "$dist/server/SpecterInsight.Server.dll" /files/SpecterInsight.Server.dll \
  --ro-bind "$dist/server/Newtonsoft.Json.dll" /files/Newtonsoft.Json.dll \
  --chdir /tmp /bin/bash -euc '
    initdb --no-locale --encoding=UTF8 --auth-local=trust --username=fixture -D /tmp/data >/dev/null
    pg_ctl -D /tmp/data -l /tmp/pg.log -o "-k /tmp -c listen_addresses= -c fsync=off" -w start >/dev/null
    trap "pg_ctl -D /tmp/data -m immediate -w stop >/dev/null" EXIT
    /dotnet/dotnet /app/Store.dll >&2
    psql -Xq --set ON_ERROR_STOP=1 -At -h /tmp -U fixture -d postgres -f /schema.sql
  '
