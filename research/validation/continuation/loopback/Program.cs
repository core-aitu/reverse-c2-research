using System.Net;
using System.Reflection;
using System.Runtime.Loader;
using System.Security.Claims;
using System.Text.Encodings.Web;
using System.Text.Json;
using System.Threading.Channels;
using Microsoft.AspNetCore.Authentication;
using Microsoft.AspNetCore.Mvc.ApplicationParts;
using Microsoft.AspNetCore.SignalR;
using Microsoft.AspNetCore.SignalR.Client;
using Microsoft.Extensions.Options;
using Npgsql;

AssemblyLoadContext.Default.Resolving+=(_,n)=>{
    foreach(var dir in new[]{"/lib-client","/lib-server"})
    {
        var path=Path.Combine(dir,n.Name+".dll");
        if(File.Exists(path))return AssemblyLoadContext.Default.LoadFromAssemblyPath(path);
    }
    return null;
};
var server=Assembly.LoadFrom("/tmp/study/bin/server/SpecterInsight.Server.dll");
var api=Assembly.LoadFrom("/lib-client/SpecterInsight.ApiClient.dll");
Type S(string name)=>server.GetType("SpecterInsight.Server."+name,true)!;
var dc=S("Controllers.DocumentationController");
var docServices=dc.GetConstructors().Single().GetParameters();
var builder=WebApplication.CreateBuilder(new WebApplicationOptions{ContentRootPath="/tmp"});
builder.Logging.ClearProviders();
builder.WebHost.UseUrls("http://127.0.0.1:0");
builder.Services.AddControllers().ConfigureApplicationPartManager(m=>{
    m.ApplicationParts.Clear();m.ApplicationParts.Add(new OneController(dc));
});
foreach(var p in docServices)
{
    if(p.ParameterType.IsInterface) builder.Services.AddSingleton(p.ParameterType,DispatchProxy.Create(p.ParameterType,typeof(UnusedService)));
    else builder.Services.AddSingleton(p.ParameterType);
}
builder.Services.AddAuthentication("Fixture").AddScheme<AuthenticationSchemeOptions,FixtureAuth>("Fixture",_=>{});
builder.Services.AddAuthorization();
builder.Services.AddSignalR();
// Only the PSObject data container is used. No runspace or scripts.
var mac=S("Services.DataAugmentation.MacVendorDataTransformService");
var dictionaryCtor=mac.GetConstructors(BindingFlags.NonPublic|BindingFlags.Public|BindingFlags.Instance)
    .Single(c=>c.GetParameters()[0].ParameterType.IsGenericType);
var transform=dictionaryCtor.Invoke([new Dictionary<string,string>()]);
File.WriteAllText("/tmp/control.csv","Mac Prefix,Vendor Name,Private\nAA:BB:01,\"Example, Incorporated\",false\n");
var dictionary=mac.GetMethod("LoadVendors",BindingFlags.NonPublic|BindingFlags.Instance)!.Invoke(transform,["/tmp/control.csv"]);
transform=dictionaryCtor.Invoke([dictionary]);
var sma=Assembly.LoadFrom("/lib-server/System.Management.Automation.dll");
var ps=sma.GetType("System.Management.Automation.PSObject",true)!;
var note=sma.GetType("System.Management.Automation.PSNoteProperty",true)!;
var record=Activator.CreateInstance(ps)!;
var props=ps.GetProperty("Properties")!.GetValue(record)!;
var add=props.GetType().GetMethods().Single(m=>m.Name=="Add" && m.GetParameters().Length==1);
add.Invoke(props,[Activator.CreateInstance(note,["MacAddress","AA:BB:01:00:00:00"])]);
var items=Array.CreateInstance(ps,1);items.SetValue(record,0);
mac.GetMethod("ApplyHelper",BindingFlags.NonPublic|BindingFlags.Instance)!.Invoke(transform,[items]);
var field=props.GetType().GetProperty("Item")!.GetValue(props,["MacAddress_vendor"]);
var actualVendor=field?.GetType().GetProperty("Value")?.GetValue(field);
Console.WriteLine(JsonSerializer.Serialize(new{id="csv-consumer",value=actualVendor,passed=Equals(actualVendor,"Example")}));
if(!Equals(actualVendor,"Example"))throw new Exception("MAC field was not augmented");
await using var ds=NpgsqlDataSource.Create("Host=/tmp;Username=fixture;Database=postgres;Pooling=false");
var db=S("Services.Database.DatabaseService");
var database=Activator.CreateInstance(db,[ds])!;
var searchType=S("Services.Documentation.DocumentationService");
var searchCtor=searchType.GetConstructors().Single();var searchParams=searchCtor.GetParameters();
var embeddings=DispatchProxy.Create(searchParams[1].ParameterType,typeof(EmptyEmbedding));
var state=Activator.CreateInstance(searchParams[2].ParameterType)!;
var nullLogger=typeof(Microsoft.Extensions.Logging.Abstractions.NullLogger<>).MakeGenericType(searchType);
var search=searchCtor.Invoke([database,embeddings,state,nullLogger.GetField("Instance")!.GetValue(null)]);
builder.Services.AddSingleton(docServices[0].ParameterType,search);
builder.Services.AddSingleton(docServices[1].ParameterType,state);
async Task<object?> DbCall(string name,params object?[] args){
    var method=db.GetMethod(name,BindingFlags.Public|BindingFlags.NonPublic|BindingFlags.Instance)!;
    var task=(Task)method.Invoke(database,args)!;await task;return task.GetType().GetProperty("Result")?.GetValue(task);
}
await DbCall("CreateDocumentationChunksTableAsync");
var chunkType=S("Services.Database.Entities.DocumentationChunk");
var chunk=Activator.CreateInstance(chunkType)!;
foreach(var (name,val) in new (string,object)[]{("Path","study.md"),("Title","Synthetic Cherries"),("Content","Synthetic Cherries guide"),("ChunkIndex",0)})
    chunkType.GetProperty(name)!.SetValue(chunk,val);
