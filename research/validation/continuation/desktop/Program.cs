using Microsoft.AspNetCore.SignalR.Client;
using Avalonia;
using Avalonia.Controls;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Threading;
using Avalonia.VisualTree;
using System.Net;
using System.Reflection;
using System.Runtime.Loader;
using System.Text.Json;

AssemblyLoadContext.Default.Resolving+=(_,n)=>{
    var path=(Directory.Exists("/files")?"/files/":"/lib-client/")+n.Name+".dll";return File.Exists(path)?AssemblyLoadContext.Default.LoadFromAssemblyPath(path):null;
};
AppBuilder.Configure<ScreenApp>().UsePlatformDetect().StartWithClassicDesktopLifetime(args,ShutdownMode.OnExplicitShutdown);
public class ScreenApp:Application
{
    public override void Initialize()
    {
        // Load only shipped XAML resources; the product startup lifecycle is outside this fixture.
        var app=(Application)Activator.CreateInstance(Assembly.LoadFrom((Directory.Exists("/files")?"/files/":"/lib-client/")+"SpecterInsight.UI.dll").GetType("SpecterInsight.UI.App",true)!)!;
        app.Initialize();
        foreach(var style in app.Styles.ToArray()){app.Styles.Remove(style);Styles.Add(style);}
        var resources=app.Resources;app.Resources=new Avalonia.Controls.ResourceDictionary();Resources=resources;
    }
    public override void OnFrameworkInitializationCompleted()
    {
        base.OnFrameworkInitializationCompleted();
        Dispatcher.UIThread.Post(async ()=>{
            int exit=0;
            try{await Run();}catch(Exception e){Console.Error.WriteLine(e);exit=1;}
            ((IClassicDesktopStyleApplicationLifetime)ApplicationLifetime!).Shutdown(exit);
        });
    }
    static async Task Settle()=>await Task.Delay(700);
    static void Log(string id,object value)=>Console.WriteLine(JsonSerializer.Serialize(new{id,value}));
    static async Task Run()
    {
        var ui=Assembly.LoadFrom((Directory.Exists("/files")?"/files/":"/lib-client/")+"SpecterInsight.UI.dll");
        var ac=Assembly.LoadFrom((Directory.Exists("/files")?"/files/":"/lib-client/")+"SpecterInsight.ApiClient.dll");
        var handler=new Reply();
        using var http=new HttpClient(handler){BaseAddress=new Uri("http://fixture.invalid")};
        var ct=ac.GetType("SpecterInsight.ApiClient.SpecterInsightClient",true)!;
        var client=ct.GetConstructors(BindingFlags.Public|BindingFlags.NonPublic|BindingFlags.Instance)
            .Single(c=>c.GetParameters().Length==2 && c.GetParameters()[1].ParameterType==typeof(HttpClient))
            .Invoke(["http://fixture.invalid",http]);
        ui.GetType("SpecterInsight.UI.Globals",true)!.GetProperty("Client",BindingFlags.Static|BindingFlags.Public|BindingFlags.NonPublic)!.SetValue(null,client);
        static string[] Texts(Control w)=>w.GetVisualDescendants().OfType<TextBlock>()
            .Where(t=>t.IsEffectivelyVisible && t.Bounds.Width>0 && t.Bounds.Height>0)
            .Select(t=>t.Text??"").Where(t=>!string.IsNullOrWhiteSpace(t)).Distinct().ToArray();
        static int Size(object? obj)=>(obj as System.Collections.IEnumerable)?.Cast<object>().Count()??-1;
        var live=Environment.GetEnvironmentVariable("STUDY_AUDIT_URL");
        if(live!=null)
        {
            handler.Mode="empty";
            var control=(Control)Activator.CreateInstance(ui.GetType("SpecterInsight.UI.EventLogControl",true)!)!;
            var liveWindow=new Window{Content=control,Width=1100,Height=650};
            liveWindow.Show();await Settle();
            var liveView=control.DataContext!;
            var receive=liveView.GetType().GetMethod("OnEventNotification",BindingFlags.Public|BindingFlags.NonPublic|BindingFlags.Instance)!;
            var arrived=new TaskCompletionSource<string>(TaskCreationOptions.RunContinuationsAsynchronously);
            await using var wire=new HubConnectionBuilder().WithUrl(live).Build();
            // Supply the fixture transport only. The shipped ApiClient registers its named subscription.
            ct.GetField("_hub",BindingFlags.Instance|BindingFlags.NonPublic)!.SetValue(client,wire);
            var hook=ct.GetMethod("OnEventNotification")!;
            var dt=hook.GetParameters()[0].ParameterType;
            var argument=System.Linq.Expressions.Expression.Parameter(dt.GenericTypeArguments[0],"evt");
            async Task Display(object dto){
                try{
                    await (Task)receive.Invoke(liveView,[dto])!;
                    arrived.TrySetResult((string)dto.GetType().GetProperty("Id")!.GetValue(dto)!);
                }catch(Exception e){arrived.TrySetException(e);throw;}
            }
            Func<object,Task> display=Display;
            var callback=System.Linq.Expressions.Expression.Lambda(dt,
                System.Linq.Expressions.Expression.Invoke(System.Linq.Expressions.Expression.Constant(display),argument),argument).Compile();
            using var subscription=(IDisposable)hook.Invoke(client,[callback])!;
            await wire.StartAsync();
            Console.WriteLine("DESKTOP-READY");Console.Out.Flush();
            var id=await arrived.Task.WaitAsync(TimeSpan.FromSeconds(12));
            await Settle();
            var entries=(System.Collections.IEnumerable)liveView.GetType().GetProperty("Events")!.GetValue(liveView)!;
            var present=entries.Cast<object>().Any(e=>Equals(e.GetType().GetProperty("Id")!.GetValue(e),id));
            var visible=Texts(liveWindow).Contains("Synthetic audit "+id);
            Console.WriteLine(JsonSerializer.Serialize(new{id="live-stock-window",eventId=id,present,visible,passed=present&&visible}));
            if(!present||!visible)throw new InvalidDataException("Live event missing from liveWindow");
            await wire.StopAsync();liveWindow.Close();return;
        }
        foreach(var mode in new[]{"success","empty","error"})
        {
            handler.Mode=mode;
            var obj=(Control)Activator.CreateInstance(ui.GetType("SpecterInsight.UI.Views.DocumentationViewer",true)!)!;
            var win=obj as Window??new Window{Content=obj};
            win.Show();await Settle();
            var vm=obj.DataContext!;
            Log("doc-"+mode,new{window=win.IsVisible,kind=obj.GetType().BaseType?.Name,
                requests=handler.Paths.ToArray(),documents=Size(vm.GetType().GetProperty("Documents")?.GetValue(vm)),
                text=Texts(win).Where(t=>t.Contains("Fixture",StringComparison.OrdinalIgnoreCase)).ToArray()});
            if(mode=="success"){
                foreach(var next in new[]{"error","empty"}){
                    handler.Mode=next;string? error=null;
                    try{var result=vm.GetType().GetMethod("UpdateDocumentation")!.Invoke(vm,null);if(result is Task task)await task;}catch(Exception e){error=e.GetBaseException().GetType().Name;}
                    await Settle();
                    Log("doc-retention-"+next,new{documents=Size(vm.GetType().GetProperty("Documents")!.GetValue(vm)),error,visible=Texts(win).Contains("Fixture Page")});
                }
            }
            win.Close();await Settle();handler.Paths.Clear();
        }
        var eventType=ui.GetType("SpecterInsight.UI.EventLogControl",true)!;
        handler.Mode="success";
        handler.Paths.Clear();
        var eventControl=(Control)Activator.CreateInstance(eventType)!;
        var window=new Window{Content=eventControl,Width=1000,Height=600};
        window.Show();await Settle();
        var evm=eventControl.DataContext!;
        var refresh=evm.GetType().GetMethods(BindingFlags.Instance|BindingFlags.Public|BindingFlags.NonPublic).Single(m=>m.Name=="Refresh" && m.GetParameters().Length==1);
        foreach(var mode in new[]{"success","error","empty"})
        {
            handler.Mode=mode;string? error=null;
            try {await (Task)refresh.Invoke(evm,[true])!;}catch(Exception e){error=e.GetBaseException().GetType().Name;}
            await Settle();Log("events-"+mode,new{window=window.IsVisible,requests=handler.Paths.ToArray(),
                events=Size(evm.GetType().GetProperty("Events")!.GetValue(evm)),error,visible=Texts(window).Where(t=>(t.Contains("fixture",StringComparison.OrdinalIgnoreCase)||t.Contains("Synthetic",StringComparison.OrdinalIgnoreCase)||t=="user-login")).ToArray()});
            handler.Paths.Clear();
        }
        window.Close();
        handler.Mode="success";handler.Paths.Clear();
        var reopened=(Control)Activator.CreateInstance(eventType)!;
        var again=new Window{Content=reopened,Width=1000,Height=600};
        again.Show();await Settle();
        await (Task)refresh.Invoke(reopened.DataContext,[true])!;await Settle();
        Log("events-reopened",new{window=again.IsVisible,events=Size(evm.GetType().GetProperty("Events")!.GetValue(reopened.DataContext)),visible=Texts(again).Contains("Synthetic audit event")});
        again.Close();
    }
}
sealed class Reply:HttpMessageHandler
{
    public List<string> Paths {get;}=[];
    public string Mode {get;set;}="success";
    protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage req,CancellationToken ct)
    {
        var path=req.RequestUri!.AbsolutePath;Paths.Add(path);
        var doc=path.TrimEnd('/').EndsWith("/Documentation",StringComparison.OrdinalIgnoreCase);
        if(!doc && !path.Contains("event",StringComparison.OrdinalIgnoreCase))throw new InvalidOperationException("Unrecognised fixture path: "+path);
        var record=Mode!="empty" && !doc ? System.Text.Json.Nodes.JsonNode.Parse(File.ReadAllText("/record.json")) : null;
        string body=Mode=="empty"?"[]":doc?"[{\"name\":\"Fixture Page\",\"contents\":\"Fixture paragraph\",\"children\":[]}]":
            "["+record!.ToJsonString()+"]";
        return Task.FromResult(new HttpResponseMessage(Mode=="error"?HttpStatusCode.ServiceUnavailable:HttpStatusCode.OK){Content=new StringContent(body,System.Text.Encoding.UTF8,"application/json")});
    }
}
