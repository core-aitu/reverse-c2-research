using System.Collections.Concurrent;
using System.Diagnostics;
using System.Linq.Expressions;
using System.Management.Automation;
using System.Reflection;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using SpecterInsight.Obfuscation.Classes;
using SpecterInsight.Obfuscation.CSharp;
using SpecterInsight.Obfuscation.CSharp.OutputTransforms;
using SpecterInsight.Obfuscation.PowerShell.AstTransforms;
using SpecterInsight.Obfuscation.PowerShell.TokenTransforms;
using SpecterInsight.Server.Classes;
using SpecterInsight.Server.Services;

internal sealed record ArtifactRecord(
    string Pipeline,
    string Run,
    string Input,
    string Output,
    string Sha256,
    long Size,
    string Format,
    string Status,
    string Limitations);

internal sealed class RuntimeReport
{
    public string StartedUtc { get; set; } = DateTime.UtcNow.ToString("O");
    public string Host { get; set; } = $"{Environment.OSVersion}; {RuntimeInformation()}";
    public List<ArtifactRecord> Artifacts { get; } = [];
    public Dictionary<string, object?> Checks { get; } = [];
    private static string RuntimeInformation() => $".NET {Environment.Version}; {System.Runtime.InteropServices.RuntimeInformation.ProcessArchitecture}; pointer={IntPtr.Size}";
}

internal static class Program
{
    private const string Root = "/home/deleteme/Desktop/SpecterInsight";
    private const string Server = Root + "/server";
    private const string Dependencies = Root + "/dependencies";
    private static readonly byte[] BinaryMarker =
    [
        89, 187, 236, 92, 156, 233, 210, 155, 51, 99, 40, 174, 160, 69, 90, 5,
        205, 154, 225, 223, 244, 120, 223, 206, 207, 23, 231, 146, 196, 135, 206, 232
    ];
    private static int _timerCallbacks;
    private static readonly ManualResetEventSlim TimerSignal = new(false);

    public static async Task<int> Main(string[] args)
    {
        if (args.Length != 2)
        {
            Console.Error.WriteLine("usage: PipelineRuntime <output-directory> <executable-zig-path>");
            return 2;
        }

        string output = Path.GetFullPath(args[0]);
        string zig = Path.GetFullPath(args[1]);
        Directory.CreateDirectory(output);
        RuntimeReport report = new();

        await Attempt(report, "C# compiler", () => RunCSharp(report, output));
        await Attempt(report, "BFlat / NativeAOT", () => RunBFlat(report, output));
        await Attempt(report, "Zig", () => RunZig(report, output, zig));
        await Attempt(report, "Zig raw-section extractor", () => RunZigRaw(report, output, zig));
        await Attempt(report, "PowerShell transforms", () => RunPowerShellTransforms(report, output));
        await Attempt(report, "PowerShell contracts", () => RunPowerShellContracts(report));
        await Attempt(report, "SRDI", () => RunSrdi(report, output));
        await Attempt(report, "PE template injection", () => RunTemplateInjection(report, output));
        await Attempt(report, "cache contracts", () => RunCacheContracts(report));

        report.Checks["finishedUtc"] = DateTime.UtcNow.ToString("O");
        string json = JsonSerializer.Serialize(report, new JsonSerializerOptions { WriteIndented = true });
        await File.WriteAllTextAsync(Path.Combine(output, "runtime-report.json"), json + Environment.NewLine);
        Console.WriteLine(json);
        return 0;
    }

    private static async Task Attempt(RuntimeReport report, string name, Func<Task> action)
    {
        try
        {
            await action();
            report.Checks[$"stage-{name}"] = new { status = "completed" };
        }
        catch (Exception ex)
        {
            report.Checks[$"stage-{name}"] = new
            {
                status = "failed",
                exception = ex.GetType().FullName,
                message = ex.Message,
                inner = ex.InnerException?.Message,
                detail = ex.ToString()
            };
        }
    }

