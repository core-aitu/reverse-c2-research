using System.Collections.Immutable;
using System.Reflection;
using System.Reflection.Emit;
using System.Reflection.Metadata;
using System.Reflection.Metadata.Ecma335;
using System.Reflection.PortableExecutable;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

if (args.Length != 2)
{
    Console.Error.WriteLine("usage: managed-components <SpecterInsight-root> <output-json>");
    return 2;
}

var root = Path.GetFullPath(args[0]);
var output = Path.GetFullPath(args[1]);
var obfuscationPath = Path.Combine(root, "client", "SpecterInsight.Obfuscation.dll");
var baselinePath = Path.Combine(root, "server", "SpecterInsight.exe");
var beaconPaths = new[]
{
    baselinePath,
    Path.Combine(root, "templates", "any", "SpecterInsight.exe"),
    Path.Combine(root, "templates", "any", "specter-dotnet-exe.exe.bak"),
    Path.Combine(root, "templates", "any", "specter-dotnet-exe.exe.bak2"),
    Path.Combine(root, "templates", "any", "specter-dotnet-exe.exe.bak3"),
};

var obfuscation = AssemblyFacts.Read(obfuscationPath, includeMethods: true);
var beacons = beaconPaths.Select(path => AssemblyFacts.Read(path, includeMethods: true)).ToArray();
var baseline = beacons[0];
var comparisons = beacons.Skip(1).Select(candidate => Compare(baseline, candidate)).ToArray();
var modules = Directory.GetFiles(Path.Combine(root, "modules"), "*.dll")
    .OrderBy(Path.GetFileName, StringComparer.OrdinalIgnoreCase)
    .Select(path => AssemblyFacts.Read(path, includeMethods: false))
    .ToArray();

var document = new
{
    schema = "specter-managed-components-v1",
    generated_utc = DateTimeOffset.UtcNow,
    evidence_kind = "static",
    limitations = new[]
    {
        "Assemblies were parsed without loading or executing them.",
        "normalized_il_sha256 canonicalizes metadata operands and control-flow targets; equality is scoped to the recorded artifact identities.",
        "Decompiler-derived semantic reports remain separate evidence and are not runtime observations."
    },
    obfuscation,
    modules,
    managed_beacons = new { baseline_sha256 = baseline.Sha256, variants = beacons, comparisons }
};

Directory.CreateDirectory(Path.GetDirectoryName(output)!);
File.WriteAllText(output, JsonSerializer.Serialize(document, new JsonSerializerOptions { WriteIndented = true }) + "\n");
return 0;

static object Compare(AssemblyFacts baseline, AssemblyFacts candidate)
{
    var left = baseline.Methods.ToDictionary(x => x.Key, StringComparer.Ordinal);
    var right = candidate.Methods.ToDictionary(x => x.Key, StringComparer.Ordinal);
    var common = left.Keys.Intersect(right.Keys, StringComparer.Ordinal).Order(StringComparer.Ordinal).ToArray();
    var unchanged = common.Where(k => left[k].NormalizedIlSha256 == right[k].NormalizedIlSha256).ToArray();
    var changed = common.Where(k => left[k].NormalizedIlSha256 != right[k].NormalizedIlSha256).ToArray();
    var onlyBaseline = left.Keys.Except(right.Keys, StringComparer.Ordinal).Order(StringComparer.Ordinal).ToArray();
    var onlyCandidate = right.Keys.Except(left.Keys, StringComparer.Ordinal).Order(StringComparer.Ordinal).ToArray();
    return new
    {
        baseline_sha256 = baseline.Sha256,
        candidate_sha256 = candidate.Sha256,
        baseline_mvid = baseline.Mvid,
        candidate_mvid = candidate.Mvid,
        counts = new
        {
            baseline_methods = left.Count,
            candidate_methods = right.Count,
            matched_by_identity = common.Length,
            unchanged_normalized_il = unchanged.Length,
            changed_normalized_il = changed.Length,
            only_baseline = onlyBaseline.Length,
            only_candidate = onlyCandidate.Length
        },
        changed_methods = changed,
        only_baseline_methods = onlyBaseline,
        only_candidate_methods = onlyCandidate
    };
}

