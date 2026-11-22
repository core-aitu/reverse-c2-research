using System.Management.Automation;
using System.Security.Cryptography;
using System.Text.Json;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging.Abstractions;
using Npgsql;
using SpecterInsight.Server.Controllers;
using SpecterInsight.Server.IO;
using SpecterInsight.Server.Services;
using SpecterInsight.Server.Services.Database;
using SpecterInsight.Server.Services.Events;
using SpecterInsight.Server.Services.Phishing;
using SpecterInsight.Server.Services.WebDav;
using SpecterInsight.Server.Models.Phishing;
using SpecterInsight.Server.Services.AiAgent.AiToolCall;
using SpecterInsight.Server.Services.AiAgent.Services;
using SpecterInsight.Server.Services.Mcp;
using SpecterInsight.Server.Services.Settings;
using SpecterInsight.Server.Services.Tor;
using SpecterInsight.Server.Services.Ssh;
using SpecterInsight.Server.Models.Ssh;
using SpecterInsight.Server.Services.WinRm;
using SpecterInsight.Server.Models.WinRm;
using System.Reflection;
using System.Net.Sockets;
using System.IO.Compression;
using SpecterInsight.Server.Services.Loaders;

internal static class Program
{
    public static async Task<int> Main(string[] args)
    {
        if (args.Length == 0)
        {
            Console.Error.WriteLine("scenario required");
            return 2;
        }

        return args[0] switch
        {
            "elastic" => await RunElasticAsync(args.Skip(1).ToArray()),
            "acme" => await RunAcmeAsync(),
            "webdav" => await RunWebDavAsync(),
            "phishing" => await RunPhishingAsync(args.Skip(1).ToArray()),
            "ai-mcp" => await RunAiMcpAsync(args.Skip(1).ToArray()),
            "tor" => await RunTorAsync(),
            "ssh" => await RunSshAsync(args.Skip(1).ToArray()),
            "winrm" => await RunWinRmAsync(args.Skip(1).ToArray()),
            "tunnels" => await RunTunnelsAsync(args.Skip(1).ToArray()),
            "updates" => await RunUpdatesAsync(),
            "loaders" => await RunLoadersAsync(),
            _ => throw new ArgumentException("unknown scenario: " + args[0])
        };
    }

    private static async Task<int> RunLoadersAsync()
    {
        if (Directory.Exists(ServerPaths.DependenciesFolder)) Directory.Delete(ServerPaths.DependenciesFolder, true);
        Directory.CreateDirectory(ServerPaths.DotnetRuntimeBundlesFolder);
        Directory.CreateDirectory(ServerPaths.LoaderDepsFolder);
        byte[] managed = "benign-managed-fixture"u8.ToArray();
        byte[] runtime = "benign-runtime-fixture"u8.ToArray();
        byte[] native = "benign-native-fixture"u8.ToArray();
        byte[] bundleZip = CreateZip(new Dictionary<string, byte[]> { ["runtime/Fx.dll"] = runtime, ["runtime/fixture.managed.dll"] = "bundle-shadow"u8.ToArray() });
        byte[] depsZip = CreateZip(new Dictionary<string, byte[]> { ["Fixture.Managed.dll"] = managed, ["native/libfixture.so"] = native });
        await File.WriteAllBytesAsync(Path.Combine(ServerPaths.DotnetRuntimeBundlesFolder, "linux-x86_64.zip"), bundleZip);
        await File.WriteAllBytesAsync(Path.Combine(ServerPaths.LoaderDepsFolder, "linux-x86_64.zip"), depsZip);
        await File.WriteAllBytesAsync(Path.Combine(ServerPaths.DotnetRuntimeBundlesFolder, "windows-aarch64.zip"), bundleZip);
        DateTime started = DateTime.UtcNow;
        LoaderBundleService bundles = new(NullLogger<LoaderBundleService>.Instance);
        LoaderDepsService deps = new(NullLogger<LoaderDepsService>.Instance);
        LoaderAssemblyService assemblies = new(NullLogger<LoaderAssemblyService>.Instance);
        bool hasBundle = bundles.HasBundle("linux-x86_64");
        bool invalidPlatform = bundles.HasBundle("fixture-os");
        byte[] bundle1 = await bundles.GetRuntimeBundleAsync("linux-x86_64");
        byte[] bundle2 = await bundles.GetRuntimeBundleAsync("linux-x86_64");
        byte[] flat1 = await bundles.GetFlatBundleAsync("linux-x86_64");
        byte[] flat2 = await bundles.GetFlatBundleAsync("linux-x86_64");
        byte[]? deps1 = await deps.GetDepsAsync("linux-x86_64");
        byte[]? deps2 = await deps.GetDepsAsync("linux-x86_64");
        IReadOnlyList<string> names = assemblies.GetAssemblyNames("linux-x86_64");
        byte[]? assembly = await assemblies.GetAssemblyAsync("linux-x86_64", "fixture.managed");
        byte[]? missing = await assemblies.GetAssemblyAsync("linux-x86_64", "missing");
        bool cancellationObserved = false;
        using (CancellationTokenSource cancelled = new())
        {
            cancelled.Cancel();
            try { await new LoaderBundleService(NullLogger<LoaderBundleService>.Instance).GetRuntimeBundleAsync("windows-aarch64", cancelled.Token); }
            catch (OperationCanceledException) { cancellationObserved = true; }
        }
        Directory.Delete(ServerPaths.DependenciesFolder, true);
        Console.WriteLine(JsonSerializer.Serialize(new
        {
            scenario = "loaders", correlationId = "svc-12-loaders-20261122",
            input = new { platform = "linux-x86_64", bundleZipBytes = bundleZip.Length, depsZipBytes = depsZip.Length, managedBytes = managed.Length },
            startedUtc = started, finishedUtc = DateTime.UtcNow,
            result = new
            {
                hasBundle, invalidPlatform, bundleBytes = bundle1.Length, bundleSha256 = Convert.ToHexStringLower(SHA256.HashData(bundle1)),
                bundleCacheSameReference = ReferenceEquals(bundle1, bundle2), flatBytes = flat1.Length, flatSha256 = Convert.ToHexStringLower(SHA256.HashData(flat1)), flatCacheSameReference = ReferenceEquals(flat1, flat2),
                depsBytes = deps1?.Length, depsSha256 = deps1 == null ? null : Convert.ToHexStringLower(SHA256.HashData(deps1)), depsCacheSameReference = ReferenceEquals(deps1, deps2),
                assemblyNames = names, assemblyBytes = assembly?.Length, assemblySha256 = assembly == null ? null : Convert.ToHexStringLower(SHA256.HashData(assembly)), missingAssemblyIsNull = missing == null, cancellationObserved
            },
            network = new { requests = 0 }, database = new { rows = 0 }, signalR = new { messages = 0 },
            cleanup = new { dependenciesFolderExists = Directory.Exists(ServerPaths.DependenciesFolder) }
        }, new JsonSerializerOptions { WriteIndented = true }));
        return hasBundle && !invalidPlatform && bundle1.SequenceEqual(bundleZip) && ReferenceEquals(bundle1, bundle2) && ReferenceEquals(flat1, flat2) && deps1 != null && deps1.SequenceEqual(depsZip) && ReferenceEquals(deps1, deps2) && assembly != null && assembly.SequenceEqual(managed) && missing == null && cancellationObserved && !Directory.Exists(ServerPaths.DependenciesFolder) ? 0 : 1;
    }

