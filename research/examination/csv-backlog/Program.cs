using System.Reflection;
using System.Text.Json;
using Microsoft.VisualBasic.FileIO;

var assembly = Assembly.LoadFrom("/app-data/SpecterInsight.Server.dll");
var type = assembly.GetType("SpecterInsight.Server.Services.DataAugmentation.MacVendorDataTransformService", true)!;
const BindingFlags flags = BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic;
var constructor = type.GetConstructors(flags).Single(c => c.GetParameters()[0].ParameterType.IsGenericType);
var service = constructor.Invoke([new Dictionary<string,string>()]);
var load = type.GetMethod("LoadVendors", flags)!;
var actual = (IDictionary<string,string>)load.Invoke(service, ["/app-data/vendors.csv"])!;
using var csv = new TextFieldParser("/app-data/vendors.csv");
csv.TextFieldType = FieldType.Delimited;
csv.SetDelimiters(",");
csv.HasFieldsEnclosedInQuotes = true;
var header = csv.ReadFields()!;
int key = Array.IndexOf(header, "Mac Prefix"), name = Array.IndexOf(header, "Vendor Name");
int matched = 0, commas = 0, missing = 0;
var distinct = new HashSet<string>();
var other = new List<object>();
while (!csv.EndOfData)
{
    var row = csv.ReadFields()!;
    var prefix = row[key].Replace(":", "").ToUpperInvariant();
    if (prefix.Length != 6 || !distinct.Add(prefix)) continue;
    var expected = row[name].Trim();
    if (!actual.TryGetValue(prefix, out var read)) missing++;
    else if (read == expected) matched++;
    else if (expected.Contains(',') && read == expected.Split(',', 2)[0].Trim()) commas++;
    else other.Add(new { prefix, expected, actual = read });
}
Console.WriteLine(JsonSerializer.Serialize(new { distinct = distinct.Count, matched, commas, missing, other }));