sealed class AssemblyFacts
{
    public required string Path { get; init; }
    public required string Sha256 { get; init; }
    public required string Name { get; init; }
    public required string? Version { get; init; }
    public required string Mvid { get; init; }
    public required int TypeCount { get; init; }
    public required int MethodCount { get; init; }
    public required object[] NamespaceCounts { get; init; }
    public required string[] References { get; init; }
    public required PInvokeFact[] PInvokes { get; init; }
    public required MethodFact[] Methods { get; init; }

    public static AssemblyFacts Read(string path, bool includeMethods)
    {
        using var stream = File.OpenRead(path);
        using var pe = new PEReader(stream);
        var md = pe.GetMetadataReader();
        var names = new MetadataNames(md);
        var typeMethods = md.TypeDefinitions.ToDictionary(h => h, h => md.GetTypeDefinition(h).GetMethods().Count);
        var nsCounts = md.TypeDefinitions
            .GroupBy(h => names.NamespaceOf(h), StringComparer.Ordinal)
            .Select(g => new { @namespace = g.Key, types = g.Count(), methods = g.Sum(h => typeMethods[h]) })
            .OrderByDescending(x => x.types).ThenBy(x => x.@namespace, StringComparer.Ordinal)
            .Cast<object>().ToArray();

        var methodOwners = new Dictionary<MethodDefinitionHandle, TypeDefinitionHandle>();
        var fieldOwners = new Dictionary<FieldDefinitionHandle, TypeDefinitionHandle>();
        foreach (var th in md.TypeDefinitions)
        {
            var td = md.GetTypeDefinition(th);
            foreach (var mh in td.GetMethods()) methodOwners[mh] = th;
            foreach (var fh in td.GetFields()) fieldOwners[fh] = th;
        }
        names.SetOwners(methodOwners, fieldOwners);

        var methods = new List<MethodFact>();
        var pinvokes = new List<PInvokeFact>();
        foreach (var mh in md.MethodDefinitions)
        {
            var method = md.GetMethodDefinition(mh);
            var owner = methodOwners[mh];
            var key = names.TypeName(owner) + "::" + md.GetString(method.Name) + names.MethodSignature(method);
            if ((method.Attributes & MethodAttributes.PinvokeImpl) != 0)
            {
                var import = method.GetImport();
                pinvokes.Add(new PInvokeFact(
                    key,
                    md.GetString(md.GetModuleReference(import.Module).Name),
                    md.GetString(import.Name),
                    import.Attributes.ToString()));
            }
            if (!includeMethods) continue;
            var token = MetadataTokens.GetToken(mh);
            string? rawHash = null, normalizedHash = null;
            var instructionCount = 0;
            if (method.RelativeVirtualAddress != 0)
            {
                try
                {
                    var body = pe.GetMethodBody(method.RelativeVirtualAddress);
                    var il = body.GetILBytes() ?? Array.Empty<byte>();
                    rawHash = Convert.ToHexStringLower(SHA256.HashData(il));
                    var normalized = IlNormalizer.Normalize(md, names, body, out instructionCount);
                    normalizedHash = Convert.ToHexStringLower(SHA256.HashData(Encoding.UTF8.GetBytes(normalized)));
                }
                catch (BadImageFormatException)
                {
                    // Preserve null body hashes for malformed or unsupported bodies.
                }
            }
            methods.Add(new MethodFact(key, $"0x{token:x8}", instructionCount, rawHash, normalizedHash));
        }

        var module = md.GetModuleDefinition();
        var assembly = md.IsAssembly ? md.GetAssemblyDefinition() : default;
        return new AssemblyFacts
        {
            Path = path,
            Sha256 = Convert.ToHexStringLower(SHA256.HashData(File.ReadAllBytes(path))),
            Name = md.IsAssembly ? md.GetString(assembly.Name) : md.GetString(module.Name),
            Version = md.IsAssembly ? assembly.Version.ToString() : null,
            Mvid = md.GetGuid(module.Mvid).ToString(),
            TypeCount = md.TypeDefinitions.Count,
            MethodCount = md.MethodDefinitions.Count,
            NamespaceCounts = nsCounts,
            References = md.AssemblyReferences.Select(h => md.GetString(md.GetAssemblyReference(h).Name)).Order(StringComparer.Ordinal).ToArray(),
            PInvokes = pinvokes.OrderBy(x => x.Method, StringComparer.Ordinal).ToArray(),
            Methods = methods.OrderBy(x => x.Key, StringComparer.Ordinal).ToArray()
        };
    }
}

