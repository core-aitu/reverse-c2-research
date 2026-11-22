using System.Collections;
using System.Reflection;
using System.Runtime.CompilerServices;
using System.Security.Cryptography;
using System.Text.Json;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging.Abstractions;
using Npgsql;
using SpecterInsight.Server.IO;
using SpecterInsight.Server.Services;
using SpecterInsight.Server.Services.Database;
using SpecterInsight.Server.Services.Database.Entities;
using SpecterInsight.Server.Services.Events;

internal static class Program
{
    private const string PipelineName = "runtime_cache_e2e";

    public static async Task<int> Main()
    {
        string? connectionString = Environment.GetEnvironmentVariable("SPECTER_CACHE_DB_CONNECTION");
        if (string.IsNullOrWhiteSpace(connectionString))
        {
            Console.Error.WriteLine("SPECTER_CACHE_DB_CONNECTION is required");
            return 2;
        }

        Directory.CreateDirectory(Path.Combine(ServerPaths.ImplantsFolder, "default"));
        File.Copy(
            "/home/deleteme/Desktop/SpecterInsight/implants/default/configuration.json",
            Path.Combine(ServerPaths.ImplantsFolder, "default", "configuration.json"),
            overwrite: true);

        await using NpgsqlDataSource dataSource = NpgsqlDataSource.Create(connectionString);
        await CreateMinimalSchemaAsync(dataSource);
        DatabaseService database = new(dataSource);

        PayloadPipeline pipeline = new(
            "runtime-cache-e2e",
            PipelineName,
            "Inert cache regeneration fixture",
            "Local isolated runtime fixture",
            "Write-Output -NoEnumerate ([byte[]](1,2,3,4));",
            PipelineOutputFormat.generic_binary,
            isPremium: false,
            enabled: true,
            new Dictionary<string, string>());
        await database.UpsertPayloadPipelineAsync(pipeline);

        IConfiguration configuration = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?> { ["RunspacePool:Capacity"] = "1" })
            .Build();
        RunspacePoolService runspaces = new(configuration);
        ServiceProvider services = new ServiceCollection().BuildServiceProvider();

        PayloadPipelineService pipelineService =
            (PayloadPipelineService)RuntimeHelpers.GetUninitializedObject(typeof(PayloadPipelineService));
        SetField(pipelineService, "_database", database);
        SetField(pipelineService, "_runspaces", runspaces);
        SetField(pipelineService, "_services", services);
        SetField(pipelineService, "_logger", NullLogger.Instance);
        SetField(pipelineService, "_configuration", configuration);

        EventLoggingService events = new(database, null!, NullLogger.Instance);
        PayloadsService payloads = new(
            NullLogger.Instance,
            null!,
            null!,
            pipelineService,
            services,
            events);
        PayloadCachingService cache = new(payloads, events, NullLogger.Instance);
        cache.UpdatePolicy(new PayloadCachePolicy(PipelineName, TimeSpan.FromMilliseconds(700), regenerateOnExpiration: true));

        byte[] first = await cache.GetPayloadAsync(
            PayloadsService.DEFAULT_IMPLANT_SETTINGS,
            PipelineName,
            new Dictionary<string, string>());
        object firstEntry = GetOnlyCacheEntry(cache);

        Dictionary<string, long> counts = new();
        bool regenerated = false;
        for (int i = 0; i < 100; i++)
        {
            await Task.Delay(50);
            counts = await ReadEventCountsAsync(dataSource);
            regenerated = Count(counts, "payload-generated") >= 2
                && Count(counts, "payload-cached") >= 2
                && Count(counts, "payload-cache-expired") >= 1
                && GetCacheCount(cache) == 1
                && !ReferenceEquals(firstEntry, GetOnlyCacheEntry(cache));
            if (regenerated)
            {
                break;
            }
        }

        cache.UpdatePolicy(new PayloadCachePolicy(PipelineName, TimeSpan.FromMinutes(10), regenerateOnExpiration: false));
        byte[] second = await cache.GetPayloadAsync(
            PayloadsService.DEFAULT_IMPLANT_SETTINGS,
            PipelineName,
            new Dictionary<string, string>());
        await Task.Delay(250);
        Dictionary<string, long> stableCounts = await ReadEventCountsAsync(dataSource);