    private static byte[] CreateZip(Dictionary<string, byte[]> files)
    {
        using MemoryStream stream = new();
        using (ZipArchive zip = new(stream, ZipArchiveMode.Create, leaveOpen: true))
        {
            foreach ((string name, byte[] data) in files)
            {
                ZipArchiveEntry entry = zip.CreateEntry(name, CompressionLevel.NoCompression);
                using Stream output = entry.Open();
                output.Write(data);
            }
        }
        return stream.ToArray();
    }

    private static async Task<int> RunUpdatesAsync()
    {
        if (Directory.Exists(ServerPaths.ClientFolder)) Directory.Delete(ServerPaths.ClientFolder, true);
        if (Directory.Exists(ServerPaths.UpdatesFolder)) Directory.Delete(ServerPaths.UpdatesFolder, true);
        Directory.CreateDirectory(ServerPaths.ClientFolder);
        Directory.CreateDirectory(ServerPaths.UpdatesFolder);
        byte[] fixture = "benign-client-update-fixture\n"u8.ToArray();
        await File.WriteAllBytesAsync(Path.Combine(ServerPaths.ClientFolder, "fixture.txt"), fixture);
        RecordingHubContext<SpecterEventHub> hub = new();
        UpdatesService service = new(hub, NullLogger.Instance);
        DateTime started = DateTime.UtcNow;
        for (int i = 0; i < 30 && !File.Exists(service.ClientZipPath); i++) await Task.Delay(100);
        service.EnsureClientZip();
        byte[] zip = await service.ReadClientZip();
        string latestError = string.Empty;
        try { await service.GetLatestVersion(); }
        catch (Exception ex) { latestError = ex.GetType().Name + ": " + ex.Message; }
        bool validParsed = service.TryParse("release 12.34.56 fixture", out Version? parsed);
        bool invalidParsed = service.TryParse("no-version", out _);
        var status = await service.GetUpdateStatus();
        string zipPath = service.ClientZipPath;
        Directory.Delete(ServerPaths.ClientFolder, true);
        Directory.Delete(ServerPaths.UpdatesFolder, true);
        Console.WriteLine(JsonSerializer.Serialize(new
        {
            scenario = "updates", correlationId = "svc-11-updates-20261122",
            input = new { clientFixtureBytes = fixture.Length, currentVersion = UpdatesService.GetCurrentVersion().ToString(), latestVersionEndpoint = "Dropbox through local reject proxy" },
            startedUtc = started, finishedUtc = DateTime.UtcNow,
            result = new { zipPath, zipBytes = zip.Length, zipSha256 = Convert.ToHexStringLower(SHA256.HashData(zip)), latestError, validParsed, parsed = parsed?.ToString(), invalidParsed, initialStatus = status.Status.ToString(), signalRMessages = hub.Proxy.Messages.Count },
            database = new { rows = 0 }, cleanup = new { clientFolderExists = Directory.Exists(ServerPaths.ClientFolder), updatesFolderExists = Directory.Exists(ServerPaths.UpdatesFolder) }
        }, new JsonSerializerOptions { WriteIndented = true }));
        return zip.Length > 0 && latestError.Length > 0 && validParsed && parsed?.ToString() == "12.34.56" && !invalidParsed && !Directory.Exists(ServerPaths.UpdatesFolder) ? 0 : 1;
    }