sealed record MethodFact(string Key, string Token, int InstructionCount, string? RawIlSha256, string? NormalizedIlSha256);
sealed record PInvokeFact(string Method, string Module, string EntryPoint, string Attributes);

sealed class MetadataNames : ISignatureTypeProvider<string, object?>
{
    readonly MetadataReader md;
    Dictionary<MethodDefinitionHandle, TypeDefinitionHandle> methodOwners = new();
    Dictionary<FieldDefinitionHandle, TypeDefinitionHandle> fieldOwners = new();
    public MetadataNames(MetadataReader md) => this.md = md;
    public void SetOwners(Dictionary<MethodDefinitionHandle, TypeDefinitionHandle> methods, Dictionary<FieldDefinitionHandle, TypeDefinitionHandle> fields)
        => (methodOwners, fieldOwners) = (methods, fields);
    public string NamespaceOf(TypeDefinitionHandle h)
    {
        var t = md.GetTypeDefinition(h);
        var ns = md.GetString(t.Namespace);
        if (ns.Length > 0) return ns;
        var parent = t.GetDeclaringType();
        return parent.IsNil ? "<global>" : NamespaceOf(parent);
    }
    public string TypeName(TypeDefinitionHandle h)
    {
        var t = md.GetTypeDefinition(h);
        var name = md.GetString(t.Name);
        var parent = t.GetDeclaringType();
        if (!parent.IsNil) return TypeName(parent) + "+" + name;
        var ns = md.GetString(t.Namespace);
        return ns.Length == 0 ? name : ns + "." + name;
    }
    public string TypeName(TypeReferenceHandle h)
    {
        var t = md.GetTypeReference(h);
        var ns = md.GetString(t.Namespace);
        return ns.Length == 0 ? md.GetString(t.Name) : ns + "." + md.GetString(t.Name);
    }
    public string MethodSignature(MethodDefinition m)
    {
        try
        {
            var s = m.DecodeSignature(this, null);
            return "(" + string.Join(",", s.ParameterTypes) + ")->" + s.ReturnType;
        }
        catch (BadImageFormatException) { return "[sig:" + Convert.ToHexString(md.GetBlobBytes(m.Signature)) + "]"; }
    }
    public string Entity(int token)
    {
        try
        {
            var h = MetadataTokens.EntityHandle(token);
            return h.Kind switch
            {
                HandleKind.TypeDefinition => TypeName((TypeDefinitionHandle)h),
                HandleKind.TypeReference => TypeName((TypeReferenceHandle)h),
                HandleKind.TypeSpecification => GetTypeFromSpecification(md, null, (TypeSpecificationHandle)h, 0),
                HandleKind.MethodDefinition => Method((MethodDefinitionHandle)h),
                HandleKind.FieldDefinition => Field((FieldDefinitionHandle)h),
                HandleKind.MemberReference => Member((MemberReferenceHandle)h),
                HandleKind.MethodSpecification => MethodSpec((MethodSpecificationHandle)h),
                HandleKind.StandaloneSignature => "standalone:" + Convert.ToHexString(md.GetBlobBytes(md.GetStandaloneSignature((StandaloneSignatureHandle)h).Signature)),
                _ => h.Kind + ":" + token.ToString("x8")
            };
        }
        catch { return "token:" + token.ToString("x8"); }
    }
    string Method(MethodDefinitionHandle h)
    {
        var m = md.GetMethodDefinition(h);
        var owner = methodOwners.TryGetValue(h, out var th) ? TypeName(th) : "?";
        return owner + "::" + md.GetString(m.Name) + MethodSignature(m);
    }
    string Field(FieldDefinitionHandle h)
    {
        var f = md.GetFieldDefinition(h);
        var owner = fieldOwners.TryGetValue(h, out var th) ? TypeName(th) : "?";
        string type;
        try { type = f.DecodeSignature(this, null); } catch { type = Convert.ToHexString(md.GetBlobBytes(f.Signature)); }
        return owner + "::" + md.GetString(f.Name) + ":" + type;
    }
    string Parent(EntityHandle h) => h.Kind switch
    {
        HandleKind.TypeDefinition => TypeName((TypeDefinitionHandle)h),
        HandleKind.TypeReference => TypeName((TypeReferenceHandle)h),
        HandleKind.TypeSpecification => GetTypeFromSpecification(md, null, (TypeSpecificationHandle)h, 0),
        HandleKind.MethodDefinition => Method((MethodDefinitionHandle)h),
        HandleKind.ModuleReference => md.GetString(md.GetModuleReference((ModuleReferenceHandle)h).Name),
        _ => h.Kind.ToString()
    };
    string Member(MemberReferenceHandle h)
    {
        var m = md.GetMemberReference(h);
        return Parent(m.Parent) + "::" + md.GetString(m.Name) + "[sig:" + Convert.ToHexString(md.GetBlobBytes(m.Signature)) + "]";
    }
    string MethodSpec(MethodSpecificationHandle h)
    {
        var m = md.GetMethodSpecification(h);
        return Entity(MetadataTokens.GetToken(m.Method)) + "<" + Convert.ToHexString(md.GetBlobBytes(m.Signature)) + ">";
    }
    static string Qualified(string ns, string name) => ns.Length == 0 ? name : ns + "." + name;
    public string GetArrayType(string elementType, ArrayShape shape) => elementType + "[" + new string(',', shape.Rank - 1) + "]";
    public string GetByReferenceType(string elementType) => elementType + "&";
    public string GetFunctionPointerType(MethodSignature<string> signature) => "fn(" + string.Join(',', signature.ParameterTypes) + ")->" + signature.ReturnType;
    public string GetGenericInstantiation(string genericType, ImmutableArray<string> typeArguments) => genericType + "<" + string.Join(',', typeArguments) + ">";
    public string GetGenericMethodParameter(object? genericContext, int index) => "!!" + index;
    public string GetGenericTypeParameter(object? genericContext, int index) => "!" + index;
    public string GetModifiedType(string modifier, string unmodifiedType, bool isRequired) => unmodifiedType + (isRequired ? " modreq(" : " modopt(") + modifier + ")";
    public string GetPinnedType(string elementType) => elementType + " pinned";
    public string GetPointerType(string elementType) => elementType + "*";
    public string GetPrimitiveType(PrimitiveTypeCode typeCode) => typeCode.ToString();
    public string GetSZArrayType(string elementType) => elementType + "[]";
    public string GetTypeFromDefinition(MetadataReader reader, TypeDefinitionHandle handle, byte rawTypeKind) => TypeName(handle);
    public string GetTypeFromReference(MetadataReader reader, TypeReferenceHandle handle, byte rawTypeKind) => TypeName(handle);
    public string GetTypeFromSpecification(MetadataReader reader, object? genericContext, TypeSpecificationHandle handle, byte rawTypeKind)
        => reader.GetTypeSpecification(handle).DecodeSignature(this, genericContext);
}