        var report = new
        {
            observed = regenerated,
            database = "disposable local PostgreSQL; minimal Event and PayloadPipelines schema",
            pipeline = PipelineName,
            output = new
            {
                size = first.Length,
                sha256Initial = Convert.ToHexStringLower(SHA256.HashData(first)),
                sha256Regenerated = Convert.ToHexStringLower(SHA256.HashData(second)),
                equal = first.SequenceEqual(second)
            },
            eventCountsAtObservation = counts,
            eventCountsAfterPolicyStop = stableCounts,
            cacheEntries = GetCacheCount(cache),
            implementation = "real PayloadCachingService -> PayloadsService -> PayloadPipelineService -> RunspacePoolService",
            limitations = "PayloadPipelineService constructor was bypassed to avoid its unrelated process patch routine; its real RunPipelineAsync method and all cache/payload methods were used. Event hub was null; database logging remained active."
        };
        Console.WriteLine(JsonSerializer.Serialize(report, new JsonSerializerOptions { WriteIndented = true }));
        return regenerated ? 0 : 1;
    }

    private static async Task CreateMinimalSchemaAsync(NpgsqlDataSource dataSource)
    {
        const string sql = """
            CREATE EXTENSION IF NOT EXISTS citext;
            CREATE TABLE IF NOT EXISTS Event (
                id SERIAL PRIMARY KEY,
                Identifier TEXT NOT NULL,
                Timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
                Type TEXT NOT NULL,
                Source TEXT NOT NULL,
                Outcome TEXT NOT NULL,
                System TEXT NOT NULL,
                Message TEXT NOT NULL,
                Details TEXT NOT NULL
            );
            CREATE TABLE IF NOT EXISTS PayloadPipelines (
                id SERIAL PRIMARY KEY,
                PipelineId TEXT NOT NULL UNIQUE,
                Name CITEXT NOT NULL UNIQUE,
                Description TEXT NOT NULL,
                Documentation TEXT NOT NULL,
                ScriptBlock TEXT NOT NULL,
                OutputFormat TEXT NOT NULL,
                IsPremium BOOLEAN NOT NULL,
                IsEnabled BOOLEAN NOT NULL,
                TestParameters TEXT NOT NULL
            );
            """;
        await using NpgsqlCommand command = dataSource.CreateCommand(sql);
        await command.ExecuteNonQueryAsync();
    }

    private static async Task<Dictionary<string, long>> ReadEventCountsAsync(NpgsqlDataSource dataSource)
    {
        Dictionary<string, long> result = new(StringComparer.Ordinal);
        await using NpgsqlCommand command = dataSource.CreateCommand("SELECT Type, COUNT(*) FROM Event GROUP BY Type;");
        await using NpgsqlDataReader reader = await command.ExecuteReaderAsync();
        while (await reader.ReadAsync())
        {
            result[reader.GetString(0)] = reader.GetInt64(1);
        }
        return result;
    }

    private static long Count(Dictionary<string, long> counts, string key) =>
        counts.TryGetValue(key, out long value) ? value : 0;

    private static void SetField(object instance, string name, object value) =>
        instance.GetType().GetField(name, BindingFlags.Instance | BindingFlags.NonPublic)!.SetValue(instance, value);

    private static IDictionary GetCache(PayloadCachingService cache) =>
        (IDictionary)typeof(PayloadCachingService)
            .GetField("_cache", BindingFlags.Instance | BindingFlags.NonPublic)!
            .GetValue(cache)!;

    private static int GetCacheCount(PayloadCachingService cache) => GetCache(cache).Count;

    private static object GetOnlyCacheEntry(PayloadCachingService cache)
    {
        IDictionary dictionary = GetCache(cache);
        if (dictionary.Count != 1)
        {
            return new object();
        }
        return dictionary.Values.Cast<object>().Single();
    }
}
