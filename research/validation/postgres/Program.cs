using System.Collections;
using System.Reflection;
using System.Text.Json;
using System.Runtime.Loader;
using Npgsql;

await using var ds=NpgsqlDataSource.Create("Host=/tmp;Username=fixture;Database=postgres;Pooling=false");
AssemblyLoadContext.Default.Resolving+=(_,n)=>n.Name=="Newtonsoft.Json"?Assembly.LoadFrom("/files/Newtonsoft.Json.dll"):null;
var asm=Assembly.LoadFrom("/files/SpecterInsight.Server.dll");
var db=asm.GetType("SpecterInsight.Server.Services.Database.DatabaseService",true)!;
var service=Activator.CreateInstance(db,[ds])!;
const BindingFlags flags=BindingFlags.Instance|BindingFlags.Public|BindingFlags.NonPublic;
async Task<object?> Call(string name,params object?[] args)
{
    var task=(Task)db.GetMethod(name,flags)!.Invoke(service,args)!;
    await task;
    return task.GetType().GetProperty("Result")?.GetValue(task);
}
async Task<object?> Scalar(string sql)
{
    await using var cmd=ds.CreateCommand(sql);return await cmd.ExecuteScalarAsync();
}
void Test(string name,object? value,object? expected)
{
    bool passed=Equals(value,expected);Console.WriteLine(JsonSerializer.Serialize(new{scenario=name,value,expected,passed}));
    if(!passed) Environment.ExitCode=1;
}
Test("postgres-version",await Scalar("SHOW server_version"),"16.4");
for(int i=0;i<2;i++)
{
    await Call("CreateDocumentationChunksTableAsync");
    await Call("CreateEventsTableAsync");
}
Test("table-names",await Scalar("SELECT string_agg(table_name,',' ORDER BY table_name) FROM information_schema.tables WHERE table_schema='public'"),"documentationchunks,event");
Test("columns-doc",await Scalar("SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='documentationchunks'"),7L);
Test("columns-event",await Scalar("SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='event'"),9L);
var ct=CancellationToken.None;
Test("initial-count",await Call("GetDocumentationChunkCountAsync",ct),0);
var model=asm.GetType("SpecterInsight.Server.Services.Database.Entities.DocumentationChunk",true)!;
var sample=Activator.CreateInstance(model)!;
void Set(string name,object? value)=>model.GetProperty(name)!.SetValue(sample,value);
Set("Path","research/test.md");Set("Title","Example");Set("Content","one");Set("ChunkIndex",1);
await Call("UpsertDocumentationChunkAsync",sample,ct);
Test("insert-count",await Call("GetDocumentationChunkCountAsync",ct),1);
Test("empty-embedding-count",await Call("GetDocumentationChunkWithEmbeddingCountAsync",ct),0);
Set("Content","two");Set("Embedding",new float[]{1f,2f});await Call("UpsertDocumentationChunkAsync",sample,ct);
Test("same-key-count",await Call("GetDocumentationChunkCountAsync",ct),1);
Test("updated-embedding-count",await Call("GetDocumentationChunkWithEmbeddingCountAsync",ct),1);
var docs=(IEnumerable)(await Call("GetAllDocumentationChunksAsync",ct))!;
Test("updated-content",model.GetProperty("Content")!.GetValue(docs.Cast<object>().Single()),"two");
Set("ChunkIndex",2);await Call("UpsertDocumentationChunkAsync",sample,ct);
Test("second-chunk",await Call("GetDocumentationChunkCountAsync",ct),2);
await Call("ClearDocumentationChunksAsync",ct);Test("clear",await Call("GetDocumentationChunkCountAsync",ct),0);

var et=asm.GetType("SpecterInsight.Server.Services.Events.Event",true)!;
var eventCtor=et.GetConstructors().Single(c=>c.GetParameters().Length==8);
var moment=new DateTime(2026,1,2,3,4,5,DateTimeKind.Utc);
object MakeEvent(string id,string type,string outcome,DateTime utc) =>
    eventCtor.Invoke([id,type,utc,"fixture-source","fixture-system",
        Enum.Parse(eventCtor.GetParameters()[5].ParameterType,outcome),"fixture message","{}"]);
var logging=asm.GetType("SpecterInsight.Server.Services.Events.EventLoggingService",true)!;
var logCtor=logging.GetConstructors().Single();
var parameters=logCtor.GetParameters();
var seen=new List<string>();
bool failBroadcast=false;
object Stub(Type iface)
{
    var proxy=DispatchProxy.Create(iface,typeof(HubStub));
    ((HubStub)proxy).Handler=(m,a)=>{
        if(m.Name is "get_Clients" or "get_All") return Stub(m.ReturnType);
        if(m.Name=="SendCoreAsync")
        {
            var payload=(object?[])a![1]!;
            seen.Add((string)et.GetProperty("Id")!.GetValue(payload.Single())!);
            return failBroadcast?Task.FromException(new IOException("synthetic send failure")):Task.CompletedTask;
        }
        throw new NotSupportedException(m.Name);
    };
    return proxy;
}
var loggerType=parameters[2].ParameterType;
object logger=loggerType.IsGenericType
    ? typeof(Microsoft.Extensions.Logging.Abstractions.NullLogger<>).MakeGenericType(loggerType.GenericTypeArguments[0]).GetProperty("Instance")?.GetValue(null)
        ?? typeof(Microsoft.Extensions.Logging.Abstractions.NullLogger<>).MakeGenericType(loggerType.GenericTypeArguments[0]).GetField("Instance")!.GetValue(null)!
    : Microsoft.Extensions.Logging.Abstractions.NullLogger.Instance;
var loggingService=logCtor.Invoke([service,Stub(parameters[1].ParameterType),logger]);
var log=logging.GetMethods().Single(m=>m.Name=="LogAsync" && m.GetParameters().Length==2);
async Task Emit(object evt) => await (Task)log.Invoke(loggingService,[evt,ct])!;
var one=MakeEvent("fixture-1","audit-first","Success",moment);
var two=MakeEvent("fixture-2","audit-second","Failure",moment.AddHours(1));
await Emit(one);
failBroadcast=true;
await Emit(two);
Test("hub-received-ids",string.Join(",",seen),"fixture-1,fixture-2");
Test("stored-events",await Scalar("SELECT count(*) FROM event"),2L);
var filtered=(IEnumerable)(await Call("GetEventsByTypeAsync","audit-second",ct))!;
var back=filtered.Cast<object>().Single();
Console.WriteLine(JsonSerializer.Serialize(new{scenario="audit-row",record=JsonSerializer.SerializeToElement(back,et,new JsonSerializerOptions(JsonSerializerDefaults.Web))}));
Test("filtered-event-id",et.GetProperty("Id")!.GetValue(back),"fixture-2");
Test("filtered-event-outcome",et.GetProperty("Outcome")!.GetValue(back)!.ToString(),"Failure");
var later=(IEnumerable)(await Call("GetAllEventsFromStartDateAsync",moment.AddMinutes(1)))!;
Test("start-date-filter",et.GetProperty("Id")!.GetValue(later.Cast<object>().Single()),"fixture-2");
await Call("DeleteEventsByTypeAsync","audit-second",ct);
Test("remaining-event",await Scalar("SELECT identifier FROM event"),"fixture-1");

public class HubStub : DispatchProxy
{
    public Func<MethodInfo,object?[]?,object?> Handler {get;set;}=null!;
    protected override object? Invoke(MethodInfo? method,object?[]? args)=>Handler(method!,args);
}
