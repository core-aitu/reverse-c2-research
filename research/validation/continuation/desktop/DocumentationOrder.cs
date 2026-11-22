using Avalonia.Controls;
using Avalonia.VisualTree;
using System.Net;
using System.Reflection;
using System.Text.Json;
using System.Threading.Channels;

sealed class HeldDocumentationRequests : HttpMessageHandler
{
    internal record Request(TaskCompletionSource<HttpResponseMessage> Reply, CancellationToken Cancellation);
    private readonly Channel<Request> arrivals = Channel.CreateUnbounded<Request>();
    internal async Task<Request> Next() =>
        await arrivals.Reader.ReadAsync().AsTask().WaitAsync(TimeSpan.FromSeconds(5));

    protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellation)
    {
        if (request.Method != HttpMethod.Get || request.RequestUri?.AbsolutePath != "/Documentation")
            throw new InvalidDataException("Unexpected documentation request");
        var completion = new TaskCompletionSource<HttpResponseMessage>(TaskCreationOptions.RunContinuationsAsynchronously);
        if (!arrivals.Writer.TryWrite(new Request(completion, cancellation)))
            throw new InvalidOperationException("Could not record request");
        return completion.Task.WaitAsync(cancellation);
    }

    internal static void Return(Request request, string name)
    {
        var json = JsonSerializer.Serialize(new[] { new { name, contents = name, children = Array.Empty<object>() } });
        request.Reply.SetResult(new HttpResponseMessage(HttpStatusCode.OK)
            { Content = new StringContent(json, System.Text.Encoding.UTF8, "application/json") });
    }
}

static class DocumentationOrder
{
    private static async Task Until(Func<bool> observed)
    {
        using var deadline = new CancellationTokenSource(TimeSpan.FromSeconds(5));
        while (!observed()) await Task.Delay(15, deadline.Token);
    }

    internal static async Task Run(Assembly ui, HeldDocumentationRequests requests, HttpClient http, bool testCancellation)
    {
        var type = ui.GetType("SpecterInsight.UI.Views.DocumentationViewer", true)!;
        var control = (Control)Activator.CreateInstance(type)!;
        var window = new Window { Content = control, Width = 1000, Height = 600 };
        window.Show();
        var first = await requests.Next();
        var model = control.DataContext!;
        var documents = model.GetType().GetProperty("Documents")!;
        string? ModelName()
        {
            var nodes = documents.GetValue(model) as System.Collections.IEnumerable;
            var node = nodes?.Cast<object>().SingleOrDefault();
            return node?.GetType().GetProperty("Name")?.GetValue(node) as string;
        }
        bool Visible(string label) => window.GetVisualDescendants().OfType<TextBlock>()
            .Any(t => t.IsEffectivelyVisible && t.Bounds.Width > 0 && t.Bounds.Height > 0 && t.Text == label);
        void Record(string id, object result) =>
            Console.WriteLine(JsonSerializer.Serialize(new { id, result }));

        var refresh = model.GetType().GetMethod("UpdateDocumentation")!;
        refresh.Invoke(model, null);
        var second = await requests.Next();
        HeldDocumentationRequests.Return(second, "Second request");
        await Until(() => ModelName() == "Second request" && Visible("Second request"));
        Record("second-response", new { model = ModelName(), visible = Visible("Second request") });
        HeldDocumentationRequests.Return(first, "First request");
        await Until(() => ModelName() == "First request" && Visible("First request") && !Visible("Second request"));
        Record("late-first-response", new { model = ModelName(), visible = Visible("First request"),
            previousVisible = Visible("Second request") });

        if (testCancellation)
        {
            // The public refresh returns void. Await its original worker to observe the terminal state.
            var worker = model.GetType().GetMethod("<UpdateDocumentation>b__5_0",
                BindingFlags.NonPublic | BindingFlags.Instance)!;
            var work = (Task)worker.Invoke(model, null)!;
            var outstanding = await requests.Next();
            http.CancelPendingRequests();
            await work.WaitAsync(TimeSpan.FromSeconds(5));
            await Avalonia.Threading.Dispatcher.UIThread.InvokeAsync(
                () => { }, Avalonia.Threading.DispatcherPriority.Background);
            var visible = Visible("First request");
            if (!outstanding.Cancellation.IsCancellationRequested || ModelName() != "First request" || !visible)
                throw new InvalidDataException("Unexpected documentation state after HTTP cancellation");
            Record("http-cancel", new { tokenCancelled = true, taskStatus = work.Status.ToString(),
                model = ModelName(), visible });
        }

        refresh.Invoke(model, null);
        var pending = await requests.Next();
        window.Content = null;
        await Until(() => control.GetVisualRoot() is null);
        HeldDocumentationRequests.Return(pending, "Detached response");
        await Until(() => ModelName() == "Detached response");
        Record("response-after-detach", new { detached = control.GetVisualRoot() is null,
            requestCancelled = pending.Cancellation.IsCancellationRequested, model = ModelName(),
            visible = Visible("Detached response") });
        window.Close();
    }
}
