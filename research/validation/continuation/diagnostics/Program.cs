using System.Reflection;
using System.Reflection.Metadata;
using System.Reflection.Metadata.Ecma335;
using System.Reflection.PortableExecutable;
using System.Runtime.Loader;
using System.Security.Cryptography;
using System.Text.Json;

var bytes=File.ReadAllBytes("/sample.exe");
var hash=Convert.ToHexStringLower(SHA256.HashData(bytes));
if(hash!=args.Single()) throw new InvalidDataException("Unknown specimen");
using var pe=new PEReader(new MemoryStream(bytes));
var md=pe.GetMetadataReader();
foreach(var h in md.TypeDefinitions)
{
    var def=md.GetTypeDefinition(h);
    var name=md.GetString(def.Name);
    if(name is not ("<Module>" or "LogEntry")) continue;
    if(def.GetMethods().Any(m=>md.GetString(md.GetMethodDefinition(m).Name)==".cctor"))
        throw new InvalidDataException("Unexpected initializer");
}
if(File.Exists("/symbols.pdb"))
{
    using var provider=MetadataReaderProvider.FromPortablePdbStream(File.OpenRead("/symbols.pdb"));
    var pm=provider.GetMetadataReader();
    var debug=pe.ReadDebugDirectory().Single(d=>d.Type==DebugDirectoryEntryType.CodeView);
    var expected=new BlobContentId(pe.ReadCodeViewDebugDirectoryData(debug).Guid,debug.Stamp);
    if(new BlobContentId(pm.DebugMetadataHeader!.Id)!=expected)throw new InvalidDataException("PDB identity");
    var typeHandle=md.TypeDefinitions.Single(h=>md.GetString(md.GetTypeDefinition(h).Name)=="LogEntry");
    foreach(var h in md.GetTypeDefinition(typeHandle).GetMethods())
    {
        var rec=pm.GetMethodDebugInformation(h);
        var point=rec.GetSequencePoints().FirstOrDefault(p=>!p.IsHidden);
        if(point.StartLine<=0)continue;
        var doc=pm.GetDocument(point.Document.IsNil?rec.Document:point.Document);
        var path=pm.GetString(doc.Name).Replace('\\','/');
        if(!path.EndsWith("/Logging/LogEntry.cs",StringComparison.OrdinalIgnoreCase))continue;
        using var file=File.OpenRead("/LogEntry.cs");
        var algorithm=pm.GetGuid(doc.HashAlgorithm);
        var digest=algorithm==Guid.Parse("8829d00f-11b8-4213-878b-770e8597ac16")?SHA256.HashData(file):SHA1.HashData(file);
        if(!digest.SequenceEqual(pm.GetBlobBytes(doc.Hash)))throw new InvalidDataException("Source checksum");
        Console.WriteLine(JsonSerializer.Serialize(new{sha256=hash,id="sequence-point",token=MetadataTokens.GetToken(h),il=point.Offset,line=point.StartLine,source="templates/source/SpecterInsight/Logging/LogEntry.cs",passed=true}));
    }
}
var assembly=new AssemblyLoadContext("diagnostic").LoadFromStream(new MemoryStream(bytes));
var type=assembly.GetType("SpecterInsight.Logging.LogEntry",true)!;
var at=new DateTime(2020,5,6,7,8,9,DateTimeKind.Utc);
object? Get(object obj,string field)=>type.GetProperty(field)!.GetValue(obj);
void Check(string id,bool passed)
{
    Console.WriteLine(JsonSerializer.Serialize(new{sha256=hash,id,passed}));
    if(!passed) Environment.ExitCode=1;
}
var empty=Activator.CreateInstance(type)!;
Check("default",Get(empty,"Message")==null && Get(empty,"System")==null && Equals(Get(empty,"Count"),1) && Equals(Get(empty,"TimeStamp"),default(DateTime)));
var ctor=type.GetConstructor([typeof(string),typeof(string),typeof(DateTime)])!;
foreach(var message in new string?[]{null,"","fixture text"})
{
    var record=ctor.Invoke(["fixture",message,at]);
    Check("explicit-"+(message==null?"null":message.Length==0?"empty":"text"),
        Equals(Get(record,"Message"),message) && Equals(Get(record,"System"),"fixture") && Equals(Get(record,"TimeStamp"),at) && Equals(Get(record,"Count"),1));
}
type.GetProperty("Count")!.SetValue(empty,-1);type.GetProperty("Message")!.SetValue(empty,"changed");
Check("setter-roundtrip",Equals(Get(empty,"Count"),-1) && Equals(Get(empty,"Message"),"changed"));