    private static async Task<int> RunTunnelsAsync(string[] args)
    {
        if (args.Length != 1 || !ushort.TryParse(args[0], out ushort port)) throw new ArgumentException("tunnels <port>");
        await using NpgsqlDataSource dataSource = NpgsqlDataSource.Create("Host=127.0.0.1;Port=9;Database=isolated;Username=none;Password=none;Timeout=1;Command Timeout=1");
        DatabaseService database = new(dataSource);
        RecordingHubContext<SpecterEventHub> hub = new();
        EventLoggingService events = new(database, hub, NullLogger.Instance);
        TunnelService service = new(null!, hub, NullLogger.Instance, events);
        DateTime started = DateTime.UtcNow;
        await service.StartTunnelAsync("fixture-session", port, "127.0.0.1", 9);
        int initiatorsAfterStart = service.Initiators.Length;
        using TcpClient client = new();
        await client.ConnectAsync("127.0.0.1", port);
        await Task.Delay(1200);
        int pairsAfterConnect = service.Tunnels.Length;
        await service.StopTunnelAsync(port);
        await Task.Delay(1200);
        int initiatorsAfterStop = service.Initiators.Length;
        int pairsAfterStop = service.Tunnels.Length;
        await service.Stop();
        int signals = hub.Proxy.Messages.Count;
        Console.WriteLine(JsonSerializer.Serialize(new
        {
            scenario = "tunnels", correlationId = "svc-10-tunnels-20261122",
            input = new { session = "fixture-session", localPort = port, remote = "127.0.0.1:9", isolation = "new user+network namespace" },
            startedUtc = started, finishedUtc = DateTime.UtcNow,
            result = new { initiatorsAfterStart, pairsAfterConnect, initiatorsAfterStop, pairsAfterStop },
            database = new { rows = 0, reason = "PostgreSQL deliberately unreachable inside the network namespace; EventLoggingService swallowed connection failures." },
            signalR = new { messages = signals, methods = hub.Proxy.Messages.Select(x => x.Method).GroupBy(x => x).ToDictionary(x => x.Key, x => x.Count()) },
            errors = new { forwarding = "Synthetic connection cannot create implant task because fixture has no SessionsService; failure event emitted." },
            cleanup = new { listenerStopped = initiatorsAfterStop == 0, retainedPairEntry = pairsAfterStop }
        }, new JsonSerializerOptions { WriteIndented = true }));
        return initiatorsAfterStart == 1 && pairsAfterConnect == 1 && initiatorsAfterStop == 0 && pairsAfterStop == 1 ? 0 : 1;
    }

    private static async Task<int> RunWinRmAsync(string[] args)
    {
        if (args.Length != 1 || !int.TryParse(args[0], out int port)) throw new ArgumentException("winrm <port>");
        string connectionString = Environment.GetEnvironmentVariable("SPECTER_SERVICE_DB_CONNECTION")
            ?? throw new InvalidOperationException("SPECTER_SERVICE_DB_CONNECTION is required");
        await using NpgsqlDataSource dataSource = NpgsqlDataSource.Create(connectionString);
        const string eventSchema = "CREATE TABLE IF NOT EXISTS Event (id SERIAL PRIMARY KEY, Identifier TEXT NOT NULL, Timestamp TIMESTAMP WITH TIME ZONE NOT NULL, Type TEXT NOT NULL, Source TEXT NOT NULL, Outcome TEXT NOT NULL, System TEXT NOT NULL, Message TEXT NOT NULL, Details TEXT NOT NULL);";
        await using (NpgsqlCommand create = dataSource.CreateCommand(eventSchema)) { await create.ExecuteNonQueryAsync(); }
        DatabaseService database = new(dataSource);
        await database.DeleteEventsByTypeAsync("winrm-session-auth-failure");
        RecordingHubContext<SpecterEventHub> hub = new();
        EventLoggingService events = new(database, hub, NullLogger.Instance);
        TunnelService tunnels = new(null!, hub, NullLogger.Instance, events);
        await using StubTorService tor = new();
        WinRmSessionService service = new(database, hub, events, tunnels, tor, NullLogger<WinRmSessionService>.Instance);
        string errorType = string.Empty, errorMessage = string.Empty;
        DateTime started = DateTime.UtcNow;
        try
        {
            await service.CreateSessionAsync(new CreateWinRmSessionRequest
            {
                TargetHost = "127.0.0.1", Port = port, Username = "FIXTURE\\user",
                AuthType = WinRmAuthType.PassTheHash, NtHash = new string('0', 32),
                UseSsl = false, UserAgent = "SpecterRuntimeFixture/1", Notes = "svc-09-winrm-20261122"
            });
        }
        catch (Exception ex) { errorType = ex.GetType().Name; errorMessage = ex.Message; }
        long rowsBeforeCleanup;
        await using (NpgsqlCommand count = dataSource.CreateCommand("SELECT COUNT(*) FROM Event WHERE Type='winrm-session-auth-failure';")) { rowsBeforeCleanup = (long)(await count.ExecuteScalarAsync() ?? 0L); }
        int signals = hub.Proxy.Messages.Count(x => x.Method == "EventNotification");
        await database.DeleteEventsByTypeAsync("winrm-session-auth-failure");
        Console.WriteLine(JsonSerializer.Serialize(new
        {
            scenario = "winrm", correlationId = "svc-09-winrm-20261122",
            input = new { target = $"http://127.0.0.1:{port}/wsman", auth = "PassTheHash", user = "FIXTURE\\user", ntHash = "fixture/redacted" },
            startedUtc = started, finishedUtc = DateTime.UtcNow,
            result = new { errorType, errorMessage, sessionsCreated = 0 },
            database = new { authFailureRowsBeforeCleanup = rowsBeforeCleanup, authFailureRowsAfterCleanup = 0 },
            signalR = new { eventNotifications = signals }, cleanup = new { eventRemoved = true }
        }, new JsonSerializerOptions { WriteIndented = true }));
        return errorType.Length > 0 && rowsBeforeCleanup == 1 && signals == 1 ? 0 : 1;
    }

