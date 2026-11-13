using System.IO.Compression;
using System.Reflection;
using System.Reflection.Emit;
using System.Reflection.Metadata;
using System.Reflection.PortableExecutable;
using System.Security.Cryptography;
using System.Text.Json;

var root=Path.GetFullPath(args[0]);
var research=Path.GetFullPath(args[1]);
var metadata=File.ReadLines(Path.Combine(research,"analysis/assemblies.jsonl"))
    .Select(s=>JsonSerializer.Deserialize<AssemblyRow>(s)!).ToArray();
using var zip=new GZipStream(File.OpenRead(Path.Combine(research,"evidence/manifest.jsonl.gz")),CompressionMode.Decompress);
using var manifest=new StreamReader(zip);
var pdbFiles=new List<string>();
var nativeTemplates=new List<string>();
var sources=new Dictionary<string,List<string>>(StringComparer.OrdinalIgnoreCase);
string? line;
while ((line=manifest.ReadLine())!=null)
{
    using var json=JsonDocument.Parse(line);
    var item=json.RootElement;
    if(item.GetProperty("kind").GetString()!="file") continue;
    var path=item.GetProperty("path").GetString()!;
    if(path.EndsWith(".pdb",StringComparison.OrdinalIgnoreCase)) pdbFiles.Add(path);
    if(path.StartsWith("templates/") && item.GetProperty("format").GetString()=="PE") nativeTemplates.Add(path);
    if(path.StartsWith("templates/source/") && path.EndsWith(".cs",StringComparison.OrdinalIgnoreCase))
    {
        var key=Path.GetFileName(path);
        if(!sources.TryGetValue(key,out var hits)) sources[key]=hits=[];
        hits.Add(path);
    }
}
var symbols=new Dictionary<BlobContentId,List<PdbRow>>();
foreach(var path in pdbFiles)
{
    using var stream=File.OpenRead(Path.Combine(root,path));
    Span<byte> magic=new byte[4];
    if(stream.Read(magic)!=4 || !magic.SequenceEqual("BSJB"u8)) continue;
    stream.Position=0;
    using var provider=MetadataReaderProvider.FromPortablePdbStream(stream);
    var md=provider.GetMetadataReader();
    var header=md.DebugMetadataHeader!;
    var id=new BlobContentId(header.Id);
    if(!symbols.TryGetValue(id,out var hits)) symbols[id]=hits=[];
    hits.Add(new PdbRow(path,md.Documents.Count,md.MethodDebugInformation.Count,header.IdStartOffset));
}
var opcodeMap=typeof(OpCodes).GetFields(BindingFlags.Static|BindingFlags.Public)
    .Where(f=>f.FieldType==typeof(OpCode)).Select(f=>(OpCode)f.GetValue(null)!)
    .Where(o=>o.OpCodeType!=OpCodeType.Nternal).ToDictionary(o=>(ushort)o.Value);
