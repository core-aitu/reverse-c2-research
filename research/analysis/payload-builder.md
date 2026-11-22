# Payload pipeline, builder, loader, and cache semantics

## Scope and evidence

This document is the static reconstruction. Its runtime companion used only inert local fixtures and did not execute generated PE/ELF/raw outputs: [pipeline-runtime.md](pipeline-runtime.md).

- Shipped artifact: `server/SpecterInsight.Server.dll`, SHA-256 `a6c454cdfc80e5d9bd3906747d30edbe4fcf69ab2e3c2b52b301b6f4b60e46a5`, MVID `1323c471-66a2-40e3-a048-50e1f9d4dcfb`.
- REA evidence: parent `ev_97af30952479e91512bf71bf76d83d6097b2eeaffdb7e375f3e7ac31ab628686`, summary `ev_846474b02fe466ec3777fb1491aa7e685d98ebaec6a6d8f4950ebee7f72f6695`.
- Selected REA scope: 8 exact types, 80 methods, 34 fields, 638 call edges, 343 field accesses, and 34 loaded user strings; all requested types matched and coverage is complete within that selection.
- Compiler-transformer evidence: Obfuscation parent `ev_30ca4ea802c2c088872145906a74227fa7b7370d6f2c5bdccb2df36bdeccf3f3`, summary `ev_279e88a799388b81cb3005bf1cc5ed0b21a5c5d283f1da3c19c29a355c228762`; 4 exact types, 60 methods, 294 call edges, 85 field accesses, 65 user strings, no unmatched types.
- Pipeline source: `dependencies/specter-pipelines.json`. The reproducible AST inventory is `research/analysis/payload-pipelines.json`; its human-readable index is `research/analysis/payload-pipelines.md`.

“Observed” below means directly present in the shipped manifest, reconstructed C#, or artifact-backed CIL metadata. “Inferred” means a consequence of those observations that has not been runtime-tested.

## Pipeline inventory result

All 117 enabled definitions parse without a PowerShell syntax error. The inventory retains every complete `scriptBlock`, its SHA-256, top-level `param()` contract, manifest defaults, test arguments, output format, command multiset, compiler commands, runtime-variable references, and nested-pipeline references.

| Property | Result |
|---|---:|
| Definitions | 117 |
| Parse-clean | 117 |
| Parse errors | 0 |
| Output formats | 35 |
| Static nested references | 53 |
| Dynamic nested references | 6 |
| Static cycles | 0 |
| Manifest/AST parameter differences | 8 pipelines |
| `Compile-CSharp` users | 14 pipelines |
| `Compile-Zig` users | 11 pipelines |
| `ConvertTo-ZigShellcode` users | 4 pipelines |

Two literal references do not resolve to a manifest name. Runtime-adjacent reconciliation found that the existing `ps_generic_stager` documentation itself calls the absent `ps_generic_cradle` name while its script implements that cradle contract; the consistent explanation is a stale former name/alias. `ps_cmstp_file -> ps_sct_file` is a dangling reference in this supplied image: the manifest has zero matching definitions and one consumer, and no local database/dump, backup, update archive, or reconstructed managed definition supplies the missing row. `generic_sct_file` is a separate parameterized component, not evidence of exact equivalence.

Six references are deliberately dynamic: `cs_runtimeinstaller_load_module` and `msbuild_cs_load_module` use `$PayloadKind`; `generic_vhd_container`, `generic_zip_encrypted` (twice), and `iso_container` use `$SourcePipeline`.

### Parameter differences

These are contract differences, not automatically defects. Parameters with defaults can be intentionally omitted from the manifest. Conversely, when no top-level `param()` block exists, `GetScriptParameters` forwards every remaining manifest/caller argument, so the three manifest-only `Build` values still reach PowerShell binding even though no AST declaration exists.

