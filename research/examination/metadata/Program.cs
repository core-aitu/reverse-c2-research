using System.IO.Compression;
using System.Reflection;
using System.Reflection.Metadata;
using System.Reflection.Metadata.Ecma335;
using System.Reflection.PortableExecutable;
using System.Text.Json;

// Arguments: working-copy root, inventory JSONL.gz, output directory.
using var compressed = new GZipStream(File.OpenRead(args[1]), CompressionMode.Decompress);
using var input = new StreamReader(compressed);
using var assemblies = File.CreateText(Path.Combine(args[2], "assemblies.jsonl"));
using var index = new StreamWriter(new GZipStream(File.Create(Path.Combine(args[2], "type-method-index.jsonl.gz")), CompressionLevel.SmallestSize));
using var signatures = File.CreateText(Path.Combine(args[2], "named-signatures.jsonl"));
var signatureNames = new SignatureNames();
var seen = new HashSet<string>();
string? line;
while ((line = input.ReadLine()) != null)
{
    using var record = JsonDocument.Parse(line);
    var obj = record.RootElement;
    if (!obj.TryGetProperty("format", out var format) || format.GetString() != "PE/CLI") continue;
    var path = obj.GetProperty("path").GetString()!;
    var hash = obj.GetProperty("sha256").GetString()!;
    var id = obj.GetProperty("id").GetString()!;
    try
    {
        using var stream = File.OpenRead(Path.Combine(args[0], path));
        using var pe = new PEReader(stream);
        var md = pe.GetMetadataReader();
        var module = md.GetModuleDefinition();
        var mvid = md.GetGuid(module.Mvid);
        var name = md.IsAssembly ? md.GetString(md.GetAssemblyDefinition().Name) : md.GetString(module.Name);
        var version = md.IsAssembly ? md.GetAssemblyDefinition().Version.ToString() : null;
        var own = name.StartsWith("SpecterInsight", StringComparison.OrdinalIgnoreCase)
            || path.StartsWith("modules/") || path.StartsWith("templates/");
        var refs = md.AssemblyReferences.Select(h => {
            var r = md.GetAssemblyReference(h);
            return new {name = md.GetString(r.Name), version = r.Version.ToString()};
        }).ToArray();
        var entry = pe.PEHeaders.CorHeader?.EntryPointTokenOrRelativeVirtualAddress;
        assemblies.WriteLine(JsonSerializer.Serialize(new {id, path, hash, name, version, mvid,
            types = md.TypeDefinitions.Count, methods = md.MethodDefinitions.Count, refs,
            entry, resources = md.ManifestResources.Count, own_candidate = own}));
        if (!own || !seen.Add(hash)) continue;
        foreach (var h in md.TypeDefinitions)
        {
            var type = md.GetTypeDefinition(h);
            string ns = md.GetString(type.Namespace), n = md.GetString(type.Name);
            bool named = n is "DocumentationEntry" or "DocumentationSearchRequest" or "DocumentationChunk" or
                "DocumentationController" or "DocumentationViewModel" or "DocumentationControl" or
                "DocumentationViewer" or "DocumentationService" or "IDocumentationService"
                || (n == "Program" && (name == "SpecterInsight.Server.Host" || name == "SpecterInsight.UI"));
            var methods = type.GetMethods();
            // Each TypeDef owns a contiguous MethodDef range in these images. Guard that invariant.
            var rows = methods.Select(m => MetadataTokens.GetRowNumber(m)).ToArray();
            if (rows.Length > 0 && rows[^1] - rows[0] + 1 != rows.Length) throw new InvalidDataException("noncontiguous MethodDefs");
            if (named)
            {
                foreach (var method in methods)
                {
                    var def=md.GetMethodDefinition(method);
                    var sig=def.DecodeSignature(signatureNames,null);
                    signatures.WriteLine(JsonSerializer.Serialize(new {hash, mvid, type=ns+"."+n,
                        token=MetadataTokens.GetToken(method), name=md.GetString(def.Name),
                        parameters=sig.ParameterTypes, returns=sig.ReturnType}));
                }
            }
            index.WriteLine(JsonSerializer.Serialize(new {hash, type_token = MetadataTokens.GetToken(h),
                type_name = named ? ns+"."+n : null,
                first_method_token = rows.Length > 0 ? (0x06000000 | rows[0]) : (int?)null,
                method_count = rows.Length,
                public_method_tokens = methods.Where(m => (md.GetMethodDefinition(m).Attributes & MethodAttributes.MemberAccessMask) == MethodAttributes.Public).Select(m=>MetadataTokens.GetToken(m)).ToArray(),
                named_methods = named ? methods.Select(m => new {token=MetadataTokens.GetToken(m),
                    name=md.GetString(md.GetMethodDefinition(m).Name), signature=Convert.ToHexString(md.GetBlobBytes(md.GetMethodDefinition(m).Signature))}).ToArray() : null}));
        }
    }
    catch (Exception ex) when (ex is BadImageFormatException or IOException)
    {
        assemblies.WriteLine(JsonSerializer.Serialize(new {id, path, hash, error=ex.Message}));
    }
}