    private static async Task RunCSharp(RuntimeReport report, string output)
    {
        References.LoadReferences(Path.Combine(Dependencies, ".NET 2.0"), Path.Combine(Dependencies, ".NET 4.0"));
        const string source = "public static class Fixture { public static int Value() => 42; }";
        CSharpAssemblyInfo info = new("fixture.dll", "Fixture", "Inert deterministic fixture", "Local Lab", "Local Lab", "1.2.3.4", "1.2.3.4");
        for (int i = 1; i <= 2; i++)
        {
            CSharpCompilerTransformer transformer = new(info, CSharpCompilerTarget.Library, CSharpCompilerFrameworkVersion.Dotnet4);
            byte[] bytes = ((ByteTransformResult)transformer.Transform(source)).Contents;
            AddArtifact(report, "C# compiler", $"run-{i}", "inert C# library: Fixture.Value()=>42", output, $"csharp-{i}.dll", bytes, "PE/.NET assembly", "observed", "Compiled by shipped Roslyn transformer; artifact not executed.");
        }
    }

    private static async Task RunBFlat(RuntimeReport report, string output)
    {
        const string source = "using System; public static class Program { public static int Main() { return 0; } }";
        for (int i = 1; i <= 2; i++)
        {
            BFlatCompilerTransformer transformer = new(
                Path.Combine(output, $"bflat-temp-{i}"),
                Path.Combine(Dependencies, "bflat", "bflat.exe"),
                Path.Combine(Dependencies, "bflat", "bflat"),
                BFlatOS.Linux,
                BFlatOptimization.Size,
                reflection: false,
                stackTrace: false,
                globalization: false,
                debug: false,
                BFlatTarget.Console,
                BFlatArchitecture.x64);
            Directory.CreateDirectory(Path.Combine(output, $"bflat-temp-{i}"));
            byte[] bytes = ((ByteTransformResult)transformer.Transform(source)).Contents;
            AddArtifact(report, "BFlat / NativeAOT", $"run-{i}", "inert C# console program returning 0", output, $"bflat-nativeaot-{i}.elf", bytes, "ELF native executable", "observed", "Compiled by shipped BFlat transformer; artifact not executed.");
        }
        await Task.CompletedTask;
    }

    private static async Task RunZig(RuntimeReport report, string output, string zig)
    {
        const string linuxSource = "pub fn main() void {}";
        const string windowsDllSource = "export fn Run(_: ?*anyopaque, _: u32) callconv(.c) bool { return true; }";
        for (int i = 1; i <= 2; i++)
        {
            string temp = Path.Combine(output, $"zig-temp-{i}");
            Directory.CreateDirectory(temp);
            ZigCompilerTransformer transformer = new(temp, zig, zig, ZigOS.Linux, ZigArchitecture.x86_64, ZigOptimization.ReleaseSmall, ZigOutputType.Exe, ZigSubsystem.Console);
            byte[] bytes = ((ByteTransformResult)transformer.Transform(linuxSource)).Contents;
            AddArtifact(report, "Zig", $"run-{i}", "inert Zig main with no effects", output, $"zig-linux-{i}.elf", bytes, "ELF native executable", "observed", "Compiled by shipped Zig transformer; artifact not executed.");
        }

        for (int i = 1; i <= 2; i++)
        {
            string dllTemp = Path.Combine(output, $"zig-pe-temp-{i}");
            Directory.CreateDirectory(dllTemp);
            ZigCompilerTransformer dllTransformer = new(dllTemp, zig, zig, ZigOS.Windows, ZigArchitecture.x86_64, ZigOptimization.ReleaseSmall, ZigOutputType.Lib, ZigSubsystem.Console);
            byte[] dll = ((ByteTransformResult)dllTransformer.Transform(windowsDllSource)).Contents;
            AddArtifact(report, "Zig PE fixture", $"run-{i}", "inert exported Run returning true", output, $"zig-inert-x64-{i}.dll", dll, "PE32+ native DLL", "observed", "Fixture for SRDI conversion only; not loaded or executed.");
        }
        await Task.CompletedTask;
    }