await DbCall("UpsertDocumentationChunkAsync",chunk,CancellationToken.None);
await using var app=builder.Build();
app.UseAuthentication();app.UseAuthorization();app.MapControllers();app.MapHub<AuditSink>("/audit-observation");
// A separate fixture route calls the file reader; it does not impersonate a product account.
string syntheticDocs="/tmp/fixture-documents";
app.MapGet("/reference/Documentation",async ()=>{
    var load=dc.GetMethod("Load",BindingFlags.Static|BindingFlags.NonPublic|BindingFlags.Public)!;
    var task=(Task)load.Invoke(null,[syntheticDocs])!;await task;
    return Results.Json(task.GetType().GetProperty("Result")!.GetValue(task));
});
app.MapGet("/errors/Documentation",()=>Results.StatusCode(503));
app.MapPost("/reference/Documentation/search",async (HttpContext http)=>{
    var reqType=S("Controllers.DocumentationSearchRequest");
    var req=await JsonSerializer.DeserializeAsync(http.Request.Body,reqType,new JsonSerializerOptions(JsonSerializerDefaults.Web));
    var instance=Activator.CreateInstance(dc,[search,state])!;
    var action=dc.GetMethod("SearchDocumentation")!;
    var parameters=action.GetParameters().Select(p=>p.ParameterType==typeof(CancellationToken)?(object)CancellationToken.None:req).ToArray();
    var task=(Task)action.Invoke(instance,parameters)!;try{await task;}catch(Exception e){Console.Error.WriteLine(e.GetBaseException());throw;}
    var result=task.GetType().GetProperty("Result")!.GetValue(task)!;
    var nested=result.GetType().GetProperty("Result")?.GetValue(result);
    
    var value=nested?.GetType().GetProperty("Value")?.GetValue(nested)??result.GetType().GetProperty("Value")?.GetValue(result);
    return Results.Json(value);
});
await app.StartAsync();
var url=app.Urls.Single();
using var http=new HttpClient{BaseAddress=new Uri(url)};
var docType=api.GetType("SpecterInsight.ApiClient.SpecterInsightClient",true)!;
var ctor=docType.GetConstructors(BindingFlags.Instance|BindingFlags.NonPublic|BindingFlags.Public)
    .Single(c=>c.GetParameters().Length==2 && c.GetParameters()[1].ParameterType==typeof(HttpClient));