| Pipeline | Manifest only | AST only | Static consequence |
|---|---|---|---|
| `dotnet_to_js_file` | `Build` | — | Script has no top-level parameter; `GetScriptParameters` forwards all arguments only when no `param()` block exists. |
| `excel_dde_data` | `Build` | — | Same no-`param()` forwarding behavior. |
| `excel_xlm_command` | `Build` | — | Same no-`param()` forwarding behavior. |
| `etw_patch` | — | `Technique` | Optional default is `PatchInMemory`. |
| `generic_zip_encrypted` | — | `SourcePipelineArgs` | Optional hashtable defaults to `@{}` and forwards source-pipeline arguments. |
| `meterpreter_ps_stager` | — | `LPort` | Optional default is `4444`. |
| `meterpreter_ps_stater_obfuscated` | — | `AmsiBypassTechnique`, `Payload`, `RHost`, `RPort`, `StringObfuscationTechnique` | Some are optional defaults, but `RHost` is mandatory for the `Reverse` parameter set and is absent from manifest/test defaults. |
| `meterpreter_shellcode` | — | `Payload`, `RHost`, `RPort` | `RHost` is mandatory for the `Reverse` parameter set and absent from manifest/test defaults. |

## `PayloadPipelineService`

### Definition lifecycle

Observed:

1. Construction attempts Windows AMSI patching and a general patch utility, builds cmdlet documentation, and logs rather than rethrows patch/documentation failures.
2. Startup optionally reads `ServerPaths.SpecterPipelines`, deserializes every definition, and upserts each database row. `PayloadPipelines:ReloadOnStartup` defaults to `true`.
3. A successful upsert broadcasts `PipelineUpdate`. Deletion removes the database record and broadcasts `PipelineDelete` through a resolved `SpecterEventHub`.
4. On non-Windows hosts, a ten-second `chmod +x` attempt prepares the Linux BFlat builder; failure is logged and startup continues.

### Execution contract

Observed:

1. Lookup accepts a lower-cased name or ID from the database.
2. A pooled PowerShell runspace receives `SpecterServicesProvider`, `IsTest`, `ImplantSettings`, `TargetFramework`, and `Build` variables.
3. `GetScriptParameters` parses the script AST. With a top-level `param()` block it converts only declared primitive/enum inputs; a declared `Hashtable` receives all still-unconsumed arguments. Without a top-level block, all remaining key/value pairs are forwarded.
4. `build`, `kind`, and `compress` are removed before binding. The service does not explicitly reject `Parser.ParseInput` diagnostics.
5. Cancellation calls `PowerShell.Stop()`.
6. Normal execution consumes the first result only. Accepted shapes are `string`, `byte[]`, `StringTransformResult`, and `ByteTransformResult`. Test execution additionally serializes `RemotePowerShellResult` as JSON.
7. If the output collection is empty, error-stream text is assembled into an exception. If both result and error collections are empty, the later `First()` call can throw `InvalidOperationException`.

Inferred risks:

- Additional output objects are silently ignored after the first. A pipeline that emits diagnostic output before its payload can therefore return the wrong object or fail type conversion.
- Parse diagnostics are not treated as a separate validation gate, although all 117 bundled definitions are currently parse-clean.

### URL construction

`GetPayloadPipelineURL` mutates the supplied argument dictionary: it inserts `build` when missing and overwrites `kind` with a lower-cased pipeline name. It uses the first comma-separated `CallbackUrlBase`, trims whitespace and slashes, and appends `/static/resources` plus the query string.

## `ImplantBuilderService`

### Build flow

Observed:

1. Convert `CreateImplantRequest` to `SerializedSettings` and add the public half of `DEFAULT_SIGNING_CERTIFICATE` when available.
2. Reject serialized settings above `default serialized length + 500` bytes.
3. Create `<implants>/<build>/{any,x86,x64,powershell}` and write the original request to `configuration.json`.
4. Read `templates/any/specter-dotnet-exe.exe` with Mono.Cecil, replace the public embedded resource `SpecterInsight.Resources.Configuration.json`, and write `any/specter-dotnet-exe.exe`.
5. For four x86/x64 EXE/DLL templates, substitution-encode the core, place its length four bytes before a 32-byte payload marker, place the decoder permutation at a 256-byte marker, and update/encode `BinaryParameters` at the `amsi.dll` marker.
6. Attempt PE version-resource/checksum rewriting with AsmResolver. Failure is logged and the unmodified binary is still written.
7. Convert each emitted DLL to sRDI shellcode using export `Run`, producing one x86 and one x64 `.srdi` file.
8. Copy every PowerShell template by basename into the build directory after replacing `{build}`, `{uribase}`, and `{path}`.

The byte substitution map is generated with `System.Random`, not a cryptographic RNG. Its inverse decoder is embedded in the output template, so this is encoding/obfuscation rather than confidentiality.

### Builder findings

