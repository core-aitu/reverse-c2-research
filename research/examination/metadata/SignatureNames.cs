using System.Collections.Immutable;
using System.Reflection.Metadata;

sealed class SignatureNames : ISignatureTypeProvider<string, object?>
{
    static string Qualified(string ns,string name) => ns.Length==0 ? name : ns+"."+name;
    public string GetPrimitiveType(PrimitiveTypeCode code) => code.ToString();
    public string GetTypeFromDefinition(MetadataReader md, TypeDefinitionHandle h, byte kind)
    {
        var t=md.GetTypeDefinition(h);
        return Qualified(md.GetString(t.Namespace),md.GetString(t.Name));
    }
    public string GetTypeFromReference(MetadataReader md, TypeReferenceHandle h, byte kind)
    {
        var t=md.GetTypeReference(h);
        return Qualified(md.GetString(t.Namespace),md.GetString(t.Name));
    }
    public string GetTypeFromSpecification(MetadataReader md, object? context, TypeSpecificationHandle h, byte kind) =>
        md.GetTypeSpecification(h).DecodeSignature(this,context);
    public string GetArrayType(string element,ArrayShape shape) => element+"["+new string(',',shape.Rank-1)+"]";
    public string GetSZArrayType(string element) => element+"[]";
    public string GetByReferenceType(string element) => element+"&";
    public string GetGenericInstantiation(string type,ImmutableArray<string> args) => type+"<"+string.Join(", ",args)+">";
    public string GetGenericMethodParameter(object? context,int index) => "!!"+index;
    public string GetGenericTypeParameter(object? context,int index) => "!"+index;
    public string GetModifiedType(string modifier,string raw,bool required) => raw+(required?" modreq(":" modopt(")+modifier+")";
    public string GetPinnedType(string element) => element+" pinned";
    public string GetPointerType(string element) => element+"*";
    public string GetFunctionPointerType(MethodSignature<string> sig) => "fn("+string.Join(", ",sig.ParameterTypes)+")->"+sig.ReturnType;
}
