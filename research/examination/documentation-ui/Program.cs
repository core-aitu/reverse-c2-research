using System.Reflection.Metadata;
using System.Reflection.Metadata.Ecma335;
using System.Reflection.PortableExecutable;
using System.Security.Cryptography;
using System.Text.Json;

using var file = File.OpenRead(args.Single());
string imageDigest = Convert.ToHexStringLower(SHA256.HashData(file));
file.Position = 0;
using var pe = new PEReader(file);
var metadata = pe.GetMetadataReader();
var methods = new List<object>();
foreach (int rid in new[] { 0x914, 0x915, 0x91c })
{
    var handle = MetadataTokens.MethodDefinitionHandle(rid);
    var definition = metadata.GetMethodDefinition(handle);
    var owner = metadata.GetTypeDefinition(definition.GetDeclaringType());
    var parent = owner.GetDeclaringType();
    var body = pe.GetMethodBody(definition.RelativeVirtualAddress);
    var code = body.GetILBytes() ?? throw new BadImageFormatException();
    methods.Add(new {
        token = $"0x{MetadataTokens.GetToken(handle):x8}",
        name = metadata.GetString(definition.Name),
        declaring_type = metadata.GetString(owner.Name),
        parent_type = parent.IsNil ? null : metadata.GetString(metadata.GetTypeDefinition(parent).Name),
        il_sha256 = Convert.ToHexStringLower(SHA256.HashData(code)),
        il_size = code.Length,
        exception_regions = body.ExceptionRegions.Length
    });
}
var setter = metadata.GetMethodDefinition(MetadataTokens.MethodDefinitionHandle(0x915));
var il = pe.GetMethodBody(setter.RelativeVirtualAddress).GetILBytes()!;
if (il[8] != 0x72) throw new BadImageFormatException("Expected ldstr at setter IL+8");
var token = System.Buffers.Binary.BinaryPrimitives.ReadInt32LittleEndian(il.AsSpan(9));
if ((uint)token >> 24 != 0x70) throw new BadImageFormatException("Expected #US token");
var result = new {
    assembly_sha256 = imageDigest,
    mvid = metadata.GetGuid(metadata.GetModuleDefinition().Mvid),
    methods,
    setter_property_argument = metadata.GetUserString(MetadataTokens.UserStringHandle(token & 0xffffff))
};
Console.WriteLine(JsonSerializer.Serialize(result, new JsonSerializerOptions { WriteIndented = true }));
