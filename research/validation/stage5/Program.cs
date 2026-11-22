using System.Reflection;
using System.Runtime.Loader;
using System.Text.Json;
using Npgsql;

var dist = Path.GetFullPath(Environment.GetEnvironmentVariable("STAGE5_DIST")
    ?? throw new InvalidOperationException("STAGE5_DIST is required"));
var serverDir = Path.Combine(dist, "server");
AssemblyLoadContext.Default.Resolving += (_, name) =>
{
    var path = Path.Combine(serverDir, name.Name + ".dll");
    return File.Exists(path) ? AssemblyLoadContext.Default.LoadFromAssemblyPath(path) : null;
};

var assembly = Assembly.LoadFrom(Path.Combine(serverDir, "SpecterInsight.Server.dll"));
var databaseType = assembly.GetType("SpecterInsight.Server.Services.Database.DatabaseService", true)!;
Type[] loadableTypes;
try { loadableTypes = assembly.GetTypes(); }
catch (ReflectionTypeLoadException exception) { loadableTypes = exception.Types.OfType<Type>().ToArray(); }
var globalInitializers = loadableTypes
    .SelectMany(t => t.GetMethods(BindingFlags.Instance | BindingFlags.Static | BindingFlags.Public | BindingFlags.NonPublic)
        .Where(m => m.Name.StartsWith("Create", StringComparison.Ordinal)
            && (m.Name.Contains("Table", StringComparison.OrdinalIgnoreCase)
                || m.Name.Contains("Schema", StringComparison.OrdinalIgnoreCase)
                || m.Name.Contains("Extension", StringComparison.OrdinalIgnoreCase)
                || m.Name.Contains("Enum", StringComparison.OrdinalIgnoreCase)))
        .Select(m => new { type = t.FullName, m.Name, parameters = m.GetParameters().Select(p => p.ParameterType.FullName).ToArray() }))
    .OrderBy(m => m.type, StringComparer.Ordinal).ThenBy(m => m.Name, StringComparer.Ordinal)
    .ToArray();
await using var source = NpgsqlDataSource.Create(
    Environment.GetEnvironmentVariable("STAGE5_DATABASE")
    ?? "Host=/tmp;Username=fixture;Database=postgres;Pooling=false");
var database = Activator.CreateInstance(databaseType, source)!;
var availableInitializers = databaseType.GetMethods(BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic)
    .Where(m => m.GetParameters().Length == 0
        && (typeof(Task).IsAssignableFrom(m.ReturnType) || m.ReturnType == typeof(void))
        && (m.Name.Contains("Create", StringComparison.OrdinalIgnoreCase)
            || m.Name.Contains("Init", StringComparison.OrdinalIgnoreCase)
            || m.Name.Contains("Migration", StringComparison.OrdinalIgnoreCase)
            || m.Name.Contains("Extension", StringComparison.OrdinalIgnoreCase)
            || m.Name.Contains("Type", StringComparison.OrdinalIgnoreCase)))
    .Select(m => new { m.Name, return_type = m.ReturnType.FullName })
    .OrderBy(m => m.Name, StringComparer.Ordinal)
    .ToArray();
var databaseCreateMethods = databaseType.GetMethods(BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic)
    .Where(m => m.Name.StartsWith("Create", StringComparison.Ordinal))
    .Select(m => new { m.Name, parameters = m.GetParameters().Select(p => p.ParameterType.FullName).ToArray() })
    .OrderBy(m => m.Name, StringComparer.Ordinal)
    .ToArray();
string[] initializerOrder =
[
    "EnsureCitextExtension", "EnsureTrigramExtension",
    "CreateAiFunctionCallStatusEnumTypeAsync", "CreateAiFunctionToolCallApprovalLevelEnumTypeAsync",
    "CreateAiSenderTypeEnumTypeAsync", "CreateAiSessionStatusEnumTypeAsync", "CreateToolCallApprovalPolicyEnumTypeAsync",
    "CreateImplantTaskStatusEnumTypeAsync",
    "CreateSshAuthTypeEnumAsync", "CreateSshTaskStatusEnumAsync",
    "CreateWinRmAuthTypeEnumAsync", "CreateWinRmTaskStatusEnumAsync",
    "CreateAccountTableAsync", "CreateAcmeAccountTableAsync", "CreateAiAgentDefinitionTableAsync",
    "CreateAiFunctionDefinitionTableAsync", "CreateAiSessionsTableAsync", "CreateAiFunctionCallResultsTableAsync", "CreateAiMessageTableAsync",
    "CreateArtifactTableAsync", "CreateAiMessageArtifactsTableAsync", "CreateAiAgentFunctionLinksTableAsync",
    "CreateDocumentationChunksTableAsync", "CreateEventsTableAsync", "CreateLabelTableAsync",
    "CreateListenerTableAsync", "CreateNetHostsTableAsync", "CreateNetCachedFilesTableAsync",
    "CreateNetCredentialsTableAsync", "CreateNetHostIpAddressesTableAsync", "CreateNetOpenPortsTableAsync",
    "CreateNetProcessesTableAsync", "CreateNetServicesTableAsync", "CreateNetUsersTableAsync",
    "CreatePayloadPipelinesTableAsync", "CreatePersonaTableAsync", "CreateEmailAccountTableAsync",
    "CreatePowerShellNotebookTableAsync", "CreateRefreshTokensTableAsync", "CreateScriptTableAsync",
    "CreateScriptLabelTableAsync", "CreateSessionTableAsync", "CreateImplantTaskTableAsync",
    "CreateSshSessionTableAsync", "CreateSshTaskTableAsync", "CreateWinRmSessionTableAsync",
    "CreateWinRmTaskTableAsync", "CreateWordlistTableAsync",
    "CreateAccountUsernameIndexAsync", "CreateMachineIdIndexAsync", "CreateNetUsersUniqueIndexAsync",
    "CreateScriptIdIndex", "CreateSessionIdIndexAsync", "CreateSshTaskSessionIdIndexAsync",
    "CreateTaskIdIndexAsync", "CreateWinRmTaskSessionIdIndexAsync"
];
var createMethods = initializerOrder.Select(name => databaseType
        .GetMethods(BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic)
        .SingleOrDefault(m => m.Name == name && m.GetParameters().All(p => p.ParameterType == typeof(CancellationToken)))
        ?? throw new MissingMethodException(databaseType.FullName, name))
    .ToArray();