| ID | Confidence | Observation and consequence |
|---|---|---|
| PB-01 | Observed | Oversize diagnostics calculate `MAX_CONFIGURATION_LENGTH - serialized.Length`; for an oversize input the reported reduction is negative. Enforcement still occurs, but the message is wrong. |
| PB-02 | Observed | `request.Build` is passed directly to `Path.Combine(ServerPaths.ImplantsFolder, request.Build)` and no validation attribute or containment check is present in `CreateImplantRequest`, controller, builder, or removal path. A rooted value can escape the base path; traversal segments can also resolve outside it. The endpoint is authorization-protected, but this is an authenticated arbitrary-path write/delete candidate requiring a disposable runtime PoC. |
| PB-03 | Observed | Marker writes rely on `Array.Copy` bounds exceptions; there is no explicit test that the native template has enough reserved capacity for the encoded core. |
| PB-04 | Observed | PowerShell templates are flattened with `Path.GetFileName`; duplicate basenames from different subdirectories overwrite earlier outputs in enumeration order. |
| PB-05 | Observed | `BinaryParameters.TryUpdatePayload` writes only `slim.Length - IntPtr.Size`, deliberately or accidentally leaving the template pointer-sized tail unchanged. Runtime intent is unknown without native-template inspection. |
| PB-06 | Inferred | `new Random()` can repeat substitution maps under correlated construction timing and is not suitable for secrecy; this does not by itself break functional decoding. |

## Payload cache

Observed:

- Default policies cover every `PayloadKind` except `All` and `csharp_config`: 20-minute lifetime and regeneration on expiration.
- Hits return the same cached `byte[]` reference and log retrieval. Misses generate a payload, add it with a timer, and log caching.
- Expiration removes the entry and, when configured, starts background regeneration through `Task.Run`.
- `ClearCache` clears both cached values and policies; later `GetPolicy` recreates missing defaults.

Cache-key findings:

| ID | Confidence | Observation and consequence |
|---|---|---|
| PC-01 | Observed | The hash includes sorted parameter **values**, not names. Different mappings with the same values collide. `Equals` normally separates them, so this is primarily collision amplification rather than direct aliasing. |
| PC-02 | Observed | `ReadOnlyDictionary` wraps the caller's mutable dictionary instead of copying it, while the hash code is frozen at construction. Later caller mutation can violate the dictionary key hash/equality contract and make entries unreachable or inconsistently comparable. |
| PC-03 | Observed | `Equals` checks that every pair on the left exists on the right but does not compare counts. It can be asymmetric for subset parameter sets when hashes collide. |
| PC-04 | Observed | Cache values expose and return the stored mutable `byte[]` directly. A caller can modify the payload seen by later callers while the logged SHA-256 remains the original. |
| PC-05 | Observed | Expiration timers are stopped but not disposed; the expiration callback is synchronous and initiates asynchronous regeneration indirectly. Long-lived churn can retain timer resources longer than necessary. |

## Compiler and loader boundaries

The manifest reaches three compiler commands: C# compilation in 14 pipelines, Zig compilation in 11, and Zig-to-shellcode conversion in 4. `Compile-CSharp -NativeAOT` selects the BFlat transformer and its OS/target/architecture options. The isolated runtime pass successfully produced two Linux x64 BFlat ELF files after supplying `libc++abi.so.1` from a temporary extracted Fedora package and restoring executable bits on shipped `lld`/`llvm-objcopy`; the outputs were not executed and were not byte-identical.

Zig pipelines statically include staged WinHTTP CLR loaders, embedded shellcode loaders, reflective loaders, and freestanding Windows/Linux shellcode generators. Their source-generation semantics are visible, but emitted PE/ELF/raw shellcode correctness is not established until the shipped Zig toolchain runs in an isolated OS-appropriate lab.

### BFlat NativeAOT

Observed argument construction is:

```text
build <source.cs> --os windows|linux --target Shared|WinExe|Exe --arch x86|arm64|x64
[--optimize-space|--optimize-time]
[--no-reflection] [--no-stacktrace-data] [--no-globalization] [--no-debug-info]
--out <temporary.exe>
```

The host selects only the Windows or Linux BFlat executable; the requested target OS is passed as an argument. Source and output are temporary files. A failed process creates a formatted stdout/stderr string but does not log or throw it; the code then tries to read the output and normally reports only a generic empty/read failure. This loses the compiler diagnostics.

