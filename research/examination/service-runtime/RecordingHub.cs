using System.Collections.Concurrent;
using Microsoft.AspNetCore.SignalR;

internal sealed record HubMessage(DateTime TimestampUtc, string Method, object?[] Arguments);

internal sealed class RecordingClientProxy : IClientProxy
{
    public ConcurrentQueue<HubMessage> Messages { get; } = new();

    public Task SendCoreAsync(string method, object?[] args, CancellationToken cancellationToken = default)
    {
        Messages.Enqueue(new HubMessage(DateTime.UtcNow, method, args));
        return Task.CompletedTask;
    }
}

internal sealed class RecordingHubClients : IHubClients
{
    private readonly RecordingClientProxy _proxy;
    public RecordingHubClients(RecordingClientProxy proxy) => _proxy = proxy;
    public IClientProxy All => _proxy;
    public IClientProxy AllExcept(IReadOnlyList<string> excludedConnectionIds) => _proxy;
    public IClientProxy Client(string connectionId) => _proxy;
    public IClientProxy Clients(IReadOnlyList<string> connectionIds) => _proxy;
    public IClientProxy Group(string groupName) => _proxy;
    public IClientProxy GroupExcept(string groupName, IReadOnlyList<string> excludedConnectionIds) => _proxy;
    public IClientProxy Groups(IReadOnlyList<string> groupNames) => _proxy;
    public IClientProxy User(string userId) => _proxy;
    public IClientProxy Users(IReadOnlyList<string> userIds) => _proxy;
}

internal sealed class RecordingGroupManager : IGroupManager
{
    public Task AddToGroupAsync(string connectionId, string groupName, CancellationToken cancellationToken = default) => Task.CompletedTask;
    public Task RemoveFromGroupAsync(string connectionId, string groupName, CancellationToken cancellationToken = default) => Task.CompletedTask;
}

internal sealed class RecordingHubContext<THub> : IHubContext<THub> where THub : Hub
{
    public RecordingClientProxy Proxy { get; } = new();
    public IHubClients Clients { get; }
    public IGroupManager Groups { get; } = new RecordingGroupManager();

    public RecordingHubContext() => Clients = new RecordingHubClients(Proxy);
}
