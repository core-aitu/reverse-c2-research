// IL code: SpecterInsight.Obfuscation
.method family hidebysig virtual 
	instance class SpecterInsight.Obfuscation.Classes.ByteTransformResult OnTransform (
		class SpecterInsight.Obfuscation.Classes.StringTransformResult input
	) cil managed 
{
	// Method begins at RVA 0x4c254
	// Header size: 12
	// Code size: 342 (0x156)
	.maxstack 27
	.locals init (
		[0] class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.SyntaxTree,
		[1] class [Microsoft.CodeAnalysis.CSharp]Microsoft.CodeAnalysis.CSharp.CSharpCompilationOptions,
		[2] class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.MetadataReference[],
		[3] class [Microsoft.CodeAnalysis.CSharp]Microsoft.CodeAnalysis.CSharp.CSharpCompilation,
		[4] valuetype [System.Runtime]System.Threading.CancellationToken,
		[5] valuetype [System.Runtime]System.Nullable`1<bool>,
		[6] valuetype [System.Collections.Immutable]System.Collections.Immutable.ImmutableArray`1<uint8>,
		[7] class [System.Runtime]System.IO.MemoryStream,
		[8] class [System.Runtime]System.IO.MemoryStream,
		[9] class [System.Runtime]System.IO.MemoryStream,
		[10] class [System.Runtime]System.IO.Stream,
		[11] class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.Emit.EmitResult,
		[12] class SpecterInsight.Obfuscation.Classes.ByteTransformResult
	)

	IL_0000: ldarg.0
	IL_0001: ldarg.1
	IL_0002: callvirt instance string SpecterInsight.Obfuscation.Classes.StringTransformResult::get_Contents()
	IL_0007: call instance string SpecterInsight.Obfuscation.CSharp.OutputTransforms.CSharpCompilerTransformer::ApplyAssemblyAttributes(string)
	IL_000c: ldnull
	IL_000d: ldstr ""
	IL_0012: ldnull
	IL_0013: ldloca.s 4
	IL_0015: initobj [System.Runtime]System.Threading.CancellationToken
	IL_001b: ldloc.s 4
	IL_001d: call class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.SyntaxTree [Microsoft.CodeAnalysis.CSharp]Microsoft.CodeAnalysis.CSharp.CSharpSyntaxTree::ParseText(string, class [Microsoft.CodeAnalysis.CSharp]Microsoft.CodeAnalysis.CSharp.CSharpParseOptions, string, class [System.Runtime]System.Text.Encoding, valuetype [System.Runtime]System.Threading.CancellationToken)
	IL_0022: stloc.0
	IL_0023: ldarg.0
	IL_0024: call instance valuetype [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.OutputKind SpecterInsight.Obfuscation.CSharp.OutputTransforms.CSharpCompilerTransformer::GetOutputKind()
	IL_0029: ldc.i4.0
	IL_002a: ldnull
	IL_002b: ldnull
	IL_002c: ldnull
	IL_002d: ldnull
	IL_002e: ldc.i4.1
	IL_002f: ldc.i4.0
	IL_0030: ldc.i4.1
	IL_0031: ldnull
	IL_0032: ldnull
	IL_0033: ldloca.s 5
	IL_0035: initobj valuetype [System.Runtime]System.Nullable`1<bool>
	IL_003b: ldloca.s 6
	IL_003d: initobj valuetype [System.Collections.Immutable]System.Collections.Immutable.ImmutableArray`1<uint8>
	IL_0043: ldloc.s 6
	IL_0045: ldloc.s 5
	IL_0047: ldc.i4.0
	IL_0048: ldc.i4.0
	IL_0049: ldc.i4.4
	IL_004a: ldnull
	IL_004b: ldc.i4.1
	IL_004c: ldc.i4.0
	IL_004d: ldnull
	IL_004e: ldnull
	IL_004f: ldnull
	IL_0050: ldnull
	IL_0051: ldnull
	IL_0052: ldc.i4.0
	IL_0053: ldc.i4.0
	IL_0054: ldc.i4.0
	IL_0055: newobj instance void [Microsoft.CodeAnalysis.CSharp]Microsoft.CodeAnalysis.CSharp.CSharpCompilationOptions::.ctor(valuetype [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.OutputKind, bool, string, string, string, class [System.Runtime]System.Collections.Generic.IEnumerable`1<string>, valuetype [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.OptimizationLevel, bool, bool, string, string, valuetype [System.Collections.Immutable]System.Collections.Immutable.ImmutableArray`1<uint8>, valuetype [System.Runtime]System.Nullable`1<bool>, valuetype [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.Platform, valuetype [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.ReportDiagnostic, int32, class [System.Runtime]System.Collections.Generic.IEnumerable`1<valuetype [System.Runtime]System.Collections.Generic.KeyValuePair`2<string, valuetype [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.ReportDiagnostic>>, bool, bool, class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.XmlReferenceResolver, class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.SourceReferenceResolver, class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.MetadataReferenceResolver, class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.AssemblyIdentityComparer, class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.StrongNameProvider, bool, valuetype [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.MetadataImportOptions, valuetype [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.NullableContextOptions)
	IL_005a: stloc.1
	IL_005b: ldarg.0
	IL_005c: call instance valuetype SpecterInsight.Obfuscation.CSharp.OutputTransforms.CSharpCompilerFrameworkVersion SpecterInsight.Obfuscation.CSharp.OutputTransforms.CSharpCompilerTransformer::get_FrameworkVersion()
	IL_0061: call valuetype [System.Collections.Immutable]System.Collections.Immutable.ImmutableArray`1<class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.MetadataReference> SpecterInsight.Obfuscation.Classes.References::GetReferences(valuetype SpecterInsight.Obfuscation.CSharp.OutputTransforms.CSharpCompilerFrameworkVersion)
	IL_0066: call !!0[] [System.Collections.Immutable]System.Linq.ImmutableArrayExtensions::ToArray<class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.MetadataReference>(valuetype [System.Collections.Immutable]System.Collections.Immutable.ImmutableArray`1<!!0>)
	IL_006b: stloc.2
	IL_006c: ldarg.0
	IL_006d: call instance string SpecterInsight.Obfuscation.CSharp.OutputTransforms.CSharpCompilerTransformer::get_InternalFileName()
	IL_0072: call string [System.Runtime]System.IO.Path::GetFileNameWithoutExtension(string)
	IL_0077: ldnull
	IL_0078: ldnull
	IL_0079: ldnull
	IL_007a: call class [Microsoft.CodeAnalysis.CSharp]Microsoft.CodeAnalysis.CSharp.CSharpCompilation [Microsoft.CodeAnalysis.CSharp]Microsoft.CodeAnalysis.CSharp.CSharpCompilation::Create(string, class [System.Runtime]System.Collections.Generic.IEnumerable`1<class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.SyntaxTree>, class [System.Runtime]System.Collections.Generic.IEnumerable`1<class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.MetadataReference>, class [Microsoft.CodeAnalysis.CSharp]Microsoft.CodeAnalysis.CSharp.CSharpCompilationOptions)
	IL_007f: ldloc.1
	IL_0080: callvirt instance class [Microsoft.CodeAnalysis.CSharp]Microsoft.CodeAnalysis.CSharp.CSharpCompilation [Microsoft.CodeAnalysis.CSharp]Microsoft.CodeAnalysis.CSharp.CSharpCompilation::WithOptions(class [Microsoft.CodeAnalysis.CSharp]Microsoft.CodeAnalysis.CSharp.CSharpCompilationOptions)
	IL_0085: ldloc.2
	IL_0086: callvirt instance class [Microsoft.CodeAnalysis.CSharp]Microsoft.CodeAnalysis.CSharp.CSharpCompilation [Microsoft.CodeAnalysis.CSharp]Microsoft.CodeAnalysis.CSharp.CSharpCompilation::AddReferences(class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.MetadataReference[])
	IL_008b: ldc.i4.1
	IL_008c: newarr [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.SyntaxTree
	IL_0091: dup
	IL_0092: ldc.i4.0
	IL_0093: ldloc.0
	IL_0094: stelem.ref
	IL_0095: callvirt instance class [Microsoft.CodeAnalysis.CSharp]Microsoft.CodeAnalysis.CSharp.CSharpCompilation [Microsoft.CodeAnalysis.CSharp]Microsoft.CodeAnalysis.CSharp.CSharpCompilation::AddSyntaxTrees(class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.SyntaxTree[])
	IL_009a: ldarg.0
	IL_009b: call instance string SpecterInsight.Obfuscation.CSharp.OutputTransforms.CSharpCompilerTransformer::get_InternalFileName()
	IL_00a0: call string [System.Runtime]System.IO.Path::GetFileNameWithoutExtension(string)
	IL_00a5: callvirt instance class [Microsoft.CodeAnalysis.CSharp]Microsoft.CodeAnalysis.CSharp.CSharpCompilation [Microsoft.CodeAnalysis.CSharp]Microsoft.CodeAnalysis.CSharp.CSharpCompilation::WithAssemblyName(string)
	IL_00aa: stloc.3
	IL_00ab: newobj instance void [System.Runtime]System.IO.MemoryStream::.ctor()
	IL_00b0: stloc.s 7
	.try
	{
		IL_00b2: newobj instance void [System.Runtime]System.IO.MemoryStream::.ctor()
		IL_00b7: stloc.s 8
		.try
		{
			IL_00b9: ldarg.0
			IL_00ba: call instance class [System.Runtime]System.IO.MemoryStream SpecterInsight.Obfuscation.CSharp.OutputTransforms.CSharpCompilerTransformer::GetApplicationManifest()
			IL_00bf: stloc.s 9
			.try
			{
				IL_00c1: ldloc.3
				IL_00c2: ldc.i4.1
				IL_00c3: ldc.i4.0
				IL_00c4: ldloc.s 9
				IL_00c6: ldnull
				IL_00c7: callvirt instance class [System.Runtime]System.IO.Stream [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.Compilation::CreateDefaultWin32Resources(bool, bool, class [System.Runtime]System.IO.Stream, class [System.Runtime]System.IO.Stream)
				IL_00cc: stloc.s 10
				.try
				{
					IL_00ce: ldloc.3
					IL_00cf: ldloc.s 7
					IL_00d1: ldloc.s 8
					IL_00d3: ldnull
					IL_00d4: ldloc.s 10
					IL_00d6: ldnull
					IL_00d7: ldnull
					IL_00d8: ldnull
					IL_00d9: ldnull
					IL_00da: ldnull
					IL_00db: ldnull
					IL_00dc: ldloca.s 4
					IL_00de: initobj [System.Runtime]System.Threading.CancellationToken
					IL_00e4: ldloc.s 4
					IL_00e6: callvirt instance class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.Emit.EmitResult [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.Compilation::Emit(class [System.Runtime]System.IO.Stream, class [System.Runtime]System.IO.Stream, class [System.Runtime]System.IO.Stream, class [System.Runtime]System.IO.Stream, class [System.Runtime]System.Collections.Generic.IEnumerable`1<class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.ResourceDescription>, class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.Emit.EmitOptions, class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.IMethodSymbol, class [System.Runtime]System.IO.Stream, class [System.Runtime]System.Collections.Generic.IEnumerable`1<class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.EmbeddedText>, class [System.Runtime]System.IO.Stream, valuetype [System.Runtime]System.Threading.CancellationToken)
					IL_00eb: stloc.s 11
					IL_00ed: ldloc.s 11
					IL_00ef: callvirt instance bool [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.Emit.EmitResult::get_Success()
					IL_00f4: brtrue.s IL_0109

					IL_00f6: ldloc.s 11
					IL_00f8: callvirt instance valuetype [System.Collections.Immutable]System.Collections.Immutable.ImmutableArray`1<class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.Diagnostic> [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.Emit.EmitResult::get_Diagnostics()
					IL_00fd: ldarg.1
					IL_00fe: callvirt instance string SpecterInsight.Obfuscation.Classes.StringTransformResult::get_Contents()
					IL_0103: newobj instance void SpecterInsight.Obfuscation.Classes.CSharpCompilationException::.ctor(valuetype [System.Collections.Immutable]System.Collections.Immutable.ImmutableArray`1<class [Microsoft.CodeAnalysis]Microsoft.CodeAnalysis.Diagnostic>, string)
					IL_0108: throw

					IL_0109: ldloc.s 7
					IL_010b: callvirt instance uint8[] [System.Runtime]System.IO.MemoryStream::ToArray()
					IL_0110: ldc.i4.1
					IL_0111: newarr SpecterInsight.Obfuscation.Classes.TransformResult
					IL_0116: dup
					IL_0117: ldc.i4.0
					IL_0118: ldarg.1
					IL_0119: stelem.ref
					IL_011a: newobj instance void SpecterInsight.Obfuscation.Classes.ByteTransformResult::.ctor(uint8[], class SpecterInsight.Obfuscation.Classes.TransformResult[])
					IL_011f: stloc.s 12
					IL_0121: leave.s IL_0153
				} // end .try
				finally
				{
					IL_0123: ldloc.s 10
					IL_0125: brfalse.s IL_012e

					IL_0127: ldloc.s 10
					IL_0129: callvirt instance void [System.Runtime]System.IDisposable::Dispose()

					IL_012e: endfinally
				} // end handler
			} // end .try
			finally
			{
				IL_012f: ldloc.s 9
				IL_0131: brfalse.s IL_013a

				IL_0133: ldloc.s 9
				IL_0135: callvirt instance void [System.Runtime]System.IDisposable::Dispose()

				IL_013a: endfinally
			} // end handler
		} // end .try
		finally
		{
			IL_013b: ldloc.s 8
			IL_013d: brfalse.s IL_0146

			IL_013f: ldloc.s 8
			IL_0141: callvirt instance void [System.Runtime]System.IDisposable::Dispose()

			IL_0146: endfinally
		} // end handler
	} // end .try
	finally
	{
		IL_0147: ldloc.s 7
		IL_0149: brfalse.s IL_0152

		IL_014b: ldloc.s 7
		IL_014d: callvirt instance void [System.Runtime]System.IDisposable::Dispose()

		IL_0152: endfinally
	} // end handler

	IL_0153: ldloc.s 12
	IL_0155: ret
} // end of method CSharpCompilerTransformer::OnTransform