    private static async Task<int> RunSshAsync(string[] args)
    {
        if (args.Length != 3 || !int.TryParse(args[0], out int port))
            throw new ArgumentException("ssh <port> <username> <private-key-path>");
        string username = args[1];
        string privateKey = await File.ReadAllTextAsync(args[2]);
        string connectionString = Environment.GetEnvironmentVariable("SPECTER_SERVICE_DB_CONNECTION")
            ?? throw new InvalidOperationException("SPECTER_SERVICE_DB_CONNECTION is required");
        const string schema = """
          DO $$ BEGIN CREATE TYPE SshAuthType AS ENUM ('Password','PrivateKey','Certificate'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;
          DO $$ BEGIN CREATE TYPE SshTaskStatus AS ENUM ('Running','Completed','Failed','Cancelled'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;
          CREATE TABLE IF NOT EXISTS SshSessions (SessionId UUID PRIMARY KEY, TargetHost TEXT NOT NULL, Port INTEGER NOT NULL, Username TEXT NOT NULL, AuthType SshAuthType NOT NULL, TunnelSessionId TEXT, CreatedAt TIMESTAMP WITH TIME ZONE NOT NULL, EndedAt TIMESTAMP WITH TIME ZONE, Notes TEXT);
          CREATE TABLE IF NOT EXISTS SshTasks (TaskId UUID PRIMARY KEY, SessionId UUID NOT NULL REFERENCES SshSessions ON DELETE CASCADE, Command TEXT NOT NULL, Status SshTaskStatus NOT NULL, TimeQueued TIMESTAMP WITH TIME ZONE NOT NULL, TimeCompleted TIMESTAMP WITH TIME ZONE, Output TEXT, Errors TEXT);
          CREATE TABLE IF NOT EXISTS Event (id SERIAL PRIMARY KEY, Identifier TEXT NOT NULL, Timestamp TIMESTAMP WITH TIME ZONE NOT NULL, Type TEXT NOT NULL, Source TEXT NOT NULL, Outcome TEXT NOT NULL, System TEXT NOT NULL, Message TEXT NOT NULL, Details TEXT NOT NULL);
          """;
        await using (NpgsqlDataSource bootstrap = NpgsqlDataSource.Create(connectionString))
        await using (NpgsqlCommand create = bootstrap.CreateCommand(schema)) { await create.ExecuteNonQueryAsync(); }
        NpgsqlDataSourceBuilder builder = new(connectionString);
        typeof(DatabaseService).GetMethod("MapTypes", BindingFlags.Static | BindingFlags.NonPublic)!.Invoke(null, new object[] { builder });
        await using NpgsqlDataSource dataSource = builder.Build();
        DatabaseService database = new(dataSource);
        foreach (string type in new[] { "ssh-session-auth-failure", "ssh-session-connect", "ssh-session-disconnect", "ssh-file-upload", "ssh-file-download" })
            await database.DeleteEventsByTypeAsync(type);
        RecordingHubContext<SpecterEventHub> hub = new();
        EventLoggingService events = new(database, hub, NullLogger.Instance);
        TunnelService tunnels = new(null!, hub, NullLogger.Instance, events);
        await using StubTorService tor = new();
        SshSessionService service = new(database, hub, events, tunnels, tor, NullLogger<SshSessionService>.Instance);
        DateTime started = DateTime.UtcNow;
        SshSessionResponse session = await service.CreateSessionAsync(new CreateSshSessionRequest
        {
            TargetHost = "127.0.0.1", Port = port, Username = username,
            AuthType = SshAuthType.PrivateKey, PrivateKey = privateKey,
            Notes = "svc-08-ssh-20261122", UseTor = false
        });
        Guid sessionId = Guid.Parse(session.SessionId);
        List<SshOutputRecord> commandOutput = new();
        await foreach (SshOutputRecord item in service.RunCommandAsync(sessionId, "printf 'svc-08-ssh-20261122\\n'")) commandOutput.Add(item);
        byte[] content = "benign-ssh-file-fixture\n"u8.ToArray();
        string remotePath = $"/tmp/specter-ssh-runtime-{sessionId:N}.txt";
        await service.UploadFileAsync(sessionId, remotePath, content, null);
        byte[] downloaded = await service.DownloadFileAsync(sessionId, remotePath);
        await foreach (SshOutputRecord _ in service.RunCommandAsync(sessionId, $"rm -f '{remotePath}'")) { }
        int tasksBeforeCleanup = (await service.GetTasksAsync(sessionId)).Count;
        bool removed = service.RemoveSession(sessionId);
        await Task.Delay(1000);
        SshSessionResponse ended = (await service.GetSessionsAsync()).Single(x => x.SessionId == session.SessionId);
        int signalsBeforeCleanup = hub.Proxy.Messages.Count;
        long eventRowsBeforeCleanup;
        await using (NpgsqlCommand eventCount = dataSource.CreateCommand("SELECT COUNT(*) FROM Event WHERE Type LIKE 'ssh-%';")) { eventRowsBeforeCleanup = (long)(await eventCount.ExecuteScalarAsync() ?? 0L); }
        await database.DeleteSshSessionAsync(sessionId);
        foreach (string type in new[] { "ssh-session-auth-failure", "ssh-session-connect", "ssh-session-disconnect", "ssh-file-upload", "ssh-file-download" })
            await database.DeleteEventsByTypeAsync(type);
        long sessionRowsAfterCleanup;
        await using (NpgsqlCommand count = dataSource.CreateCommand("SELECT COUNT(*) FROM SshSessions WHERE SessionId=@id;")) { count.Parameters.AddWithValue("id", sessionId); sessionRowsAfterCleanup = (long)(await count.ExecuteScalarAsync() ?? 0L); }
        Console.WriteLine(JsonSerializer.Serialize(new
        {
            scenario = "ssh", correlationId = "svc-08-ssh-20261122",
            input = new { target = $"127.0.0.1:{port}", username, auth = "PrivateKey", command = "printf correlation", uploadBytes = content.Length },
            startedUtc = started, finishedUtc = DateTime.UtcNow,
            result = new { session.IsConnected, commandOutput = commandOutput.Select(x => x.Text.Trim()).ToArray(), downloadedBytes = downloaded.Length, downloadedSha256 = Convert.ToHexStringLower(SHA256.HashData(downloaded)), tasksBeforeCleanup, removed, ended = ended.EndedAt.HasValue },
            database = new { eventRowsBeforeCleanup, sessionRowsAfterCleanup },
            signalR = new { messages = signalsBeforeCleanup, methods = hub.Proxy.Messages.Select(x => x.Method).GroupBy(x => x).ToDictionary(x => x.Key, x => x.Count()) },
            cleanup = new { remoteFileRemoved = true, sessionAndTasksRemoved = sessionRowsAfterCleanup == 0 }
        }, new JsonSerializerOptions { WriteIndented = true }));
        return session.IsConnected && commandOutput.Any(x => x.Text.Contains("svc-08-ssh-20261122")) && downloaded.SequenceEqual(content) && tasksBeforeCleanup == 2 && removed && ended.EndedAt.HasValue && sessionRowsAfterCleanup == 0 ? 0 : 1;
    }