var applications = new List<object>();
foreach (var method in createMethods)
{
    try
    {
        var arguments = method.GetParameters().Select(_ => (object)CancellationToken.None).ToArray();
        await (Task)method.Invoke(database, arguments)!;
        applications.Add(new { method = method.Name, applied = true, error = (string?)null });
    }
    catch (Exception exception)
    {
        applications.Add(new
        {
            method = method.Name,
            applied = false,
            error = exception.GetBaseException().GetType().Name + ": " + exception.GetBaseException().Message
        });
    }
}

const string catalogSql = """
WITH user_tables AS (
  SELECT c.oid, n.nspname AS schema_name, c.relname AS table_name
  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
  WHERE c.relkind IN ('r','p') AND n.nspname NOT IN ('pg_catalog','information_schema')
), table_rows AS (
  SELECT t.schema_name, t.table_name,
    (SELECT jsonb_agg(jsonb_build_object(
       'ordinal', a.attnum,
       'name', a.attname,
       'type', format_type(a.atttypid,a.atttypmod),
       'not_null', a.attnotnull,
       'identity', nullif(a.attidentity,''),
       'default', pg_get_expr(d.adbin,d.adrelid)
     ) ORDER BY a.attnum)
     FROM pg_attribute a LEFT JOIN pg_attrdef d ON (d.adrelid,d.adnum)=(a.attrelid,a.attnum)
     WHERE a.attrelid=t.oid AND a.attnum>0 AND NOT a.attisdropped) AS columns,
    (SELECT coalesce(jsonb_agg(jsonb_build_object(
       'name', con.conname,
       'type', con.contype,
       'definition', pg_get_constraintdef(con.oid),
       'validated', con.convalidated
     ) ORDER BY con.conname),'[]'::jsonb)
     FROM pg_constraint con WHERE con.conrelid=t.oid) AS constraints,
    (SELECT coalesce(jsonb_agg(jsonb_build_object(
       'name', idx.relname,
       'definition', pg_get_indexdef(idx.oid),
       'unique', i.indisunique,
       'primary', i.indisprimary,
       'valid', i.indisvalid
     ) ORDER BY idx.relname),'[]'::jsonb)
     FROM pg_index i JOIN pg_class idx ON idx.oid=i.indexrelid WHERE i.indrelid=t.oid) AS indexes
  FROM user_tables t
), enum_rows AS (
  SELECT n.nspname AS schema_name, t.typname AS enum_name,
    jsonb_agg(e.enumlabel ORDER BY e.enumsortorder) AS labels
  FROM pg_type t JOIN pg_namespace n ON n.oid=t.typnamespace
  JOIN pg_enum e ON e.enumtypid=t.oid
  WHERE n.nspname NOT IN ('pg_catalog','information_schema')
  GROUP BY n.nspname,t.typname
), extension_rows AS (
  SELECT e.extname AS name, e.extversion AS version, n.nspname AS schema_name
  FROM pg_extension e JOIN pg_namespace n ON n.oid=e.extnamespace
), migration_candidates AS (
  SELECT n.nspname AS schema_name, c.relname AS relation_name
  FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
  WHERE c.relkind IN ('r','v','m') AND n.nspname NOT IN ('pg_catalog','information_schema')
    AND (c.relname ILIKE '%migration%' OR c.relname ILIKE '%version%')
)
SELECT jsonb_build_object(
  'observed_at_utc', timezone('utc',clock_timestamp()),
  'database', current_database(),
  'postgres_version', current_setting('server_version'),
  'tables', coalesce((SELECT jsonb_agg(to_jsonb(x) ORDER BY schema_name,table_name) FROM table_rows x),'[]'::jsonb),
  'enums', coalesce((SELECT jsonb_agg(to_jsonb(x) ORDER BY schema_name,enum_name) FROM enum_rows x),'[]'::jsonb),
  'extensions', coalesce((SELECT jsonb_agg(to_jsonb(x) ORDER BY name) FROM extension_rows x),'[]'::jsonb),
  'migration_relations', coalesce((SELECT jsonb_agg(to_jsonb(x) ORDER BY schema_name,relation_name) FROM migration_candidates x),'[]'::jsonb)
);
""";

await using var command = source.CreateCommand(catalogSql);
var catalog = (string)(await command.ExecuteScalarAsync())!;
var output = new
{
    artifact = new
    {
        path = Path.Combine(serverDir, "SpecterInsight.Server.dll"),
        sha256 = Convert.ToHexString(System.Security.Cryptography.SHA256.HashData(
            File.ReadAllBytes(Path.Combine(serverDir, "SpecterInsight.Server.dll")))).ToLowerInvariant(),
        mvid = databaseType.Module.ModuleVersionId
    },
    global_initializers = globalInitializers,
    available_initializers = availableInitializers,
    database_create_methods = databaseCreateMethods,
    create_methods = applications,
    catalog = JsonSerializer.Deserialize<JsonElement>(catalog)
};
Console.WriteLine(JsonSerializer.Serialize(output, new JsonSerializerOptions { WriteIndented = true }));
