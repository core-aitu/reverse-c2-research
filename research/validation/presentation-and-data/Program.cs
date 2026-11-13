using System.Globalization;
using System.Reflection;
using System.Runtime.Loader;
using System.Text.Json;
using Microsoft.VisualBasic.FileIO;

const BindingFlags all=BindingFlags.Public|BindingFlags.NonPublic|BindingFlags.Instance|BindingFlags.Static;
var ctx=new FixtureContext();
var server=ctx.LoadFromAssemblyPath("/files/SpecterInsight.Server.dll");
var ui=ctx.LoadFromAssemblyPath("/files/SpecterInsight.UI.dll");
var api=ctx.LoadFromAssemblyPath("/files/SpecterInsight.ApiClient.dll");
var vendor=server.GetType("SpecterInsight.Server.Services.DataAugmentation.MacVendorDataTransformService",true)!;
var ctor=vendor.GetConstructors(all).Single(c=>!c.IsStatic && c.GetParameters().Length==1 && c.GetParameters()[0].ParameterType.IsGenericType);
var service=ctor.Invoke([new Dictionary<string,string>(StringComparer.OrdinalIgnoreCase){{"AABBCC","Example Vendor"}}]);
var lookup=vendor.GetMethod("TryLookupVendor",all)!;
var load=vendor.GetMethod("LoadVendors",all)!;
object? GetVendor(string? input)
{
    object?[] args=[input,null];
    var ok=(bool)lookup.Invoke(service,args)!;
    return ok ? args[1] : null;
}
foreach(var (name,input,expected) in new (string,string?,string?)[]{
    ("colon","aa:bb:cc:11:22:33","Example Vendor"),("compact","AABBCC112233","Example Vendor"),
    ("unknown","001122334455",null),("empty","",null),("null",null,null)})
    Test("vendor-"+name,GetVendor(input),expected);

var actual=(IDictionary<string,string>)load.Invoke(service,["/vendors.csv"])!;
var missing=(IDictionary<string,string>)load.Invoke(service,["/tmp/absent.csv"])!;
Test("vendor-missing-file",missing.Count,0);
File.WriteAllText("/tmp/quoted.csv","Mac Prefix,Vendor Name,Private\nAA:BB:01,Plain Vendor,false\nAA:BB:02,\"Example, Incorporated\",false\nAA:BB:01,Second Vendor,false\nAA:BB:03,,false\n");
var sample=(IDictionary<string,string>)load.Invoke(service,["/tmp/quoted.csv"])!;
Test("vendor-plain",sample["AABB01"],"Plain Vendor");
Test("vendor-quoted-comma",sample["AABB02"],"Example");
Test("vendor-empty-organization",sample.ContainsKey("AABB03"),false);
using var csv=new TextFieldParser("/vendors.csv");
csv.TextFieldType=FieldType.Delimited;
csv.SetDelimiters(",");
csv.HasFieldsEnclosedInQuotes=true;
var header=csv.ReadFields()!;
int prefix=Array.IndexOf(header,"Mac Prefix"), organization=Array.IndexOf(header,"Vendor Name");
var expectedRows=new Dictionary<string,string>(StringComparer.OrdinalIgnoreCase);
int shortRows=0,longRows=0;
while(!csv.EndOfData)
{
    var cols=csv.ReadFields()!;
    var id=cols[prefix].Replace(":","").ToUpperInvariant();
    if(id.Length!=6){longRows++;continue;}
    shortRows++;
    expectedRows.TryAdd(id,cols[organization].Trim());
}
int exact=0,missingPrefix=0,different=0,commaPrefix=0,emptyMissing=0;
foreach(var (id,company) in expectedRows)
{
    if(!actual.TryGetValue(id,out var read)){missingPrefix++;if(company.Length==0)emptyMissing++;}
    else if(read==company) exact++;
    else {different++;if(company.Contains(',') && read==company.Split(',',2)[0].Trim())commaPrefix++;}
}
Console.WriteLine(JsonSerializer.Serialize(new {scenario="catalog-comparison",shortRows,longRows,unique_short=expectedRows.Count,
    product_keys=actual.Count,exact,missingPrefix,different,commaPrefix,emptyMissing,independent_parser="TextFieldParser"}));
if(exact+missingPrefix+different!=expectedRows.Count || exact==0) Environment.ExitCode=1;

