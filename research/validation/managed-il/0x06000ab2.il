// IL code: SpecterInsight.Obfuscation
.method family hidebysig virtual 
	instance class SpecterInsight.Obfuscation.Classes.ByteTransformResult OnTransform (
		class SpecterInsight.Obfuscation.Classes.StringTransformResult input
	) cil managed 
{
	// Method begins at RVA 0x4bb14
	// Header size: 12
	// Code size: 233 (0xe9)
	.maxstack 5
	.locals init (
		[0] class SpecterInsight.Obfuscation.Classes.TemporaryFile,
		[1] class SpecterInsight.Obfuscation.Classes.TemporaryFile,
		[2] class [System.Collections]System.Collections.Generic.List`1<string>,
		[3] class [MedallionShell]Medallion.Shell.Command,
		[4] uint8[],
		[5] class SpecterInsight.Obfuscation.Classes.ByteTransformResult,
		[6] class [System.Runtime]System.Exception
	)

	IL_0000: ldarg.0
	IL_0001: call instance string SpecterInsight.Obfuscation.CSharp.OutputTransforms.BFlatCompilerTransformer::get_TemporaryFolder()
	IL_0006: ldstr ".exe"
	IL_000b: newobj instance void SpecterInsight.Obfuscation.Classes.TemporaryFile::.ctor(string, string)
	IL_0010: stloc.0
	.try
	{
		IL_0011: ldarg.0
		IL_0012: call instance string SpecterInsight.Obfuscation.CSharp.OutputTransforms.BFlatCompilerTransformer::get_TemporaryFolder()
		IL_0017: ldstr ".cs"
		IL_001c: newobj instance void SpecterInsight.Obfuscation.Classes.TemporaryFile::.ctor(string, string)
		IL_0021: stloc.1
		.try
		{
			IL_0022: ldloc.1
			IL_0023: callvirt instance string SpecterInsight.Obfuscation.Classes.TemporaryFile::get_FullName()
			IL_0028: ldarg.1
			IL_0029: callvirt instance string SpecterInsight.Obfuscation.Classes.StringTransformResult::get_Contents()
			IL_002e: call void [System.Runtime]System.IO.File::WriteAllText(string, string)
			IL_0033: ldarg.0
			IL_0034: ldloc.0
			IL_0035: callvirt instance string SpecterInsight.Obfuscation.Classes.TemporaryFile::get_FullName()
			IL_003a: ldloc.1
			IL_003b: callvirt instance string SpecterInsight.Obfuscation.Classes.TemporaryFile::get_FullName()
			IL_0040: call instance class [System.Collections]System.Collections.Generic.List`1<string> SpecterInsight.Obfuscation.CSharp.OutputTransforms.BFlatCompilerTransformer::GetArguments(string, string)
			IL_0045: stloc.2
			IL_0046: ldarg.0
			IL_0047: ldloc.2
			IL_0048: call instance class [MedallionShell]Medallion.Shell.Command SpecterInsight.Obfuscation.CSharp.OutputTransforms.BFlatCompilerTransformer::Run(class [System.Runtime]System.Collections.Generic.IList`1<string>)
			IL_004d: stloc.3
			IL_004e: ldloc.3
			IL_004f: callvirt instance void [MedallionShell]Medallion.Shell.Command::Wait()
			IL_0054: ldloc.3
			IL_0055: callvirt instance class [MedallionShell]Medallion.Shell.CommandResult [MedallionShell]Medallion.Shell.Command::get_Result()
			IL_005a: callvirt instance bool [MedallionShell]Medallion.Shell.CommandResult::get_Success()
			IL_005f: brtrue.s IL_0082

			IL_0061: ldstr "Output:\r\n{0}\r\n\r\nErrors: {1}"
			IL_0066: ldloc.3
			IL_0067: callvirt instance class [MedallionShell]Medallion.Shell.CommandResult [MedallionShell]Medallion.Shell.Command::get_Result()
			IL_006c: callvirt instance string [MedallionShell]Medallion.Shell.CommandResult::get_StandardOutput()
			IL_0071: ldloc.3
			IL_0072: callvirt instance class [MedallionShell]Medallion.Shell.CommandResult [MedallionShell]Medallion.Shell.Command::get_Result()
			IL_0077: callvirt instance string [MedallionShell]Medallion.Shell.CommandResult::get_StandardError()
			IL_007c: call string [System.Runtime]System.String::Format(string, object, object)
			IL_0081: pop

			IL_0082: nop
			.try
			{
				IL_0083: ldloc.0
				IL_0084: callvirt instance string SpecterInsight.Obfuscation.Classes.TemporaryFile::get_FullName()
				IL_0089: call uint8[] [System.Runtime]System.IO.File::ReadAllBytes(string)
				IL_008e: stloc.s 4
				IL_0090: ldloc.s 4
				IL_0092: brfalse.s IL_0099

				IL_0094: ldloc.s 4
				IL_0096: ldlen
				IL_0097: brtrue.s IL_00a4

				IL_0099: ldstr "The contents of the exported executable were null or empty."
				IL_009e: newobj instance void [System.Runtime]System.Exception::.ctor(string)
				IL_00a3: throw

				IL_00a4: ldloc.s 4
				IL_00a6: ldc.i4.1
				IL_00a7: newarr SpecterInsight.Obfuscation.Classes.TransformResult
				IL_00ac: dup
				IL_00ad: ldc.i4.0
				IL_00ae: ldarg.1
				IL_00af: stelem.ref
				IL_00b0: newobj instance void SpecterInsight.Obfuscation.Classes.ByteTransformResult::.ctor(uint8[], class SpecterInsight.Obfuscation.Classes.TransformResult[])
				IL_00b5: stloc.s 5
				IL_00b7: leave.s IL_00e6
			} // end .try
			catch [System.Runtime]System.Exception
			{
				IL_00b9: stloc.s 6
				IL_00bb: ldstr "Failed to compile the provided code using the native AOT compiler.\r\n\r\nException:  {0}"
				IL_00c0: ldloc.s 6
				IL_00c2: callvirt instance string [System.Runtime]System.Exception::get_Message()
				IL_00c7: call string [System.Runtime]System.String::Format(string, object)
				IL_00cc: newobj instance void [System.Runtime]System.Exception::.ctor(string)
				IL_00d1: throw
			} // end handler
		} // end .try
		finally
		{
			IL_00d2: ldloc.1
			IL_00d3: brfalse.s IL_00db

			IL_00d5: ldloc.1
			IL_00d6: callvirt instance void [System.Runtime]System.IDisposable::Dispose()

			IL_00db: endfinally
		} // end handler
	} // end .try
	finally
	{
		IL_00dc: ldloc.0
		IL_00dd: brfalse.s IL_00e5

		IL_00df: ldloc.0
		IL_00e0: callvirt instance void [System.Runtime]System.IDisposable::Dispose()

		IL_00e5: endfinally
	} // end handler

	IL_00e6: ldloc.s 5
	IL_00e8: ret
} // end of method BFlatCompilerTransformer::OnTransform