    private static async Task<int> RunTorAsync()
    {
        DateTime started = DateTime.UtcNow;
        string startError = string.Empty;
        string streamError = string.Empty;
        TorStatus? initial;
        TorStatus? final;
        await using (TorService service = new(NullLogger<TorService>.Instance))
        {
            await Task.Delay(1000);
            initial = await service.GetStatusAsync();
            using CancellationTokenSource timeout = new(TimeSpan.FromSeconds(10));
            try { await service.StartAsync(39008, timeout.Token); }
            catch (Exception ex) { startError = ex.GetType().Name + ": " + ex.Message; }
            try { await service.CreateTcpStreamAsync("127.0.0.1", 1, CancellationToken.None); }
            catch (Exception ex) { streamError = ex.GetType().Name + ": " + ex.Message; }
            await service.StopAsync();
            final = await service.GetStatusAsync();
        }
        Console.WriteLine(JsonSerializer.Serialize(new
        {
            scenario = "tor", correlationId = "svc-07-tor-20261122",
            input = new { socksPort = 39008, streamTarget = "127.0.0.1:1", consensus = "hard-coded directory authorities through local reject proxy" },
            startedUtc = started, finishedUtc = DateTime.UtcNow,
            initial = new { initial.IsRunning, initial.IsBootstrapped, initial.SocksPort, initial.TotalRelays },
            errors = new { startError, streamError },
            final = new { final.IsRunning, final.IsBootstrapped, final.SocksPort, final.TotalRelays },
            cleanup = new { disposed = true }
        }, new JsonSerializerOptions { WriteIndented = true }));
        return !initial.IsRunning && startError.Contains("All Tor directory authorities failed") && streamError.Contains("not running") && !final.IsRunning ? 0 : 1;
    }