object? ConvertValue(string cls,object? input,string method="Convert")
{
    var type=ui.GetType("SpecterInsight.UI.Converters."+cls,true)!;
    var converter=Activator.CreateInstance(type);
    return type.GetMethod(method)!.Invoke(converter,[input,typeof(object),null,CultureInfo.InvariantCulture]);
}
var outcome=api.GetType("SpecterInsight.ApiClient.EventOutcome",true)!;
foreach(var (value,fg,bg) in new[]{("Success","#52c27a","#132a1e"),("Failure","#e05c5c","#2a1313")})
{
    var input=Enum.Parse(outcome,value);
    Test("outcome-foreground-"+value,Color("EventOutcomeToForegroundConverter",input),fg);
    Test("outcome-background-"+value,Color("EventOutcomeToBadgeBackgroundConverter",input),bg);
}
Test("outcome-unknown",Color("EventOutcomeToForegroundConverter",null),"#c8d0d0");
foreach(var (input,accent,row) in new[]{("user-example","#9e68c8","#241533"),("other","#6a8080","#1a2424")})
{
    Test("event-accent-"+input,Color("EventTypeToAccentConverter",input),accent);
    Test("event-row-"+input,Color("EventTypeToRowBackgroundConverter",input),row);
}
foreach(var cls in new[]{"BoolToGridLengthConverter","BoolToBrushConverter"})
{
    var type=ui.GetType("SpecterInsight.UI.Converters."+cls,true)!;
    var obj=Activator.CreateInstance(type)!;
    var isGrid=cls.Contains("Grid");
    var yes=type.GetProperty(isGrid ? "TrueLength":"TrueBrush")!;
    var no=type.GetProperty(isGrid ? "FalseLength":"FalseBrush")!;
    if(isGrid)
    {
        yes.SetValue(obj,Activator.CreateInstance(yes.PropertyType,new object[]{12.0}));
        no.SetValue(obj,Activator.CreateInstance(no.PropertyType,new object[]{33.0}));
    }
    else
    {
        yes.SetValue(obj,ConvertValue("EventOutcomeToForegroundConverter",Enum.Parse(outcome,"Success")));
        no.SetValue(obj,ConvertValue("EventOutcomeToForegroundConverter",Enum.Parse(outcome,"Failure")));
    }
    foreach(var input in new object?[]{true,false,"true",null})
    {
        var converted=type.GetMethod("Convert")!.Invoke(obj,[input,typeof(object),null,CultureInfo.InvariantCulture]);
        var value=input is true ? yes.GetValue(obj) : no.GetValue(obj);
        Test(cls+"-"+(input?.GetType().Name??"null")+"-"+(input?.ToString()??""),Equals(converted,value),true);
    }
}
foreach(var name in new[]{"BoolToGridLengthConverter","BoolToBrushConverter","EventOutcomeToForegroundConverter","EventOutcomeToBadgeBackgroundConverter","EventTypeToAccentConverter","EventTypeToRowBackgroundConverter"})
{
    try {ConvertValue(name,null,"ConvertBack");Test(name+"-reverse","returned","System.NotSupportedException");}
    catch(TargetInvocationException e) {var error=e.InnerException!.GetType().FullName;
        Console.WriteLine(JsonSerializer.Serialize(new{scenario=name+"-reverse",exception=error}));
        if(error!="System.NotSupportedException") Environment.ExitCode=1;}
}
var logType=ui.GetType("SpecterInsight.UI.ViewModels.EventLogViewModel",true)!;
var view=Activator.CreateInstance(logType)!;
var match=logType.GetMethod("IsMatch",all)!;
var et=api.GetType("SpecterInsight.ApiClient.Event",true)!;
using var logLine=JsonDocument.Parse(File.ReadLines("/storage.jsonl").Single(line=>line.Contains("\"scenario\":\"audit-row\"")));
var audit=logLine.RootElement.GetProperty("record").Deserialize(et,new JsonSerializerOptions(JsonSerializerDefaults.Web))!;
Test("stored-event-client-id",et.GetProperty("Id")!.GetValue(audit),"fixture-2");
Test("stored-event-client-brush",Color("EventOutcomeToForegroundConverter",et.GetProperty("Outcome")!.GetValue(audit)),"#e05c5c");
var min=(DateTime)logType.GetProperty("AppliedMinimumTimestamp")!.GetValue(view)!;
void ProbeFilter(string label,bool recent,string selected,string pattern,bool expected)
{
    // Exercise both sides of the rolling cutoff independent of the date the probe runs.
    et.GetProperty("Timestamp")!.SetValue(audit,new DateTimeOffset(min.ToUniversalTime().AddHours(recent?2:-2)));
    var types=new HashSet<string>{selected};
    var rx=new System.Text.RegularExpressions.Regex(pattern,System.Text.RegularExpressions.RegexOptions.CultureInvariant);
    var value=match.Invoke(view,[audit,types,min,rx]);
    Test("audit-filter-"+label,value,expected);
}
ProbeFilter("message",true,"audit-second","fixture message",true);
ProbeFilter("source",true,"audit-second","^fixture-source$",true);
ProbeFilter("details",true,"audit-second","^\\{\\}$",true);
ProbeFilter("different-type",true,"audit-first","fixture",false);
ProbeFilter("no-text-match",true,"audit-second","absent",false);
ProbeFilter("outside-time-window",false,"audit-second","fixture",false);
string? Color(string cls,object? input)
{
    var brush=ConvertValue(cls,input)!;
    var value=brush.GetType().GetProperty("Color")!.GetValue(brush)!;
    var type=value.GetType();
    return "#"+string.Concat(new[]{"R","G","B"}.Select(c=>((byte)type.GetProperty(c)!.GetValue(value)!).ToString("x2")));
}
void Test(string name,object? actual,object? expected)
{
    bool passed=Equals(actual,expected);
    Console.WriteLine(JsonSerializer.Serialize(new{scenario=name,actual,expected,passed}));
    if(!passed) Environment.ExitCode=1;
}
sealed class FixtureContext : AssemblyLoadContext
{
    protected override Assembly? Load(AssemblyName n)
    {
        var path=Path.Combine("/files",n.Name+".dll");return File.Exists(path)?LoadFromAssemblyPath(path):null;
    }
}