var client=ctor.Invoke([url,http]);
async Task<object?> ApiDocs()
{
    var task=(Task)docType.GetMethods().Single(m=>m.Name=="DocumentationAsync" && m.GetParameters().Length==0).Invoke(client,null)!;
    await task;return task.GetType().GetProperty("Result")!.GetValue(task);
}
void Test(string id,bool passed)
{
    Console.WriteLine(JsonSerializer.Serialize(new{id,passed}));if(!passed)throw new Exception(id);
}
var dir=syntheticDocs;
Directory.CreateDirectory(Path.Combine(dir,"Guide"));
File.WriteAllText(Path.Combine(dir,"Guide","Overview.md"),"Folder summary");
File.WriteAllText(Path.Combine(dir,"Guide","Sample.md"),"# Synthetic fixture\n");
var denied=await http.GetAsync("/Documentation");Test("no-fixture-identity-401",denied.StatusCode==HttpStatusCode.Unauthorized);
bool exception=false;try{await ApiDocs();}catch(Exception ex){exception=ex.GetBaseException().GetType().Name=="ApiException";}
Test("api-denied-exception",exception);
http.DefaultRequestHeaders.Add("X-Study-Identity","reader");
var response=await http.GetAsync("/Documentation");
Test("product-filter-rejects-fixture-identity",response.StatusCode==HttpStatusCode.Unauthorized);
client=ctor.Invoke([url+"/errors",http]);
bool badStatus=false;
try {await ApiDocs();} catch(Exception ex){badStatus=ex.GetBaseException().GetType().Name=="ApiException";}
Test("fixture-http-503-client-exception",badStatus);
client=ctor.Invoke([url+"/reference",http]);
response=await http.GetAsync("/reference/Documentation");
Test("fixture-reader-200",response.StatusCode==HttpStatusCode.OK);
var opts=new JsonSerializerOptions(JsonSerializerDefaults.Web);
var body=await response.Content.ReadAsStringAsync();var tree=await ApiDocs();
var decoded=JsonSerializer.SerializeToElement(tree,opts);
Test("api-read-dto",decoded[0].GetProperty("name").GetString()=="Guide" &&
 decoded[0].GetProperty("children").EnumerateArray().Any(c=>c.GetProperty("name").GetString()=="Sample" && c.GetProperty("contents").GetString()=="# Synthetic fixture\n"));
Console.WriteLine(JsonSerializer.Serialize(new{id="http-pair",method="GET",path="/reference/Documentation",status=(int)response.StatusCode,body=JsonSerializer.Deserialize<JsonElement>(body)}));
Directory.Delete(Path.Combine(dir,"Guide"),true);

Test("empty-tree",JsonSerializer.Serialize(await ApiDocs(),opts)=="[]");