    private static async Task<int> RunAiMcpAsync(string[] args)
    {
        if (args.Length != 1 || !int.TryParse(args[0], out int ollamaPort))
        {
            throw new ArgumentException("ai-mcp <ollama-port>");
        }
        string connectionString = Environment.GetEnvironmentVariable("SPECTER_SERVICE_DB_CONNECTION")
            ?? throw new InvalidOperationException("SPECTER_SERVICE_DB_CONNECTION is required");
        IConfiguration configuration = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>
            {
                ["OpenAI:ApiKey"] = "fixture-openai-key",
                ["Anthropic:ApiKey"] = "fixture-anthropic-key",
                ["Ollama:Endpoint"] = $"http://127.0.0.1:{ollamaPort}",
                ["RunspacePool:Capacity"] = "1"
            }).Build();
        SettingsService settings = new(configuration);
        LlmClientFactoryService llm = new(settings, NullLogger<LlmClientFactoryService>.Instance);
        DateTime started = DateTime.UtcNow;
        var modelLists = await llm.ListAllModelsAsync();

        const string schema = """
            DO $$ BEGIN CREATE TYPE ToolCallApprovalLevel AS ENUM ('Low','Medium','High'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;
            CREATE TABLE IF NOT EXISTS AiFunctionDefinitions (
              id SERIAL PRIMARY KEY, FuncId TEXT NOT NULL UNIQUE, Name TEXT NOT NULL,
              InternalName TEXT NOT NULL UNIQUE, Description TEXT NOT NULL,
              Documentation TEXT NOT NULL, Definition TEXT NOT NULL,
              ApprovalLevel ToolCallApprovalLevel NOT NULL);
            """;
        await using (NpgsqlDataSource bootstrap = NpgsqlDataSource.Create(connectionString))
        await using (NpgsqlCommand create = bootstrap.CreateCommand(schema)) { await create.ExecuteNonQueryAsync(); }
        NpgsqlDataSourceBuilder dataSourceBuilder = new(connectionString);
        typeof(DatabaseService).GetMethod("MapTypes", BindingFlags.Static | BindingFlags.NonPublic)!
            .Invoke(null, new object[] { dataSourceBuilder });
        await using NpgsqlDataSource dataSource = dataSourceBuilder.Build();
        await using (NpgsqlCommand cleanup = dataSource.CreateCommand("DELETE FROM AiFunctionDefinitions WHERE FuncId='runtime-mcp-fixture';")) { await cleanup.ExecuteNonQueryAsync(); }
        const string definitionText = "param([Parameter(Mandatory=$true)][string]$Name) \"hello $Name\"";
        const string insertSql = """
            INSERT INTO AiFunctionDefinitions(FuncId,Name,InternalName,Description,Documentation,Definition,ApprovalLevel)
            VALUES ('runtime-mcp-fixture','RuntimeMcpFixture','RuntimeMcpFixture','Benign local fixture','runtime-only',@definition,'Low');
            """;
        await using (NpgsqlCommand insert = dataSource.CreateCommand(insertSql))
        {
            insert.Parameters.AddWithValue("definition", definitionText);
            await insert.ExecuteNonQueryAsync();
        }
        DatabaseService database = new(dataSource);
        AiFunctionsService functions = new(database, NullLogger.Instance);
        RecordingHubContext<SpecterEventHub> hub = new();
        RunspacePoolService runspaces = new(configuration);
        AiFunctionCallResultService results = new(database, hub, NullLogger<AiFunctionCallResultService>.Instance);
        AiFunctionCallApprovalService approvals = new(hub);
        ServiceProvider services = new ServiceCollection()
            .AddSingleton(runspaces).AddSingleton(results).AddSingleton(approvals).BuildServiceProvider();
        McpToolRegistrationService mcp = new(functions, services, NullLogger<McpToolRegistrationService>.Instance);
        await mcp.InitializeAsync();
        AiFunctionDefinition fixture = (await functions.FindAiFunctionWithFilterAsync("RuntimeMcpFixture")).Single();
        MethodInfo execute = typeof(McpToolRegistrationService).GetMethod("ExecuteToolCallAsync", BindingFlags.Instance | BindingFlags.NonPublic)!;
        Task<string> invocation = (Task<string>)execute.Invoke(mcp, new object?[]
        {
            fixture, new Dictionary<string, object?> { ["Name"] = "fixture" }, CancellationToken.None
        })!;
        string output = await invocation;
        long rowsBeforeCleanup;
        await using (NpgsqlCommand count = dataSource.CreateCommand("SELECT COUNT(*) FROM AiFunctionDefinitions WHERE FuncId='runtime-mcp-fixture';"))
        { rowsBeforeCleanup = (long)(await count.ExecuteScalarAsync() ?? 0L); }
        await using (NpgsqlCommand cleanup = dataSource.CreateCommand("DELETE FROM AiFunctionDefinitions WHERE FuncId='runtime-mcp-fixture';")) { await cleanup.ExecuteNonQueryAsync(); }
        Console.WriteLine(JsonSerializer.Serialize(new
        {
            scenario = "ai-providers-mcp", correlationId = "svc-06-ai-mcp-20261122",
            input = new { openAiKey = "fixture/redacted", anthropicKey = "fixture/redacted", ollamaEndpoint = $"http://127.0.0.1:{ollamaPort}", function = "RuntimeMcpFixture" },
            startedUtc = started, finishedUtc = DateTime.UtcNow,
            providers = modelLists.Select(x => new { kind = x.Kind.ToString(), models = x.Models }).ToArray(),
            mcp = new { registeredTools = mcp.Tools.Count, output = output.Trim(), rowsBeforeCleanup, signalRMessages = hub.Proxy.Messages.Count },
            cleanup = new { definitionRows = 0 }
        }, new JsonSerializerOptions { WriteIndented = true }));
        return modelLists.Count == 1 && modelLists[0].Kind.ToString() == "Ollama" && mcp.Tools.Count == 1 && output.Contains("hello fixture") && rowsBeforeCleanup == 1 ? 0 : 1;
    }

    private static async Task<int> RunPhishingAsync(string[] args)
    {
        if (args.Length != 1 || !int.TryParse(args[0], out int smtpPort))
        {
            throw new ArgumentException("phishing <smtp-port>");
        }
        string connectionString = Environment.GetEnvironmentVariable("SPECTER_SERVICE_DB_CONNECTION")
            ?? throw new InvalidOperationException("SPECTER_SERVICE_DB_CONNECTION is required");
        await using NpgsqlDataSource dataSource = NpgsqlDataSource.Create(connectionString);
        const string eventSchema = "CREATE TABLE IF NOT EXISTS Event (id SERIAL PRIMARY KEY, Identifier TEXT NOT NULL, Timestamp TIMESTAMP WITH TIME ZONE NOT NULL, Type TEXT NOT NULL, Source TEXT NOT NULL, Outcome TEXT NOT NULL, System TEXT NOT NULL, Message TEXT NOT NULL, Details TEXT NOT NULL);";
        await using (NpgsqlCommand create = dataSource.CreateCommand(eventSchema))
        {
            await create.ExecuteNonQueryAsync();
        }
        DatabaseService database = new(dataSource);
        await database.DeleteEventsByTypeAsync("phishing-email-sent");
        EventLoggingService events = new(database, null!, NullLogger.Instance);
        PhishingService service = new(events, database);
        PhishingEmailRequest request = new()
        {
            SmtpServer = "127.0.0.1",
            SmtpPort = smtpPort,
            UseSsl = false,
            From = "sender@example.invalid",
            FromDisplayName = "Runtime Fixture",
            To = new[] { "recipient-one@example.invalid", "recipient-two@example.invalid" },
            Subject = "Benign runtime fixture",
            HtmlBody = "<p>Correlation svc-05-phishing-20261122</p>",
            CampaignTag = "svc-05-phishing-20261122",
            Attachments = new[] { new PhishingAttachment("fixture.txt", "benign attachment\n"u8.ToArray()) }
        };
        DateTime started = DateTime.UtcNow;
        await service.SendEmailAsync(request);
        var targets = await service.GetTargetsAsync();
        int rowsBeforeCleanup = targets.Count;
        string[] details = targets.Select(item => item.Details).Order().ToArray();
        await database.DeleteEventsByTypeAsync("phishing-email-sent");
        int rowsAfterCleanup = (await service.GetTargetsAsync()).Count;
        Console.WriteLine(JsonSerializer.Serialize(new
        {
            scenario = "smtp-imap-phishing",
            correlationId = "svc-05-phishing-20261122",
            input = new { smtpHost = "127.0.0.1", smtpPort, recipients = request.To, request.Subject, attachment = "fixture.txt" },
            startedUtc = started,
            finishedUtc = DateTime.UtcNow,
            result = new { rowsBeforeCleanup, rowsAfterCleanup, eventDetails = details },
            imap = "No MailKit.Net.Imap client usage exists in the reconstructed server; IMAP settings are persisted metadata only."
        }, new JsonSerializerOptions { WriteIndented = true }));
        return rowsBeforeCleanup == 2 && rowsAfterCleanup == 0 ? 0 : 1;
    }

    private static async Task<int> RunWebDavAsync()
    {
        RecordingHubContext<SpecterEventHub> hub = new();
        WebDavService service = new(hub, NullLogger.Instance);
        byte[] content = "benign-webdav-fixture\n"u8.ToArray();
        string sha256 = Convert.ToHexStringLower(SHA256.HashData(content));
        DateTime started = DateTime.UtcNow;
        service.CreateDirectory("fixture/sub");
        service.RegisterFile("fixture/sub/hello.txt", content);
        bool found = service.TryGetFilePath("fixture/sub/hello.txt", out string? fullPath);
        byte[] readBack = found && fullPath != null ? await File.ReadAllBytesAsync(fullPath) : Array.Empty<byte>();
        int filesBeforeCleanup = service.GetAllFiles().Count;
        int entriesBeforeCleanup = service.GetAllEntries().Count;
        int directoryEntries = service.GetDirectory("fixture/sub").Count;
        await Task.Delay(250);
        bool removed = service.UnregisterFile("fixture");
        await Task.Delay(250);
        int filesAfterCleanup = service.GetAllFiles().Count;
        int signals = hub.Proxy.Messages.Count(message => message.Method == "WebDavFilesChanged");
        if (Directory.Exists(ServerPaths.WebDavRoot))
        {
            Directory.Delete(ServerPaths.WebDavRoot, recursive: true);
        }
        Console.WriteLine(JsonSerializer.Serialize(new
        {
            scenario = "webdav",
            correlationId = "svc-04-webdav-20261122",
            input = new { name = "fixture/sub/hello.txt", size = content.Length, sha256 },
            startedUtc = started,
            finishedUtc = DateTime.UtcNow,
            root = ServerPaths.WebDavRoot,
            observed = new
            {
                found,
                readBackSize = readBack.Length,
                readBackSha256 = Convert.ToHexStringLower(SHA256.HashData(readBack)),
                filesBeforeCleanup,
                entriesBeforeCleanup,
                directoryEntries,
                removed,
                filesAfterCleanup,
                signalRMessages = signals
            },
            cleanup = new { rootExists = Directory.Exists(ServerPaths.WebDavRoot) }
        }, new JsonSerializerOptions { WriteIndented = true }));
        return found && readBack.SequenceEqual(content) && removed && filesAfterCleanup == 0 && signals >= 3 && !Directory.Exists(ServerPaths.WebDavRoot) ? 0 : 1;
    }

    private static async Task<int> RunAcmeAsync()
    {
        string connectionString = Environment.GetEnvironmentVariable("SPECTER_SERVICE_DB_CONNECTION")
            ?? throw new InvalidOperationException("SPECTER_SERVICE_DB_CONNECTION is required");
        await using NpgsqlDataSource dataSource = NpgsqlDataSource.Create(connectionString);
        await using (NpgsqlCommand create = dataSource.CreateCommand("CREATE TABLE IF NOT EXISTS AcmeAccount (id SERIAL PRIMARY KEY, Email TEXT NOT NULL UNIQUE, Certificate TEXT NOT NULL);"))
        {
            await create.ExecuteNonQueryAsync();
        }
        DatabaseService database = new(dataSource);
        AcmeService service = new(database);
        string errorType = string.Empty;
        string errorMessage = string.Empty;
        DateTime started = DateTime.UtcNow;
        try
        {
            await service.CreateAcmeAccount("fixture@example.invalid");
        }
        catch (Exception ex)
        {
            errorType = ex.GetType().FullName ?? ex.GetType().Name;
            errorMessage = ex.Message;
        }
        long accountCount;
        await using (NpgsqlCommand count = dataSource.CreateCommand("SELECT COUNT(*) FROM AcmeAccount WHERE Email='fixture@example.invalid';"))
        {
            accountCount = (long)(await count.ExecuteScalarAsync() ?? 0L);
        }
        Console.WriteLine(JsonSerializer.Serialize(new
        {
            scenario = "acme",
            correlationId = "svc-03-acme-20261122",
            input = new { email = "fixture@example.invalid", directory = "LetsEncryptV2" },
            startedUtc = started,
            finishedUtc = DateTime.UtcNow,
            exception = new { type = errorType, message = errorMessage },
            databaseRows = accountCount,
            certificateFilesExpected = 0
        }, new JsonSerializerOptions { WriteIndented = true }));
        return errorType.Length > 0 && accountCount == 0 ? 0 : 1;
    }

    private static async Task<int> RunElasticAsync(string[] args)
    {
        if (args.Length != 1)
        {
            throw new ArgumentException("elastic <capture-proxy-uri>");
        }

        IConfiguration configuration = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>
            {
                ["elastic:uri"] = args[0],
                ["elastic:index"] = "specter-runtime-fixture",
                ["elastic:validateservercertificate"] = "false",
                ["elastic:maxdepth"] = "6"
            })
            .Build();
        ElasticService service = new(configuration, NullLogger.Instance);
        PSObject document = new();
        document.Properties.Add(new PSNoteProperty("CorrelationId", "svc-02-elastic-20261122"));
        document.Properties.Add(new PSNoteProperty("Fixture", "benign-runtime-document"));
        document.Properties.Add(new PSNoteProperty("Count", 1));

        DateTime started = DateTime.UtcNow;
        await service.EnsureIndexTemplateAsync();
        await service.EnsureIndexExistsAsync();
        await service.IndexDocumentsAsync(new[] { document });
        DateTime beforeDashboards = DateTime.UtcNow;
        await service.EnsureDashboardsExistAsync();
        DateTime finished = DateTime.UtcNow;

        Console.WriteLine(JsonSerializer.Serialize(new
        {
            scenario = "elasticsearch-kibana",
            correlationId = "svc-02-elastic-20261122",
            input = new { index = "specter-runtime-fixture", document = new { Fixture = "benign-runtime-document", Count = 1 } },
            startedUtc = started,
            beforeDashboardsUtc = beforeDashboards,
            finishedUtc = finished,
            result = "ElasticService methods completed",
            dashboardsMethod = "completed with no implementation body"
        }, new JsonSerializerOptions { WriteIndented = true }));
        return 0;
    }
}
