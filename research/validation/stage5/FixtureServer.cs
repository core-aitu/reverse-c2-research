using System.Net;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text.Json;
using Microsoft.AspNetCore.SignalR;

var builder = WebApplication.CreateBuilder(args);
builder.Logging.ClearProviders();
using var rsa = RSA.Create(2048);
var request = new CertificateRequest("CN=localhost", rsa, HashAlgorithmName.SHA256, RSASignaturePadding.Pkcs1);
request.CertificateExtensions.Add(new X509BasicConstraintsExtension(false, false, 0, false));
request.CertificateExtensions.Add(new X509SubjectKeyIdentifierExtension(request.PublicKey, false));
var san = new SubjectAlternativeNameBuilder();
san.AddDnsName("localhost");
san.AddIpAddress(IPAddress.Loopback);
request.CertificateExtensions.Add(san.Build());
using var certificate = request.CreateSelfSigned(DateTimeOffset.UtcNow.AddMinutes(-5), DateTimeOffset.UtcNow.AddDays(1));
builder.WebHost.ConfigureKestrel(options => options.ListenLocalhost(1337, listen => listen.UseHttps(certificate)));
builder.Services.AddSignalR();
var app = builder.Build();
app.Use(async (context, next) =>
{
    Console.WriteLine(JsonSerializer.Serialize(new
    {
        observed_at_utc = DateTimeOffset.UtcNow,
        method = context.Request.Method,
        path = context.Request.Path.Value,
        authorization = context.Request.Headers.Authorization.ToString().StartsWith("Bearer ", StringComparison.Ordinal)
    }));
    Console.Out.Flush();
    await next();
});
app.MapPost("/Accounts/authenticate", () => Results.Json(new
{
    username = "admin",
    enabled = true,
    jwtToken = "stage5.fixture.jwt",
    refreshToken = "stage5.fixture.refresh"
}));
app.MapPost("/Accounts/revoke-token", () => Results.Ok());
app.MapPost("/Accounts/refresh-token", () => Results.Json(new
{
    username = "admin",
    enabled = true,
    jwtToken = "stage5.fixture.jwt.refreshed",
    refreshToken = "stage5.fixture.refresh"
}));
app.MapHub<FixtureHub>("/SignalR");
app.MapFallback((HttpContext context) =>
{
    context.Response.ContentType = "application/json";
    return context.Response.WriteAsync("[]");
});
await app.RunAsync();

public sealed class FixtureHub : Hub;