    private static async Task RunPowerShellTransforms(RuntimeReport report, string output)
    {
        const string source = "# fixture comment\n$value = 'fixture-value'\nWrite-Output $value\n";
        for (int i = 1; i <= 2; i++)
        {
            string removed = ((StringTransformResult)new PwshRemoveCommentsTokenTransformer().Transform(source)).Contents;
            AddArtifact(report, "PowerShell remove-comments", $"run-{i}", "benign local script", output, $"powershell-comments-{i}.ps1", Encoding.UTF8.GetBytes(removed), "UTF-8 PowerShell source", "observed", "Pure token transform; output not executed.");

            string encoded = ((StringTransformResult)new PwshStringBase64Transformer().Transform(source)).Contents;
            AddArtifact(report, "PowerShell string-base64", $"run-{i}", "benign local script", output, $"powershell-base64-{i}.ps1", Encoding.UTF8.GetBytes(encoded), "UTF-8 PowerShell source", "observed", "Uses randomized helper name/offset; output not executed.");
        }
        await Task.CompletedTask;
    }

    private static async Task RunZigRaw(RuntimeReport report, string output, string zig)
    {
        const string source = "export fn _start() noreturn { while (true) {} }";
        for (int i = 1; i <= 2; i++)
        {
            string temp = Path.Combine(output, $"zig-raw-temp-{i}");
            Directory.CreateDirectory(temp);
            ZigShellcodeExtractTransformer transformer = new(
                temp,
                zig,
                zig,
                ZigOS.Freestanding,
                ZigArchitecture.x86_64,
                ZigOptimization.ReleaseSmall);
            byte[] bytes = ((ByteTransformResult)transformer.Transform(source)).Contents;
            AddArtifact(
                report,
                "Zig raw-section extractor",
                $"run-{i}",
                "inert freestanding _start containing only an infinite loop",
                output,
                $"zig-raw-text-{i}.bin",
                bytes,
                "raw .text bytes",
                "observed",
                "Extracted with the shipped transformer and zig objcopy; bytes were never executed.");
        }
        await Task.CompletedTask;
    }

    private static async Task RunPowerShellContracts(RuntimeReport report)
    {
        using (PowerShell ps = PowerShell.Create())
        {
            ps.AddScript("'first'; 'second'; [byte[]](1,2,3)");
            var values = ps.Invoke();
            report.Checks["multiOutput"] = new
            {
                outputCount = values.Count,
                types = values.Select(x => x?.BaseObject?.GetType().FullName).ToArray(),
                firstValue = values.FirstOrDefault()?.BaseObject?.ToString(),
                serviceBehavior = "PayloadPipelineService uses First()/FirstOrDefault(); later outputs are ignored."
            };
        }

        using (PowerShell ps = PowerShell.Create())
        using (CancellationTokenSource cts = new(TimeSpan.FromMilliseconds(250)))
        {
            ps.AddScript("Start-Sleep -Seconds 30; 'late'");
            using CancellationTokenRegistration registration = cts.Token.Register(ps.Stop);
            Stopwatch sw = Stopwatch.StartNew();
            Exception? observed = null;
            try
            {
                await Task.Run(() => ps.Invoke());
            }
            catch (Exception ex)
            {
                observed = ex;
            }
            sw.Stop();
            report.Checks["cancellation"] = new
            {
                requestedAfterMs = 250,
                elapsedMs = sw.ElapsedMilliseconds,
                invocationState = ps.InvocationStateInfo.State.ToString(),
                exception = observed?.GetType().FullName,
                errorCount = ps.Streams.Error.Count
            };
        }
    }

    private static async Task RunSrdi(RuntimeReport report, string output)
    {
        byte[] original = await File.ReadAllBytesAsync(Path.Combine(output, "zig-inert-x64-1.dll"));
        for (int i = 1; i <= 2; i++)
        {
            byte[] input = original.ToArray();
            byte[] bytes = SRDI.ConvertToShellcode(input, "Run", "fixture", SrdiFlags.None);
            AddArtifact(report, "SRDI", $"run-{i}", "inert x64 PE DLL exported Run", output, $"srdi-{i}.bin", bytes, "raw SRDI wrapper bytes", "observed", "Converted but never mapped or executed; converter zeroes the first two bytes of its input array.");
            report.Checks[$"srdiInputMutationRun{i}"] = new { before = "4d5a", after = Convert.ToHexString(input.AsSpan(0, 2)).ToLowerInvariant() };
        }
    }

