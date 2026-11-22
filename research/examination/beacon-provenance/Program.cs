using System.Reflection.Metadata;
using System.Reflection.Metadata.Ecma335;
using System.Reflection.PortableExecutable;
using System.Security.Cryptography;
using System.Text.Json;

if (args.Length != 2)
    throw new ArgumentException("Expected distribution root and diagnostic-variants.json");
string root = Path.GetFullPath(args[0]);
using var inventory = JsonDocument.Parse(File.ReadAllBytes(args[1]));
string Hash(byte[] b) => Convert.ToHexStringLower(SHA256.HashData(b));
string Relative(string path) => Path.GetRelativePath(root, path);
var pdbIds = new Dictionary<BlobContentId, List<string>>();
int pdbCount = 0, portableCount = 0;
foreach (var file in Directory.EnumerateFiles(root, "*", SearchOption.AllDirectories)
                              .Where(p => p.EndsWith(".pdb", StringComparison.OrdinalIgnoreCase)))
{
    pdbCount++;
    using var stream = File.OpenRead(file);
    using var header = new BinaryReader(stream, System.Text.Encoding.UTF8, true);
    if (stream.Length < 4 || header.ReadUInt32() != 0x424a5342) continue;
    stream.Position = 0;
    using var metadata = MetadataReaderProvider.FromPortablePdbStream(stream);
    var id = new BlobContentId(metadata.GetMetadataReader().DebugMetadataHeader!.Id);
    if (!pdbIds.TryGetValue(id, out var paths)) pdbIds[id] = paths = [];
    paths.Add(Relative(file));
    portableCount++;
}
var rows = new List<object>();
var comparisons = new List<object>();
const string symbolsPath = "server/SpecterInsight.pdb";
const string sourcePath = "templates/source/SpecterInsight/Logging/LogEntry.cs";
byte[] symbols = File.ReadAllBytes(Path.Combine(root, symbolsPath));
using var symbolsProvider = MetadataReaderProvider.FromPortablePdbStream(new MemoryStream(symbols));
var pm = symbolsProvider.GetMetadataReader();
var pdbHeader = pm.DebugMetadataHeader!;
var originalId = new BlobContentId(pdbHeader.Id);
var normalized = (byte[])symbols.Clone();
Array.Clear(normalized, pdbHeader.IdStartOffset, pdbHeader.Id.Length);
var docHandle = pm.Documents.Single(h => pm.GetString(pm.GetDocument(h).Name)
    .Replace('\\', '/').EndsWith("/Logging/LogEntry.cs", StringComparison.Ordinal));
var doc = pm.GetDocument(docHandle);
var source = File.ReadAllBytes(Path.Combine(root, sourcePath));
var lines = File.ReadAllLines(Path.Combine(root, sourcePath));
var hashAlgorithm = pm.GetGuid(doc.HashAlgorithm);
if (hashAlgorithm != new Guid("ff1816ec-aa5e-4d10-87f7-6f4963833460") &&
    hashAlgorithm != new Guid("8829d00f-11b8-4213-878b-770e8597ac16"))
    throw new InvalidDataException("Unexpected source checksum algorithm");
var sourceChecksum = hashAlgorithm == new Guid("ff1816ec-aa5e-4d10-87f7-6f4963833460")
    ? SHA1.HashData(source) : SHA256.HashData(source);
if (!pm.GetBlobBytes(doc.Hash).SequenceEqual(sourceChecksum))
    throw new InvalidDataException("Source checksum mismatch");
var embeddedGuid = new Guid("0e8a571b-6926-466e-b4ad-8ab04611f5fe");
var embedded = pm.GetCustomDebugInformation(docHandle)
                 .Any(h => pm.GetGuid(pm.GetCustomDebugInformation(h).Kind) == embeddedGuid);
