using System.Collections;
using System.Reflection;
using System.Text.Json;
using Avalonia.Controls;
using Avalonia.VisualTree;
using Microsoft.AspNetCore.SignalR.Client;

internal static class Observe
{
    // The network fixture already committed to sending this ID only after DESKTOP-READY.
    private const string Id = "fixture-live-window";
    private const BindingFlags AllStatic = BindingFlags.Static | BindingFlags.Public | BindingFlags.NonPublic;

    public static async Task Run(Assembly ui, object client, string fixtureUrl)
    {
        var control = (Control)Activator.CreateInstance(ui.GetType("SpecterInsight.UI.EventLogControl", true)!)!;
        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(12));
        var window = new Window { Content = control, Width = 1100, Height = 650 };
        window.Show();
        try
        {
            var view = control.DataContext!;
            var events = view.GetType().GetProperty("Events")!;
            var globals = ui.GetType("SpecterInsight.UI.Globals", true)!;
            await using var connection = new HubConnectionBuilder().WithUrl(fixtureUrl).Build();
            client.GetType().GetField("_hub", BindingFlags.Instance | BindingFlags.NonPublic)!.SetValue(client, connection);
            await connection.StartAsync(timeout.Token);
            var connected = globals.GetMethod("RaiseConnected", AllStatic)!;
            await (Task)connected.Invoke(null, [client])!;
            bool initialEmpty = !((IEnumerable)events.GetValue(view)!).Cast<object>().Any();
            if (!initialEmpty) throw new InvalidDataException("Expected empty initial audit list");
            Console.WriteLine("DESKTOP-READY");
            Console.Out.Flush();

            bool visible = false;
            int matches = 0;
            while (!visible || matches == 0)
            {
                await Task.Delay(40, timeout.Token);
                matches = ((IEnumerable)events.GetValue(view)!).Cast<object>().Count(e =>
                    Equals(e.GetType().GetProperty("Id")!.GetValue(e), Id));
                visible = window.GetVisualDescendants().OfType<TextBlock>().Any(text =>
                    text.IsEffectivelyVisible && text.Bounds.Width > 0 && text.Bounds.Height > 0 &&
                    text.Text == "Synthetic audit " + Id);
            }
            var result = new { id = "stock-connected-audit", eventId = Id, initialEmpty, matches, visible,
                               passed = matches == 1 && visible };
            Console.WriteLine(JsonSerializer.Serialize(result));
            if (!result.passed) throw new InvalidDataException("Duplicate event on first visible result");
            await connection.StopAsync(timeout.Token);
        }
        finally { window.Close(); }
    }
}