    private static async Task RunTemplateInjection(RuntimeReport report, string output)
    {
        string[] templates =
        [
            Path.Combine(Root, "templates", "x86", "specter-win-x86-exe.exe"),
            Path.Combine(Root, "templates", "x64", "specter-win-x64-exe.exe")
        ];

        foreach (string template in templates)
        {
            string arch = template.Contains("x86") ? "x86" : "x64";
            byte[] pristine = await File.ReadAllBytesAsync(template);
            int binaryOffset = Find(pristine, BinaryMarker);
            byte[] mapMarker = (byte[])typeof(ByteEncodingMap).GetField("MARKER", BindingFlags.NonPublic | BindingFlags.Static)!.GetValue(null)!;
            int mapOffset = Find(pristine, mapMarker);
            byte[] parameterMarker = Encoding.UTF8.GetBytes("amsi.dll");
            int parameterOffset = Find(pristine, parameterMarker);
            (string sectionName, int sectionStart, int sectionEnd) = FindContainingPeSection(pristine, binaryOffset);
            int contiguousCapacity = sectionEnd - binaryOffset;

            report.Checks[$"templateMarkers-{arch}"] = new
            {
                file = template,
                fileSize = pristine.Length,
                binaryOffset,
                decoderOffset = mapOffset,
                parametersOffset = parameterOffset,
                containingSection = sectionName,
                sectionRawStart = sectionStart,
                sectionRawEnd = sectionEnd,
                capacityToSectionRawEnd = contiguousCapacity,
                explicitCapacityCheckInCompileAsync = false
            };

            for (int run = 1; run <= 2; run++)
            {
                byte[] data = pristine.ToArray();
                byte[] core = Enumerable.Range(0, Math.Min(4096, contiguousCapacity)).Select(x => (byte)(x % 251)).ToArray();
                byte[] encodedCore = core.ToArray();
                ByteEncodingMap map = ByteEncodingMap.Generate();
                map.Encode(encodedCore);
                BitConverter.GetBytes(encodedCore.Length).CopyTo(data, binaryOffset - 4);
                encodedCore.CopyTo(data, binaryOffset);
                bool mapUpdated = map.TryUpdatePayload(data);

                int parameterSize = GetParameterStructSize();
                byte[] tailBefore = data.AsSpan(parameterOffset + parameterSize - IntPtr.Size, IntPtr.Size).ToArray();
                Architecture architecture = arch == "x86" ? Architecture.x86 : Architecture.x64;
                bool parametersUpdated = new BinaryParameters().TryUpdatePayload(data, map, architecture, BinaryParameters.SpecterAmsiBypassType.None);
                byte[] tailAfter = data.AsSpan(parameterOffset + parameterSize - IntPtr.Size, IntPtr.Size).ToArray();

                AddArtifact(report, "PE template injection", $"{arch}-run-{run}", $"{arch} shipped template + 4096-byte inert pattern", output, $"template-injected-{arch}-{run}.exe", data, arch == "x86" ? "PE32 executable" : "PE32+ executable", "observed", "Exact marker/update primitives; AsmResolver version rewrite and execution intentionally omitted.");
                report.Checks[$"templateUpdate-{arch}-{run}"] = new
                {
                    mapUpdated,
                    parametersUpdated,
                    pointerSizedTailBytes = IntPtr.Size,
                    tailBefore = Convert.ToHexString(tailBefore).ToLowerInvariant(),
                    tailAfter = Convert.ToHexString(tailAfter).ToLowerInvariant(),
                    tailUnchanged = tailBefore.SequenceEqual(tailAfter)
                };
            }

            byte[] exact = Enumerable.Repeat((byte)0xA5, contiguousCapacity).ToArray();
            byte[] over = Enumerable.Repeat((byte)0x5A, contiguousCapacity + 1).ToArray();
            bool exactFitsArray = binaryOffset + exact.Length <= pristine.Length;
            bool overFitsArray = binaryOffset + over.Length <= pristine.Length;
            byte[] exactProbe = pristine.ToArray();
            byte[] overProbe = pristine.ToArray();
            bool exactCopySucceeded = TryCopy(exact, exactProbe, binaryOffset);
            byte nextSectionByteBefore = pristine[sectionEnd];
            bool overCopySucceeded = TryCopy(over, overProbe, binaryOffset);
            byte nextSectionByteAfter = overProbe[sectionEnd];
            report.Checks[$"markerCapacity-{arch}"] = new
            {
                inferredCapacityToSectionRawEnd = contiguousCapacity,
                exactFitsBackingArray = exactFitsArray,
                overFitsBackingArray = overFitsArray,
                exactCopySucceeded,
                overCopySucceeded,
                nextSectionByteBefore,
                nextSectionByteAfter,
                overDidOverwriteNextSection = nextSectionByteAfter == 0x5A && nextSectionByteBefore != nextSectionByteAfter,
                compileAsyncRejectsOversizeBeforeCopy = false
            };
        }
    }

