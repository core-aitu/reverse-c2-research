using SpecterInsight.Server.Services.Tor;
using SpecterInsight.Server.Services.Tor.Core.Circuit;

internal sealed class StubTorService : ITorService
{
    public bool IsRunning => false;
    public bool IsBootstrapped => false;
    public int SocksPort => 0;
    public Task StartAsync(int socksPort = 9050, CancellationToken ct = default) => Task.CompletedTask;
    public Task StopAsync(CancellationToken ct = default) => Task.CompletedTask;
    public Task<TorStatus> GetStatusAsync(CancellationToken ct = default) => Task.FromResult(new TorStatus());
    public Task<List<TorExitNode>> GetExitNodesAsync(string? countryCode = null, CancellationToken ct = default) => Task.FromResult(new List<TorExitNode>());
    public Task SetExitCountryAsync(string countryCode, CancellationToken ct = default) => Task.CompletedTask;
    public Task SetExitNodeAsync(string fingerprint, CancellationToken ct = default) => Task.CompletedTask;
    public Task SetExitNodesAsync(string[] fingerprints, CancellationToken ct = default) => Task.CompletedTask;
    public Task ClearExitSelectionAsync(CancellationToken ct = default) => Task.CompletedTask;
    public Task RotateCircuitsAsync(CancellationToken ct = default) => Task.CompletedTask;
    public HttpClient CreateHttpClient() => new();
    public Task<Stream> CreateTcpStreamAsync(string host, int port, CancellationToken ct = default) => throw new NotSupportedException();
    public Task<Stream> CreateTcpStreamAsync(string host, int port, string? exitCountry, string? exitFingerprint, CancellationToken ct = default) => throw new NotSupportedException();
    public Task<TorCircuitPool> CreateCircuitPoolAsync(string? exitCountry, string? exitFingerprint, CancellationToken ct = default) => throw new NotSupportedException();
    public ValueTask DisposeAsync() => ValueTask.CompletedTask;
}