static class IlNormalizer
{
    static readonly Dictionary<ushort, OpCode> opcodes = typeof(OpCodes).GetFields(BindingFlags.Public | BindingFlags.Static)
        .Where(f => f.FieldType == typeof(OpCode))
        .Select(f => (OpCode)f.GetValue(null)!)
        .ToDictionary(o => unchecked((ushort)o.Value));

    public static string Normalize(MetadataReader md, MetadataNames names, MethodBodyBlock body, out int instructionCount)
    {
        var il = body.GetILBytes() ?? Array.Empty<byte>();
        var sb = new StringBuilder();
        var p = 0;
        instructionCount = 0;
        while (p < il.Length)
        {
            var offset = p;
            ushort value = il[p++];
            if (value == 0xfe) value = (ushort)(0xfe00 | il[p++]);
            if (!opcodes.TryGetValue(value, out var op)) throw new BadImageFormatException($"unknown opcode {value:x4}");
            sb.Append(offset.ToString("x4")).Append(':').Append(op.Name);
            switch (op.OperandType)
            {
                case OperandType.InlineNone: break;
                case OperandType.ShortInlineI: sb.Append(':').Append(unchecked((sbyte)il[p++])); break;
                case OperandType.InlineI: sb.Append(':').Append(ReadI4(il, ref p)); break;
                case OperandType.InlineI8: sb.Append(':').Append(ReadI8(il, ref p)); break;
                case OperandType.ShortInlineR: sb.Append(':').Append(BitConverter.Int32BitsToSingle(ReadI4(il, ref p)).ToString("R")); break;
                case OperandType.InlineR: sb.Append(':').Append(BitConverter.Int64BitsToDouble(ReadI8(il, ref p)).ToString("R")); break;
                case OperandType.ShortInlineVar: sb.Append(':').Append(il[p++]); break;
                case OperandType.InlineVar: sb.Append(':').Append(ReadU2(il, ref p)); break;
                case OperandType.ShortInlineBrTarget:
                    { var d = unchecked((sbyte)il[p++]); sb.Append(":IL_").Append((p + d).ToString("x4")); break; }
                case OperandType.InlineBrTarget:
                    { var d = ReadI4(il, ref p); sb.Append(":IL_").Append((p + d).ToString("x4")); break; }
                case OperandType.InlineSwitch:
                    {
                        var n = ReadI4(il, ref p); var baseOffset = p + n * 4; sb.Append(':');
                        for (var i = 0; i < n; i++) { if (i > 0) sb.Append(','); sb.Append("IL_").Append((baseOffset + ReadI4(il, ref p)).ToString("x4")); }
                        break;
                    }
                case OperandType.InlineString:
                    {
                        var token = ReadI4(il, ref p); string s;
                        try { s = md.GetUserString(MetadataTokens.UserStringHandle(token)); } catch { s = "<invalid>"; }
                        sb.Append(':').Append(JsonSerializer.Serialize(s)); break;
                    }
                case OperandType.InlineField:
                case OperandType.InlineMethod:
                case OperandType.InlineSig:
                case OperandType.InlineTok:
                case OperandType.InlineType:
                    sb.Append(':').Append(names.Entity(ReadI4(il, ref p))); break;
                default: throw new BadImageFormatException("unsupported operand " + op.OperandType);
            }
            sb.AppendLine(); instructionCount++;
        }
        foreach (var r in body.ExceptionRegions)
        {
            sb.Append("EH:").Append(r.Kind).Append(':').Append(r.TryOffset).Append(':').Append(r.TryLength)
              .Append(':').Append(r.HandlerOffset).Append(':').Append(r.HandlerLength).Append(':').Append(r.FilterOffset);
            if (!r.CatchType.IsNil) sb.Append(':').Append(names.Entity(MetadataTokens.GetToken(r.CatchType)));
            sb.AppendLine();
        }
        return sb.ToString();
    }
    static ushort ReadU2(byte[] b, ref int p) { var v = BitConverter.ToUInt16(b, p); p += 2; return v; }
    static int ReadI4(byte[] b, ref int p) { var v = BitConverter.ToInt32(b, p); p += 4; return v; }
    static long ReadI8(byte[] b, ref int p) { var v = BitConverter.ToInt64(b, p); p += 8; return v; }
}