    private static async Task RunCacheContracts(RuntimeReport report)
    {
        Type serviceType = typeof(PayloadCachingService);
        Type keyType = serviceType.GetNestedType("PayloadCacheKey", BindingFlags.NonPublic)!;
        ConstructorInfo keyCtor = keyType.GetConstructors(BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic).Single();
        object NewKey(IDictionary<string, string> p) => keyCtor.Invoke(["fixture-build", "fixture-kind", p]);

        object empty = NewKey(new Dictionary<string, string>());
        object extra = NewKey(new Dictionary<string, string> { ["a"] = "1" });
        object sameA = NewKey(new Dictionary<string, string> { ["a"] = "1", ["b"] = "2" });
        object sameB = NewKey(new Dictionary<string, string> { ["b"] = "2", ["a"] = "1" });
        report.Checks["cacheKeyEquality"] = new
        {
            emptyEqualsExtra = empty.Equals(extra),
            extraEqualsEmpty = extra.Equals(empty),
            reorderedEquals = sameA.Equals(sameB),
            reorderedHashEqual = sameA.GetHashCode() == sameB.GetHashCode()
        };

        const int workers = 24;
        ConcurrentDictionary<object, byte[]> cache = new();
        Barrier barrier = new(workers);
        int generated = 0;
        int added = 0;
        byte[][] returned = new byte[workers][];
        Task[] tasks = Enumerable.Range(0, workers).Select(i => Task.Run(() =>
        {
            object key = NewKey(new Dictionary<string, string> { ["x"] = "y" });
            if (!cache.TryGetValue(key, out byte[]? hit))
            {
                barrier.SignalAndWait();
                Interlocked.Increment(ref generated);
                byte[] candidate = BitConverter.GetBytes(i);
                if (cache.TryAdd(key, candidate)) Interlocked.Increment(ref added);
                returned[i] = candidate;
            }
            else returned[i] = hit;
        })).ToArray();
        await Task.WhenAll(tasks);
        report.Checks["cacheRaceModel"] = new
        {
            implementationPattern = "same TryGetValue -> generate -> TryAdd sequence as PayloadCachingService",
            workers,
            generated,
            successfulAdds = added,
            cachedEntries = cache.Count,
            distinctCallerResults = returned.Select(Convert.ToHexString).Distinct().Count(),
            scope = "service-equivalent concurrency contract using the real private key type; PayloadsService was not invoked"
        };

        Type valueType = serviceType.GetNestedType("PayloadCacheValue", BindingFlags.NonPublic)!;
        Type actionType = typeof(Action<>).MakeGenericType(keyType);
        ParameterExpression parameter = Expression.Parameter(keyType, "key");
        MethodInfo callback = typeof(Program).GetMethod(nameof(TimerCallback), BindingFlags.Static | BindingFlags.NonPublic)!;
        Delegate callbackDelegate = Expression.Lambda(actionType, Expression.Call(callback, Expression.Convert(parameter, typeof(object))), parameter).Compile();
        PayloadCachePolicy policy = new("fixture-kind", TimeSpan.FromMilliseconds(180), regenerateOnExpiration: true);
        ConstructorInfo valueCtor = valueType.GetConstructors(BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic).Single();
        _timerCallbacks = 0;
        TimerSignal.Reset();
        Stopwatch sw = Stopwatch.StartNew();
        _ = valueCtor.Invoke([new byte[] { 1, 2, 3 }, policy, callbackDelegate, NewKey(new Dictionary<string, string>())]);
        bool signaled = TimerSignal.Wait(TimeSpan.FromSeconds(3));
        sw.Stop();
        report.Checks["cacheTimer"] = new
        {
            signaled,
            callbackCount = _timerCallbacks,
            elapsedMs = sw.ElapsedMilliseconds,
            regeneratePolicy = policy.RegenerateOnExpiration,
            scope = "actual private PayloadCacheValue timer/callback; outer async regeneration requires live PayloadsService and was not invoked"
        };

        _timerCallbacks = 0;
        TimerSignal.Reset();
        object resetValue = valueCtor.Invoke([new byte[] { 4, 5, 6 }, policy, callbackDelegate, NewKey(new Dictionary<string, string>())]);
        Stopwatch resetWatch = Stopwatch.StartNew();
        await Task.Delay(100);
        valueType.GetMethod("ResetExpiration", BindingFlags.Instance | BindingFlags.Public)!.Invoke(resetValue, null);
        bool resetSignaled = TimerSignal.Wait(TimeSpan.FromSeconds(3));
        resetWatch.Stop();
        report.Checks["cacheTimerReset"] = new
        {
            signaled = resetSignaled,
            callbackCount = _timerCallbacks,
            elapsedMs = resetWatch.ElapsedMilliseconds,
            resetAfterMs = 100,
            policyMs = policy.Lifespan.TotalMilliseconds,
            expirationWasExtended = resetWatch.ElapsedMilliseconds >= 240
        };
    }

