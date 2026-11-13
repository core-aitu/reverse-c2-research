using System.Net;
using System.Reflection;
using System.Runtime.Loader;
using System.Text.Json;

// Run only inside the offline namespace. The product executables are never started.
var server = new FixtureContext("/server");
var client = new FixtureContext("/client");
var serverAssembly = server.LoadFromAssemblyPath("/server/SpecterInsight.Server.dll");
var clientAssembly = client.LoadFromAssemblyPath("/client/SpecterInsight.ApiClient.dll");
var controller = serverAssembly.GetType("SpecterInsight.Server.Controllers.DocumentationController", true)!;
var load = controller.GetMethod("Load", BindingFlags.NonPublic | BindingFlags.Public | BindingFlags.Static)!;
var fixture = "/tmp/docs";
Directory.CreateDirectory(fixture+"/Guide");
File.WriteAllText(fixture+"/Guide/Overview.md", "Folder summary");
File.WriteAllText(fixture+"/Guide/Sample.md", "# Synthetic fixture\n");
File.WriteAllText(fixture+"/Guide/ignore.tmp", "Excluded control");
var task = (Task)load.Invoke(null, new object[]{fixture})!;
await task;
var tree = task.GetType().GetProperty("Result")!.GetValue(task);
var json = JsonSerializer.Serialize(tree, new JsonSerializerOptions(JsonSerializerDefaults.Web));
using (var document=JsonDocument.Parse(json))
{
    var folder=document.RootElement.EnumerateArray().Single();
    var children=folder.GetProperty("children").EnumerateArray().ToDictionary(n=>n.GetProperty("name").GetString()!);
    bool passed=folder.GetProperty("name").GetString()=="Guide" &&
        folder.GetProperty("contents").GetString()=="Folder summary" &&
        children.Count==2 && children.ContainsKey("Sample") &&
        children["Overview"].GetProperty("contents").GetString()=="Folder summary";
    Console.WriteLine(JsonSerializer.Serialize(new {scenario="server-folder-overview",passed}));
    if (!passed) Environment.ExitCode=1;
}
var missingTask=(Task)load.Invoke(null,new object[]{"/tmp/absent"})!;
await missingTask;
var missing=missingTask.GetType().GetProperty("Result")!.GetValue(missingTask);
bool missingPassed=JsonSerializer.Serialize(missing)=="[]";
Console.WriteLine(JsonSerializer.Serialize(new {scenario="server-missing-directory",passed=missingPassed}));
if (!missingPassed) Environment.ExitCode=1;
var api = clientAssembly.GetType("SpecterInsight.ApiClient.SpecterInsightClient", true)!;
var handler = new FixtureHandler(json);
using var http = new HttpClient(handler){BaseAddress = new Uri("https://fixture.invalid/")};
var ctor = api.GetConstructors(BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Instance).First(c => c.GetParameters().All(p => p.ParameterType == typeof(string) || p.ParameterType == typeof(HttpClient))
    && c.GetParameters().Any(p => p.ParameterType == typeof(HttpClient)));
var instance = ctor.Invoke(ctor.GetParameters().Select(p => p.ParameterType == typeof(string) ? (object)"https://fixture.invalid/" : http).ToArray());
var method = api.GetMethods().Single(m => m.Name == "DocumentationAsync" && m.GetParameters().Length==0);
object? decodedTree=null;
async Task<string> ReadResponse()
{
    var request = (Task)method.Invoke(instance, null)!;
    await request;
    var decoded=request.GetType().GetProperty("Result")!.GetValue(request);
    decodedTree ??= decoded;
    return JsonSerializer.Serialize(decoded, new JsonSerializerOptions(JsonSerializerDefaults.Web));
}
foreach (var scenario in new[]{"tree","empty","null","error"})
{
    handler.Body = scenario switch {"empty"=>"[]","null"=>"null",_=>json};
    handler.Status = scenario=="error" ? HttpStatusCode.ServiceUnavailable : HttpStatusCode.OK;
    string? value=null, error=null;
    try { value=await ReadResponse(); }
    catch (Exception e) { error=e.GetBaseException().GetType().FullName; }
    bool parsed=scenario switch {
        "tree"=>value==json && value.Contains("Synthetic fixture") && value.Contains("Sample") && !value.Contains("ignore.tmp"),
        "empty"=>value=="[]", _=>error=="SpecterInsight.ApiClient.ApiException"};
    bool passed=parsed && handler.Verb=="GET" && handler.Path=="/Documentation";
    Console.WriteLine(JsonSerializer.Serialize(new { scenario, passed, handler.Verb, handler.Path, status=(int)handler.Status, value, error }));
    if (!passed) Environment.ExitCode=1;
}

var ui = client.LoadFromAssemblyPath("/client/SpecterInsight.UI.dll");
var viewType = ui.GetType("SpecterInsight.UI.ViewModels.Controls.DocumentationViewModel", true)!;
var vm = Activator.CreateInstance(viewType)!;
var roots = (System.Collections.IList)viewType.GetProperty("RootEntries")!.GetValue(vm)!;
foreach (var node in (System.Collections.IEnumerable)decodedTree!) roots.Add(node);
var find = viewType.GetMethod("FindEntryByPath", BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Instance)!;
foreach (var path in new[]{"Guide/Sample","Guide/Missing"})
{
    var node=find.Invoke(vm, new object[]{path});
    var content=node?.GetType().GetProperty("Contents")?.GetValue(node) as string;
    bool passed=path=="Guide/Sample" ? content=="# Synthetic fixture\n" : node==null;
    Console.WriteLine(JsonSerializer.Serialize(new{scenario="ui-path",path,passed,content}));
    if (!passed) Environment.ExitCode=1;
}

sealed class FixtureContext(string root) : AssemblyLoadContext()
{
    protected override Assembly? Load(AssemblyName name)
    {
        var candidate=Path.Combine(root,name.Name+".dll");
        return File.Exists(candidate) ? LoadFromAssemblyPath(candidate) : null;
    }
}

sealed class FixtureHandler(string body) : HttpMessageHandler
{
    public string Body {get;set;}=body;
    public HttpStatusCode Status {get;set;}=HttpStatusCode.OK;
    public string? Verb {get;private set;}
    public string? Path {get;private set;}
    protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request,CancellationToken token)
    {
        Verb=request.Method.Method;
        Path=request.RequestUri?.AbsolutePath;
        return Task.FromResult(new HttpResponseMessage(Status){Content=new StringContent(Body,System.Text.Encoding.UTF8,"application/json")});
    }
}