await (Task)db.GetMethod("CreateEventsTableAsync",BindingFlags.Instance|BindingFlags.Public|BindingFlags.NonPublic)!.Invoke(database,null)!;
var serviceType=S("Services.Events.EventLoggingService");var argsInfo=serviceType.GetConstructors().Single().GetParameters();
var ctx=app.Services.GetRequiredService<IHubContext<AuditSink>>();
var hubProxy=DispatchProxy.Create(argsInfo[1].ParameterType,typeof(ContextBridge));
((ContextBridge)hubProxy).Context=ctx;
var logger=Microsoft.Extensions.Logging.Abstractions.NullLogger.Instance;
var audit=Activator.CreateInstance(serviceType,[database,hubProxy,logger])!;
var evt=S("Services.Events.Event");
var evtCtor=evt.GetConstructors().Single(c=>c.GetParameters().Length==8);
var log=serviceType.GetMethods().Single(m=>m.Name=="LogAsync" && m.GetParameters().Length==2);
async Task Emit(string id){
    var e=evtCtor.Invoke([id,"user-login",new DateTime(2027,1,2,3,4,5,DateTimeKind.Utc),"test-source","test-system",Enum.Parse(evtCtor.GetParameters()[5].ParameterType,"Success"),id=="fixture-live-window"?"Synthetic audit "+id:"Synthetic audit event","{}"]);
    await (Task)log.Invoke(audit,[e,CancellationToken.None])!;
}
await using var conn=new HubConnectionBuilder().WithUrl(url+"/audit-observation").Build();
var queue=Channel.CreateUnbounded<string>();
using var sub=conn.On<JsonElement>("EventNotification", value=>{
    var obj=JsonSerializer.Deserialize(value,api.GetType("SpecterInsight.ApiClient.Event",true)!,opts)!;
    var id=(string)obj.GetType().GetProperty("Id")!.GetValue(obj)!;
    if(id=="fixture-online"){
        var captured=JsonSerializer.SerializeToNode(value)!.AsObject();
        Console.WriteLine(JsonSerializer.Serialize(new{id="signalr-record",record=captured}));
    }
    queue.Writer.TryWrite(id);
});
async Task<string> Next()=>await queue.Reader.ReadAsync(new CancellationTokenSource(TimeSpan.FromSeconds(5)).Token);
await conn.StartAsync();await Emit("fixture-online");Test("signalr-first",await Next()=="fixture-online");
await conn.StopAsync();await Emit("fixture-offline");await conn.StartAsync();await Emit("fixture-reconnected");
Test("signalr-first-after-new-connection",await Next()=="fixture-reconnected");
var searchReq=api.GetType("SpecterInsight.ApiClient.DocumentationSearchRequest",true)!;
var input=Activator.CreateInstance(searchReq)!;searchReq.GetProperty("Query")!.SetValue(input,"Cherries");searchReq.GetProperty("TopK")!.SetValue(input,3);
var sendSearch=docType.GetMethods().Single(m=>m.Name=="SearchAsync" && m.GetParameters().Length==1 && m.GetParameters()[0].ParameterType==searchReq);
var searchTask=(Task)sendSearch.Invoke(client,[input])!;await searchTask;
var hits=(System.Collections.IEnumerable)searchTask.GetType().GetProperty("Result")!.GetValue(searchTask)!;
var first=hits.Cast<object>().First();
Test("search-api-to-stored-fixture",Equals(first.GetType().GetProperty("Title")!.GetValue(first),"Synthetic Cherries"));
await using var cmd=ds.CreateCommand("select string_agg(identifier, ',' order by id) from event");
Test("database-correlated",Equals(await cmd.ExecuteScalarAsync(),"fixture-online,fixture-offline,fixture-reconnected"));
if(File.Exists("/screen/Screen.dll")){
    var start=new System.Diagnostics.ProcessStartInfo("/dotnet/dotnet","/screen/Screen.dll"){UseShellExecute=false,RedirectStandardOutput=true};
    start.Environment["STUDY_AUDIT_URL"]=url+"/audit-observation";
    using var child=System.Diagnostics.Process.Start(start)!;
    try{
        var ready=await child.StandardOutput.ReadLineAsync().WaitAsync(TimeSpan.FromSeconds(20));
        Test("stock-window-subscribed",ready=="DESKTOP-READY");
        await Emit("fixture-live-window");
        var received=await child.StandardOutput.ReadLineAsync().WaitAsync(TimeSpan.FromSeconds(20));
        using var result=JsonDocument.Parse(received!);
        Test("stock-window-visible",result.RootElement.GetProperty("passed").GetBoolean() && result.RootElement.GetProperty("eventId").GetString()=="fixture-live-window");
        Console.WriteLine(received);
        await child.WaitForExitAsync().WaitAsync(TimeSpan.FromSeconds(10));Test("stock-window-exit",child.ExitCode==0);
        await using var check=ds.CreateCommand("select count(*) from event where identifier='fixture-live-window'");
        Test("same-id-in-database",Equals(await check.ExecuteScalarAsync(),1L));
    }finally{if(!child.HasExited)child.Kill(true);}
}
await conn.StopAsync();await app.StopAsync();

sealed class OneController(Type type):ApplicationPart,IApplicationPartTypeProvider
{
 public override string Name=>"Documentation fixture";public IEnumerable<TypeInfo> Types=>[type.GetTypeInfo()];
}
public class UnusedService:DispatchProxy
{
 protected override object? Invoke(MethodInfo? m,object?[]? a)=>throw new NotSupportedException(m?.Name);
}
public class ContextBridge:DispatchProxy
{
 public IHubContext<AuditSink> Context {get;set;}=null!;
 protected override object? Invoke(MethodInfo? m,object?[]? a)=>m?.Name switch {
    "get_Clients"=>Context.Clients,"get_Groups"=>Context.Groups,_=>throw new NotSupportedException(m?.Name)};
}
public class AuditSink:Hub {}
sealed class FixtureAuth(IOptionsMonitor<AuthenticationSchemeOptions> o,ILoggerFactory l,UrlEncoder e):AuthenticationHandler<AuthenticationSchemeOptions>(o,l,e)
{
 protected override Task<AuthenticateResult> HandleAuthenticateAsync()=>Task.FromResult(
    Request.Headers["X-Study-Identity"]=="reader"?AuthenticateResult.Success(new AuthenticationTicket(new ClaimsPrincipal(new ClaimsIdentity([new Claim(ClaimTypes.Name,"reader")],Scheme.Name)),Scheme.Name)):AuthenticateResult.NoResult());
}

public class EmptyEmbedding:DispatchProxy
{
 protected override object? Invoke(MethodInfo? m,object?[]? a)
 {
    if(m!.ReturnType==typeof(Task<float[]>)) return Task.FromResult(Array.Empty<float>());
    throw new NotSupportedException(m.Name+" returns "+m.ReturnType);
 }
}