    private static void TimerCallback(object _) { Interlocked.Increment(ref _timerCallbacks); TimerSignal.Set(); }

    private static int GetParameterStructSize()
    {
        Type nested = typeof(BinaryParameters).GetNestedType("PARAMETERS", BindingFlags.NonPublic)!;
        return System.Runtime.InteropServices.Marshal.SizeOf(nested);
    }

    private static int Find(byte[] haystack, byte[] needle)
    {
        for (int i = 0; i <= haystack.Length - needle.Length; i++)
        {
            if (haystack.AsSpan(i, needle.Length).SequenceEqual(needle)) return i;
        }
        return -1;
    }

    private static bool TryCopy(byte[] source, byte[] destination, int offset)
    {
        try
        {
            source.CopyTo(destination, offset);
            return true;
        }
        catch (ArgumentException)
        {
            return false;
        }
    }

    private static (string Name, int Start, int End) FindContainingPeSection(byte[] pe, int fileOffset)
    {
        int peOffset = BitConverter.ToInt32(pe, 0x3c);
        ushort sectionCount = BitConverter.ToUInt16(pe, peOffset + 6);
        ushort optionalHeaderSize = BitConverter.ToUInt16(pe, peOffset + 20);
        int sectionTable = peOffset + 24 + optionalHeaderSize;
        for (int i = 0; i < sectionCount; i++)
        {
            int row = sectionTable + i * 40;
            string name = Encoding.ASCII.GetString(pe, row, 8).TrimEnd('\0');
            int rawSize = BitConverter.ToInt32(pe, row + 16);
            int rawStart = BitConverter.ToInt32(pe, row + 20);
            int rawEnd = rawStart + rawSize;
            if (fileOffset >= rawStart && fileOffset < rawEnd) return (name, rawStart, rawEnd);
        }
        throw new InvalidDataException($"Offset {fileOffset} is outside PE raw sections.");
    }

    private static void AddArtifact(RuntimeReport report, string pipeline, string run, string input, string outputDirectory, string fileName, byte[] bytes, string format, string status, string limitations)
    {
        string path = Path.Combine(outputDirectory, fileName);
        File.WriteAllBytes(path, bytes);
        report.Artifacts.Add(new ArtifactRecord(pipeline, run, input, path, Convert.ToHexString(SHA256.HashData(bytes)).ToLowerInvariant(), bytes.LongLength, format, status, limitations));
    }
}