foreach (var sample in inventory.RootElement.EnumerateArray())
{
    var paths = sample.GetProperty("paths").EnumerateArray().Select(v => v.GetString()!).ToArray();
    var image = File.ReadAllBytes(Path.Combine(root, paths[0]));
    var hash = Hash(image);
    if (hash != sample.GetProperty("sha256").GetString() ||
        !paths.All(p => hash == Hash(File.ReadAllBytes(Path.Combine(root, p)))))
        throw new InvalidDataException("Image hash mismatch");
    using var pe = new PEReader(new MemoryStream(image));
    var md = pe.GetMetadataReader();
    var entries = pe.ReadDebugDirectory();
    var entry = entries.Single(e => e.Type == DebugDirectoryEntryType.CodeView);
    var codeview = pe.ReadCodeViewDebugDirectoryData(entry);
    var checksum = pe.ReadPdbChecksumDebugDirectoryData(entries.Single(e => e.Type == DebugDirectoryEntryType.PdbChecksum));
    if (checksum.AlgorithmName != "SHA256") throw new InvalidDataException(checksum.AlgorithmName);
    var id = new BlobContentId(codeview.Guid, entry.Stamp);
    var sameId = id == originalId;
    var checksumMatch = SHA256.HashData(normalized).SequenceEqual(checksum.Checksum);
    var matched = pdbIds.GetValueOrDefault(id) ?? [];
    rows.Add(new {paths, sha256 = hash, mvid = md.GetGuid(md.GetModuleDefinition().Mvid),
        codeviewGuid = codeview.Guid, codeviewStamp = entry.Stamp, age = codeview.Age,
        pdbLeaf = codeview.Path.Replace('\\','/').Split('/').Last(), portable = entry.IsPortableCodeView,
        embeddedPortablePdb = entries.Any(e => e.Type == DebugDirectoryEntryType.EmbeddedPortablePdb),
        pdbChecksumExpected = Convert.ToHexStringLower(checksum.Checksum.AsSpan()),
        suppliedSymbolIdMatches = sameId, suppliedSymbolChecksumMatches = checksumMatch,
        matchingPortableFiles = matched.Order(StringComparer.Ordinal)});
    if (!sameId || !checksumMatch) continue;
    var type = md.TypeDefinitions.Single(h => md.GetString(md.GetTypeDefinition(h).Name) == "LogEntry" &&
             md.GetString(md.GetTypeDefinition(h).Namespace) == "SpecterInsight.Logging");
    var oldHashes = sample.GetProperty("methods").EnumerateArray().ToDictionary(
        m => Convert.ToInt32(m.GetProperty("token").GetString(), 16),
        m => m.GetProperty("il_sha256").GetString());
    foreach (var h in md.GetTypeDefinition(type).GetMethods())
    {
        var method = md.GetMethodDefinition(h);
        var body = pe.GetMethodBody(method.RelativeVirtualAddress).GetILBytes()!;
        if (!oldHashes.Remove(MetadataTokens.GetToken(h), out var oldHash) || Hash(body) != oldHash)
            throw new InvalidDataException("Previous IL reading differs from the current image");
        var info = pm.GetMethodDebugInformation(h);
        var points = info.GetSequencePoints().ToArray();
        var visible = points.Where(p => !p.IsHidden).ToArray();
        if (visible.Length == 0 || visible.Any(p => (p.Document.IsNil ? info.Document : p.Document) != docHandle ||
            p.Offset < 0 || p.Offset >= body.Length || p.StartLine < 1 || p.EndLine < p.StartLine ||
            p.EndLine > lines.Length || p.StartColumn < 1 || p.StartColumn > lines[p.StartLine-1].Length + 1 ||
            p.EndColumn < 1 || p.EndColumn > lines[p.EndLine-1].Length + 1))
            throw new InvalidDataException("Sequence point outside method or source");
        comparisons.Add(new {token = $"0x{MetadataTokens.GetToken(h):x8}", name = md.GetString(method.Name),
            ilBytes = body.Length, ilSha256 = Hash(body), hiddenCount = points.Length - visible.Length,
            visible = visible.Select(p => new {il = p.Offset, startLine = p.StartLine, startColumn = p.StartColumn,
                                               endLine = p.EndLine, endColumn = p.EndColumn})});
    }
    if (oldHashes.Count != 0) throw new InvalidDataException("Unmatched prior method");
}
var result = new {pdbCount, portableCount, portableUniqueIds = pdbIds.Count, symbolsPath, symbolSha256 = Hash(symbols),
    symbolGuid = originalId.Guid, symbolStamp = originalId.Stamp, symbolNormalizedSha256 = Hash(normalized),
    sourcePath, sourceSha256 = Hash(source), documentRow = MetadataTokens.GetRowNumber(docHandle),
    documentChecksumAlgorithm = hashAlgorithm, documentChecksum = Convert.ToHexStringLower(sourceChecksum),
    sourceEmbeddedRecord = embedded, comparisons, variants = rows};
Console.WriteLine(JsonSerializer.Serialize(result, new JsonSerializerOptions {WriteIndented = true}));