### Zig compiler and shellcode extractor

`ZigCompilerTransformer` writes `.zig` source and any `StringTransformResult.EmbeddedFiles`, then runs the host-specific bundled Zig executable. It selects `build-exe` or `build-lib -dynamic`, adds a target triple (`x86|x86_64|aarch64` plus `windows-gnu`, `linux-gnu`, `macos-none`, or `freestanding-none`), optimization, output path, optional Windows subsystem/linker script, `-fPIC` for freestanding, `-lc -lm` for Linux, and points both Zig caches at the shared temporary folder. Process failure includes stdout/stderr in the thrown exception.

The shellcode transformer writes a linker script with `_start` and `.text` at address zero, compiles an executable, and invokes `zig objcopy --output-target binary -j .text`. It rejects empty bytes and outputs beginning with `MZ` or ELF magic. These checks establish wrapper removal only; they do not prove position independence, absence of relocations, or runtime correctness.

`EmbeddedFiles` names are combined directly with the source temporary directory. No rooted/path-traversal or containment check is visible before write/delete. Whether untrusted callers can populate those keys requires tracing every producing transform.

### Loader HTTP services

`LoadersController` exposes five GET actions without the custom `[Authorize]`: runtime ZIP, zstd flat bundle, dependency ZIP, individual assembly, and assembly manifest. The `{loaderId}` route value is never read or validated by any action; it is cosmetic rather than a capability token in this snapshot.

`LoaderDepsService` prefers `dependencies/loader-deps/<platform>.zip`; otherwise it can package top-level PowerShell-specific DLLs only on a Windows server. `LoaderAssemblyService` indexes DLL basenames first from loader-deps and then from the runtime ZIP, so dependency entries win duplicates. ZIP/index exceptions are logged and a partial or empty index is cached. Returned arrays are shared cache references.

`LoaderBundleService` accepts six declared platform strings, prefers a staged ZIP, and otherwise packages the host's `host/fxr` and `shared/Microsoft.NETCore.App`. The non-staged path validates OS family but not requested architecture: for example, an x64 Linux host can package its local runtime under `linux-aarch64`. Flat packaging also ignores its `dotnetRoot` parameter and reads `RuntimeEnvironment.GetRuntimeDirectory()`, stores basenames rather than relative paths, appends `.so` entries from loader deps, and compresses at zstd level 19. These are static compatibility/collision findings; no bundle was fetched or loaded.

| ID | Confidence | Observation and consequence |
|---|---|---|
| PL-01 | Observed | All loader routes omit custom authorization and ignore `loaderId`; network exposure depends only on hosting/reverse proxy. |
| PL-02 | Observed | Non-staged runtime packaging checks OS family but not architecture, so the route label can disagree with packaged host bytes. |
| PL-03 | Observed | Flat local packaging ignores the selected dotnet root and flattens names, allowing same-basename collisions/ambiguity. |
| PL-04 | Observed | Assembly/deps/bundle caches return shared mutable arrays; downstream mutation changes later responses. |
| PL-05 | Observed | BFlat process diagnostics are constructed and discarded on failure. |
| PL-06 | Observed, reachability unknown | Zig embedded-file names have no containment check before temporary-directory write/delete. |

## Reproduction

```text
dotnet build research/examination/pipelines/PipelineAst.csproj --artifacts-path /tmp/specter-pipeline-build
dotnet /tmp/specter-pipeline-build/bin/PipelineAst/debug/PipelineAst.dll dependencies/specter-pipelines.json research/analysis
```

The first command only compiles the local analyzer. The second calls `System.Management.Automation.Language.Parser.ParseInput` and never creates a runspace or invokes a script block.

## Runtime follow-up

The inert runtime pass established C#/BFlat/Zig/PowerShell output hashes, deterministic and randomized cases, Zig raw `.text` extraction, SRDI input mutation, multi-output selection, cancellation, cache-race behavior, timer reset, full expiration/removal/background regeneration through real services and disposable PostgreSQL, marker section bounds, `capacity + 1` overwrite, and the unchanged pointer-sized tail. It also closed `ps_sct_file` for this image as a proven dangling manifest reference rather than synthesizing a replacement. See [pipeline-runtime.md](pipeline-runtime.md).

The authoritative list of unresolved questions is maintained only in [gaps.md](../validation/gaps.md).
