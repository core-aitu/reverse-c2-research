using System.Reflection;
using System.Text.Json;
using SmartComponents.LocalEmbeddings;

using var embedder=new LocalEmbedder();
var a=embedder.Embed("A short guide to growing tomatoes.").Values.ToArray();
var b=embedder.Embed("A short guide to growing tomatoes.").Values.ToArray();
var c=embedder.Embed("The compressor manual lists mechanical parts.").Values.ToArray();
int dimensions=embedder.Dimensions;
void Test(string scenario,bool passed)
{
    Console.WriteLine(JsonSerializer.Serialize(new{scenario,passed,dimensions}));
    if(!passed) Environment.ExitCode=1;
}
Test("dimension",dimensions==384 && a.Length==dimensions && c.Length==dimensions);
Test("finite",a.All(float.IsFinite) && c.All(float.IsFinite));
Test("identical-input",a.SequenceEqual(b));
Test("changed-input",!a.SequenceEqual(c));
var server=Assembly.LoadFrom("/server/SpecterInsight.Server.dll");
var type=server.GetType("SpecterInsight.Server.Services.Documentation.DocumentationService",true)!;
var similarity=type.GetMethod("CosineSimilarity",BindingFlags.Static|BindingFlags.NonPublic|BindingFlags.Public)!;

double Score(float[] x,float[] y)=>Convert.ToDouble(similarity.Invoke(null,[x,y]));
Test("server-cosine-self",Math.Abs(Score(a,b)-1)<0.0001);
Test("server-cosine-change",Score(a,c)<Score(a,b));
Test("server-cosine-orthogonal",Math.Abs(Score([1f,0f],[0f,1f]))<1e-6);
var chunks=type.GetMethod("ChunkByHeadings",BindingFlags.Static|BindingFlags.NonPublic|BindingFlags.Public)!;
(string,string)[] Split(string text)=>((IEnumerable<(string,string)>)chunks.Invoke(null,[text,"Fallback"])!).ToArray();
Test("markdown-empty",Split("").SequenceEqual(new[]{("Fallback","")}));
Test("markdown-fallback",Split("A plain paragraph.").SequenceEqual(new[]{("Fallback","A plain paragraph.")}));
Test("markdown-headings",Split("# One\nA short paragraph.\n## Two\nSecond paragraph.").SequenceEqual(
    new[]{("One","# One\nA short paragraph."),("Two","## Two\nSecond paragraph.")}));