var structure=new List<object>();
var debug=new List<object>();
foreach(var group in metadata.GroupBy(a=>a.hash))
{
    var a=group.First();
    using var stream=File.OpenRead(Path.Combine(root,a.path));
    var actual=Convert.ToHexStringLower(SHA256.HashData(stream));
    if(actual!=a.hash) throw new InvalidDataException(a.path);
    stream.Position=0;
    using var pe=new PEReader(stream);
    var md=pe.GetMetadataReader();
    int noRva=0,otherCode=0,valid=0,exceptionBodies=0;
    var accessors=new Dictionary<MethodDefinitionHandle,bool>();
    foreach(var property in md.PropertyDefinitions)
    {
        var access=md.GetPropertyDefinition(property).GetAccessors();
        if(!access.Getter.IsNil) accessors[access.Getter]=true;
        if(!access.Setter.IsNil) accessors[access.Setter]=false;
    }
    int exactGetters=0,exactSetters=0;
    long instructions=0,ilBytes=0;
    var failures=new Dictionary<string,int>();
    foreach(var handle in md.MethodDefinitions)
    {
        var method=md.GetMethodDefinition(handle);
        if(method.RelativeVirtualAddress==0){noRva++;continue;}
        if((method.ImplAttributes & MethodImplAttributes.CodeTypeMask)!=MethodImplAttributes.IL){otherCode++;continue;}
        try
        {
            var body=pe.GetMethodBody(method.RelativeVirtualAddress);
            var bytes=body.GetILBytes()!;
            var count=Decode(bytes,body,opcodeMap);
            if(accessors.TryGetValue(handle,out var getter) && IsDirectAccessor(bytes,md,method,getter))
            {
                if(getter) exactGetters++; else exactSetters++;
            }
            instructions+=count;ilBytes+=bytes.Length;valid++;
            if(body.ExceptionRegions.Length>0) exceptionBodies++;
        }
        catch(Exception e) when(e is BadImageFormatException or OverflowException or ArgumentOutOfRangeException)
        {
            var reason=e is BadImageFormatException ? e.Message : e.GetType().Name;
            failures[reason]=failures.GetValueOrDefault(reason)+1;
        }
    }
    structure.Add(new {a.hash,paths=group.Select(x=>x.path),selected=group.Any(x=>x.own_candidate),a.methods,
        no_rva=noRva,other_code=otherCode,validated=valid,failures,instructions,il_bytes=ilBytes,with_exception_regions=exceptionBodies,property_accessors=accessors.Count,direct_field_getters=exactGetters,direct_field_setters=exactSetters});
    if(!group.Any(x=>x.own_candidate)) continue;
    var entries=pe.ReadDebugDirectory();
    var expected=entries.Where(d=>d.Type==DebugDirectoryEntryType.PdbChecksum).Select(pe.ReadPdbChecksumDebugDirectoryData).ToArray();
    foreach(var de in entries.Where(d=>d.Type==DebugDirectoryEntryType.CodeView))
    {
        var cv=pe.ReadCodeViewDebugDirectoryData(de);
        var id=new BlobContentId(cv.Guid,de.Stamp);
        var matches=new List<object>();
        foreach(var pdb in symbols.GetValueOrDefault(id)??[])
        {
            var bytes=File.ReadAllBytes(Path.Combine(root,pdb.Path));
            var fileHash=Convert.ToHexStringLower(SHA256.HashData(bytes));
            using var provider=MetadataReaderProvider.FromPortablePdbStream(new MemoryStream(bytes));
            var pdbmd=provider.GetMetadataReader();
            int sourceNames=0,sourceExact=0;
            foreach(var h in pdbmd.Documents)
            {
                var doc=pdbmd.GetDocument(h);
                var leaf=Path.GetFileName(pdbmd.GetString(doc.Name).Replace('\\','/'));
                if(!sources.TryGetValue(leaf,out var candidates)) continue;
                sourceNames++;
                var algo=pdbmd.GetGuid(doc.HashAlgorithm);
                bool sha256=algo==Guid.Parse("8829d00f-11b8-4213-878b-770e8597ac16");
                bool sha1=algo==Guid.Parse("ff1816ec-aa5e-4d10-87f7-6f4963833460");
                if(!sha256 && !sha1) continue;
                var want=pdbmd.GetBlobBytes(doc.Hash);
                if(candidates.Any(p=>{
                    using var f=File.OpenRead(Path.Combine(root,p));
                    return (sha256?SHA256.HashData(f):SHA1.HashData(f)).SequenceEqual(want);
                })) sourceExact++;
            }
            Array.Clear(bytes,pdb.IdOffset,20);
            var checks=expected.Select(x=>{
                using var alg=IncrementalHash.CreateHash(new HashAlgorithmName(x.AlgorithmName));
                alg.AppendData(bytes);
                return new {algorithm=x.AlgorithmName,matches=alg.GetHashAndReset().SequenceEqual(x.Checksum)};
            }).ToArray();
            matches.Add(new {pdb.Path,fileHash,pdb.Documents,pdb.MethodRows,checks,source_leaf_matches=sourceNames,
                source_content_matches=sourceExact});
        }
        debug.Add(new {a.hash,paths=group.Select(x=>x.path),guid=cv.Guid,stamp=de.Stamp,cv.Age,portable=de.IsPortableCodeView,matches});
    }
}
var native=nativeTemplates.Select(path=>{
    using var f=File.OpenRead(Path.Combine(root,path));
    var sha=Convert.ToHexStringLower(SHA256.HashData(f));f.Position=0;
    using var pe=new PEReader(f);
    return new {path,sha,machine=pe.PEHeaders.CoffHeader.Machine.ToString(),magic=pe.PEHeaders.PEHeader!.Magic.ToString(),
        sections=pe.PEHeaders.SectionHeaders.Length,subsystem=pe.PEHeaders.PEHeader.Subsystem.ToString(),
        is_dll=pe.PEHeaders.IsDll,clr_header=pe.PEHeaders.CorHeader!=null};
}).ToArray();
File.WriteAllLines(Path.Combine(research,"analysis/il-structure.jsonl"),structure.Select(x=>JsonSerializer.Serialize(x)));
File.WriteAllText(Path.Combine(research,"evidence/debug-bindings.json"),JsonSerializer.Serialize(new {pdb_files=pdbFiles.Count,
    portable_files=symbols.Values.Sum(g=>g.Count),bindings=debug,native_templates=native},new JsonSerializerOptions{WriteIndented=true})+"\n");

