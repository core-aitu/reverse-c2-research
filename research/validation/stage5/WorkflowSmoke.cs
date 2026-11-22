using System.Net;
using System.Reflection;
using System.Runtime.Loader;
using System.Security.Cryptography;
using System.Text.Json;
using Avalonia;
using Avalonia.Controls;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Media.Imaging;
using Avalonia.ReactiveUI;
using Avalonia.Threading;
using Avalonia.VisualTree;
using Microsoft.AspNetCore.SignalR.Client;

AssemblyLoadContext.Default.Resolving += (_, name) =>
{
    var path = Path.Combine("/files", name.Name + ".dll");
    return File.Exists(path) ? AssemblyLoadContext.Default.LoadFromAssemblyPath(path) : null;
};
AppBuilder.Configure<WorkflowApp>().UsePlatformDetect().UseReactiveUI()
    .StartWithClassicDesktopLifetime(args, ShutdownMode.OnExplicitShutdown);

public sealed class WorkflowApp : Application
{
    public override void Initialize()
    {
        var assembly = Assembly.LoadFrom("/files/SpecterInsight.UI.dll");
        var stock = (Application)Activator.CreateInstance(assembly.GetType("SpecterInsight.UI.App", true)!)!;
        stock.Initialize();
        foreach (var style in stock.Styles.ToArray())
        {
            stock.Styles.Remove(style);
            Styles.Add(style);
        }
        var resources = stock.Resources;
        stock.Resources = new Avalonia.Controls.ResourceDictionary();
        Resources = resources;
    }

    public override void OnFrameworkInitializationCompleted()
    {
        base.OnFrameworkInitializationCompleted();
        Dispatcher.UIThread.Post(async () =>
        {
            var exit = 0;
            try { await RunAsync(); }
            catch (Exception exception) { Console.Error.WriteLine(exception); exit = 1; }
            ((IClassicDesktopStyleApplicationLifetime)ApplicationLifetime!).Shutdown(exit);
        });
    }

    private static async Task RunAsync()
    {
        string[] workflows =
        [
            "ListenersControl", "CertificatesControl", "ImplantsControl", "PayloadPipelinesControl",
            "SessionsControl", "ScriptsControl", "WordlistsControl", "ModulesControl", "EventLogControl",
            "PowerShellNotebookWorkspaceControl", "WinRmSessionsControl", "TunnelsControl",
            "AiAgentProfilesControl", "AiAgentFunctionsControl", "AiAgentSessionsControl",
            "DocumentationHomeControl", "DocumentationControl", "DocumentationIndexControl",
            "NetworkMapControl", "ArtifactsControl", "UsersControl", "SoftwareUpdatesControl",
            "LicenseControl", "SettingsControl"
        ];
        var output = Environment.GetEnvironmentVariable("STAGE5_UI_OUTPUT") ?? "/out/workflows";
        Directory.CreateDirectory(output);
        var ui = Assembly.LoadFrom("/files/SpecterInsight.UI.dll");
        var api = Assembly.LoadFrom("/files/SpecterInsight.ApiClient.dll");
        using var http = new HttpClient(new EmptyBackend()) { BaseAddress = new Uri("http://fixture.invalid") };
        var clientType = api.GetType("SpecterInsight.ApiClient.SpecterInsightClient", true)!;
        var client = clientType.GetConstructors(BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Instance)
            .Single(c => c.GetParameters().Length == 2 && c.GetParameters()[1].ParameterType == typeof(HttpClient))
            .Invoke(["http://fixture.invalid", http]);
        await using var hub = new HubConnectionBuilder().WithUrl("http://fixture.invalid/hub").Build();
        clientType.GetField("_hub", BindingFlags.Instance | BindingFlags.NonPublic)!.SetValue(client, hub);
        ui.GetType("SpecterInsight.UI.Globals", true)!
            .GetProperty("Client", BindingFlags.Static | BindingFlags.Public | BindingFlags.NonPublic)!
            .SetValue(null, client);
        Type[] loadableTypes;
        try { loadableTypes = ui.GetTypes(); }
        catch (ReflectionTypeLoadException exception) { loadableTypes = exception.Types.OfType<Type>().ToArray(); }

        var results = new List<object>();
        foreach (var name in workflows)
        {
            Window? window = null;
            try
            {
                var type = loadableTypes.Single(t => t.Name == name && typeof(Control).IsAssignableFrom(t));
                var control = (Control)Activator.CreateInstance(type)!;
                window = control as Window ?? new Window { Content = control, Width = 1200, Height = 720 };
                if (window.Width <= 0 || double.IsNaN(window.Width)) window.Width = 1200;
                if (window.Height <= 0 || double.IsNaN(window.Height)) window.Height = 720;
                window.Show();
                await Task.Delay(300);
                var bitmap = new RenderTargetBitmap(new PixelSize(1200, 720), new Vector(96, 96));
                bitmap.Render(window);
                var path = Path.Combine(output, name + ".png");
                bitmap.Save(path);
                var bytes = await File.ReadAllBytesAsync(path);
                results.Add(new
                {
                    workflow = name,
                    passed = window.IsVisible && control.IsEffectivelyVisible && bytes.Length > 100,
                    window_visible = window.IsVisible,
                    control_visible = control.IsEffectivelyVisible,
                    width = window.Bounds.Width,
                    height = window.Bounds.Height,
                    visual_count = window.GetVisualDescendants().Count(),
                    data_context = control.DataContext?.GetType().FullName,
                    png_bytes = bytes.Length,
                    png_sha256 = Convert.ToHexString(SHA256.HashData(bytes)).ToLowerInvariant(),
                    error = (string?)null
                });
            }
            catch (Exception exception)
            {
                results.Add(new
                {
                    workflow = name,
                    passed = false,
                    error = exception.GetBaseException().GetType().Name + ": " + exception.GetBaseException().Message
                });
            }
            finally
            {
                window?.Close();
                await Task.Delay(50);
            }
        }
        var passed = results.Count(r => (bool)r.GetType().GetProperty("passed")!.GetValue(r)!);
        Console.WriteLine(JsonSerializer.Serialize(new
        {
            observed_at_utc = DateTimeOffset.UtcNow,
            expected = workflows.Length,
            passed,
            complete = passed == workflows.Length,
            workflows = results
        }, new JsonSerializerOptions { WriteIndented = true }));
        if (passed != workflows.Length) Environment.ExitCode = 1;
    }
}

internal sealed class EmptyBackend : HttpMessageHandler
{
    protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
    {
        var path = request.RequestUri?.AbsolutePath ?? "";
        var body = path.Contains("status", StringComparison.OrdinalIgnoreCase) ? "{}" : "[]";
        return Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK)
        {
            Content = new StringContent(body, System.Text.Encoding.UTF8, "application/json")
        });
    }
}
