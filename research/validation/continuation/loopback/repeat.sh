#!/usr/bin/env bash
set -euo pipefail
sdk=$(realpath "$1"); dist=$(realpath "$2"); pg=$(realpath "$3"); build=$(realpath -m "$4")
dir=$(cd "$(dirname "$0")" && pwd)
mkdir -p "$build"/{home,app}
export DOTNET_CLI_HOME="$build/home" DOTNET_NOLOGO=1 DOTNET_CLI_TELEMETRY_OPTOUT=1
"$sdk/dotnet" build "$dir/Loop.csproj" -p:Dist="$dist" --artifacts-path "$build/artifacts" -o "$build/app" >&2
printf 'fixture:x:%s:%s:fixture:/tmp:/bin/bash\n' "$(id -u)" "$(id -g)" > "$build/passwd"
files=()
for path in "$dist"/{client,server}/*.dll; do
 name=$(basename "$path")
 # Only application assemblies used by administrative fixtures, or their vendor dependencies.
 case "$name" in SpecterInsight.*) case "$name" in SpecterInsight.ApiClient.dll|SpecterInsight.Server.dll|SpecterInsight.UI.dll) ;; *) continue;; esac;; esac
 files+=(--ro-bind "$path" "/lib-$(basename "$(dirname "$path")")/$name")
done
files+=(--ro-bind "$dist/server/runtimes/unix/lib/net10.0/System.Management.Automation.dll" /lib-server/System.Management.Automation.dll)
files+=(--ro-bind "$dist/server/runtimes/unix/lib/netstandard1.6/Microsoft.Management.Infrastructure.dll" /lib-server/Microsoft.Management.Infrastructure.dll --ro-bind "$dist/server/runtimes/linux-x64/native/libpsl-native.so" /lib-server/libpsl-native.so)
screen=()
if [[ $# -ge 6 ]]; then
 screen+=(--ro-bind "$(realpath "$5")" /screen --ro-bind "$(realpath "$6")" /Xvfb --ro-bind /etc/fonts /etc/fonts --ro-bind "$dist/client/runtimes/linux-x64/native" /native --setenv DISPLAY :77 --setenv FONTCONFIG_PATH /etc/fonts)
fi
bwrap --unshare-all --die-with-parent --new-session --clearenv --ro-bind /usr /usr --symlink usr/lib /lib --symlink usr/lib64 /lib64 --symlink usr/bin /bin --dir /etc --ro-bind "$build/passwd" /etc/passwd --proc /proc --dev /dev --tmpfs /tmp --dir /lib-client --dir /lib-server "${files[@]}" \
 --setenv HOME /tmp "${screen[@]}" --setenv LD_LIBRARY_PATH /pg/lib:/native:/lib-server --setenv PATH /pg/bin:/usr/bin --setenv LC_ALL C --setenv DOTNET_EnableDiagnostics 0 --ro-bind "$sdk" /dotnet --ro-bind "$pg" /pg --ro-bind "$build/app" /app --chdir /tmp /bin/bash -euc '
 mkdir -p /tmp/study/bin/server
 cp /lib-server/SpecterInsight.Server.dll /tmp/study/bin/server/
 initdb --no-locale --encoding=UTF8 -A trust -U fixture -D /tmp/pgdata >/dev/null
 pg_ctl -D /tmp/pgdata -l /tmp/pg.log -o "-k /tmp -c listen_addresses= -c fsync=off" -w start >/dev/null
 trap "pg_ctl -D /tmp/pgdata -m immediate -w stop >/dev/null" EXIT
 if [[ -x /Xvfb ]]; then
    /Xvfb :77 -screen 0 1280x800x24 -nolisten tcp >/tmp/xlog 2>&1 &
    for ((i=0;i<40;i++)); do xdpyinfo >/dev/null 2>&1 && break; sleep 0.1; done
    xdpyinfo >/dev/null
 fi
 timeout 80 /dotnet/dotnet /app/Loop.dll
 '