// Match the entire unprefixed body of an accessor from the Property semantics table.
static bool IsDirectAccessor(byte[] il, MetadataReader md, MethodDefinition def, bool getter)
{
    bool stat=(def.Attributes & MethodAttributes.Static)!=0;
    byte[] prefix=(stat,getter) switch {
        (false,true)=>[0x02,0x7b], (false,false)=>[0x02,0x03,0x7d],
        (true,true)=>[0x7e], (true,false)=>[0x02,0x80]};
    if(il.Length!=prefix.Length+5 || !il.AsSpan().StartsWith(prefix) || il[^1]!=0x2a) return false;
    var token=System.Buffers.Binary.BinaryPrimitives.ReadInt32LittleEndian(il.AsSpan(prefix.Length,4));
    if((token>>24)!=0x04) return false;
    var index=token & 0xffffff;
    if(index==0 || index>md.FieldDefinitions.Count) return false;
    var field=md.GetFieldDefinition(System.Reflection.Metadata.Ecma335.MetadataTokens.FieldDefinitionHandle(index));
    return field.GetDeclaringType()==def.GetDeclaringType() && ((field.Attributes & FieldAttributes.Static)!=0)==stat;
}

static int Decode(byte[] bytes,MethodBodyBlock body,Dictionary<ushort,OpCode> opcodes)
{
    var boundaries=new HashSet<int>();
    var targets=new List<int>();
    int at=0;
    void Require(int n)
    {
        if(n<0 || n>bytes.Length-at) throw new BadImageFormatException("operand span");
    }
    int Int32At(int p)=>System.Buffers.Binary.BinaryPrimitives.ReadInt32LittleEndian(bytes.AsSpan(p,4));
    while(at<bytes.Length)
    {
        boundaries.Add(at);
        ushort key=bytes[at++];
        if(key==0xfe){Require(1);key=(ushort)(0xfe00|bytes[at++]);}
        if(!opcodes.TryGetValue(key,out var op)) throw new BadImageFormatException("unknown opcode");
        int size=op.OperandType switch {
            OperandType.InlineNone=>0,
            OperandType.ShortInlineI or OperandType.ShortInlineVar or OperandType.ShortInlineBrTarget=>1,
            OperandType.InlineVar=>2,
            OperandType.InlineI8 or OperandType.InlineR=>8,
            OperandType.InlineSwitch=>-1,
            OperandType.InlineI or OperandType.ShortInlineR or OperandType.InlineString or OperandType.InlineSig or
            OperandType.InlineMethod or OperandType.InlineField or OperandType.InlineTok or OperandType.InlineType or OperandType.InlineBrTarget=>4,
            _=>throw new BadImageFormatException("unknown operand type")};
        if(size==-1)
        {
            Require(4);
            int n=Int32At(at);at+=4;
            if(n<0) throw new BadImageFormatException("negative switch count");
            size=checked(n*4);Require(size);
            for(int i=0;i<n;i++) targets.Add(checked(at+size+Int32At(at+i*4)));
        }
        else
        {
            Require(size);
            if(op.OperandType==OperandType.ShortInlineBrTarget) targets.Add(at+size+(sbyte)bytes[at]);
            if(op.OperandType==OperandType.InlineBrTarget) targets.Add(checked(at+size+Int32At(at)));
        }
        at+=size;
    }
    if(targets.Any(t=>!boundaries.Contains(t))) throw new BadImageFormatException("branch target boundary");
    bool End(int n)=>n==bytes.Length || boundaries.Contains(n);
    foreach(var eh in body.ExceptionRegions)
    {
        if(!boundaries.Contains(eh.TryOffset) || !End(checked(eh.TryOffset+eh.TryLength))
            || !boundaries.Contains(eh.HandlerOffset) || !End(checked(eh.HandlerOffset+eh.HandlerLength))
            || (eh.Kind==ExceptionRegionKind.Filter && !boundaries.Contains(eh.FilterOffset)))
            throw new BadImageFormatException("exception region boundary");
    }
    return boundaries.Count;
}
record AssemblyRow(string path,string hash,int methods,bool own_candidate);
record PdbRow(string Path,int Documents,int MethodRows,int IdOffset);
