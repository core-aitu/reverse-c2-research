// IL code: SpecterInsight.Obfuscation
.method family hidebysig virtual 
	instance class SpecterInsight.Obfuscation.Classes.ByteTransformResult OnTransform (
		class SpecterInsight.Obfuscation.Classes.StringTransformResult input
	) cil managed 
{
	// Method begins at RVA 0x4c914
	// Header size: 12
	// Code size: 414 (0x19e)
	.maxstack 5
	.locals init (
		[0] class SpecterInsight.Obfuscation.Classes.TemporaryFile,
		[1] class SpecterInsight.Obfuscation.Classes.TemporaryFile,
		[2] class [System.Collections]System.Collections.Generic.List`1<string>,
		[3] class [System.Collections]System.Collections.Generic.List`1<string>,
		[4] class [MedallionShell]Medallion.Shell.Command,
		[5] string,
		[6] valuetype [System.Collections]System.Collections.Generic.Dictionary`2/Enumerator<string, uint8[]>,
		[7] valuetype [System.Runtime]System.Collections.Generic.KeyValuePair`2<string, uint8[]>,
		[8] string,
		[9] uint8[],
		[10] class SpecterInsight.Obfuscation.Classes.ByteTransformResult,
		[11] class [System.Runtime]System.Exception,
		[12] valuetype [System.Collections]System.Collections.Generic.List`1/Enumerator<string>,
		[13] string
	)

	IL_0000: ldarg.0
	IL_0001: call instance string SpecterInsight.Obfuscation.CSharp.OutputTransforms.ZigCompilerTransformer::get_TemporaryFolder()
	IL_0006: ldstr ".zig"
	IL_000b: newobj instance void SpecterInsight.Obfuscation.Classes.TemporaryFile::.ctor(string, string)
	IL_0010: stloc.0
	.try
	{
		IL_0011: ldarg.0
		IL_0012: call instance string SpecterInsight.Obfuscation.CSharp.OutputTransforms.ZigCompilerTransformer::get_TemporaryFolder()
		IL_0017: ldstr ".bin"
		IL_001c: newobj instance void SpecterInsight.Obfuscation.Classes.TemporaryFile::.ctor(string, string)
		IL_0021: stloc.1
		.try
		{
			IL_0022: ldloc.0
			IL_0023: callvirt instance string SpecterInsight.Obfuscation.Classes.TemporaryFile::get_FullName()
			IL_0028: ldarg.1
			IL_0029: callvirt instance string SpecterInsight.Obfuscation.Classes.StringTransformResult::get_Contents()
			IL_002e: call void [System.Runtime]System.IO.File::WriteAllText(string, string)
			IL_0033: newobj instance void class [System.Collections]System.Collections.Generic.List`1<string>::.ctor()
			IL_0038: stloc.2
			IL_0039: ldarg.1
			IL_003a: callvirt instance class [System.Collections]System.Collections.Generic.Dictionary`2<string, uint8[]> SpecterInsight.Obfuscation.Classes.StringTransformResult::get_EmbeddedFiles()
			IL_003f: brfalse.s IL_00a5

			IL_0041: ldloc.0
			IL_0042: callvirt instance string SpecterInsight.Obfuscation.Classes.TemporaryFile::get_FullName()
			IL_0047: call string [System.Runtime]System.IO.Path::GetDirectoryName(string)
			IL_004c: stloc.s 5
			IL_004e: ldarg.1
			IL_004f: callvirt instance class [System.Collections]System.Collections.Generic.Dictionary`2<string, uint8[]> SpecterInsight.Obfuscation.Classes.StringTransformResult::get_EmbeddedFiles()
			IL_0054: callvirt instance valuetype [System.Collections]System.Collections.Generic.Dictionary`2/Enumerator<!0, !1> class [System.Collections]System.Collections.Generic.Dictionary`2<string, uint8[]>::GetEnumerator()
			IL_0059: stloc.s 6
			.try
			{
				IL_005b: br.s IL_008c
				// loop start (head: IL_008c)
					IL_005d: ldloca.s 6
					IL_005f: call instance valuetype [System.Runtime]System.Collections.Generic.KeyValuePair`2<!0, !1> valuetype [System.Collections]System.Collections.Generic.Dictionary`2/Enumerator<string, uint8[]>::get_Current()
					IL_0064: stloc.s 7
					IL_0066: ldloc.s 5
					IL_0068: ldloca.s 7
					IL_006a: call instance !0 valuetype [System.Runtime]System.Collections.Generic.KeyValuePair`2<string, uint8[]>::get_Key()
					IL_006f: call string [System.Runtime]System.IO.Path::Combine(string, string)
					IL_0074: stloc.s 8
					IL_0076: ldloc.s 8
					IL_0078: ldloca.s 7
					IL_007a: call instance !1 valuetype [System.Runtime]System.Collections.Generic.KeyValuePair`2<string, uint8[]>::get_Value()
					IL_007f: call void [System.Runtime]System.IO.File::WriteAllBytes(string, uint8[])
					IL_0084: ldloc.2
					IL_0085: ldloc.s 8
					IL_0087: callvirt instance void class [System.Collections]System.Collections.Generic.List`1<string>::Add(!0)

					IL_008c: ldloca.s 6
					IL_008e: call instance bool valuetype [System.Collections]System.Collections.Generic.Dictionary`2/Enumerator<string, uint8[]>::MoveNext()
					IL_0093: brtrue.s IL_005d
				// end loop

				IL_0095: leave.s IL_00a5
			} // end .try
			finally
			{
				IL_0097: ldloca.s 6
				IL_0099: constrained. valuetype [System.Collections]System.Collections.Generic.Dictionary`2/Enumerator<string, uint8[]>
				IL_009f: callvirt instance void [System.Runtime]System.IDisposable::Dispose()
				IL_00a4: endfinally
			} // end handler

			IL_00a5: ldarg.0
			IL_00a6: ldloc.0
			IL_00a7: callvirt instance string SpecterInsight.Obfuscation.Classes.TemporaryFile::get_FullName()
			IL_00ac: ldloc.1
			IL_00ad: callvirt instance string SpecterInsight.Obfuscation.Classes.TemporaryFile::get_FullName()
			IL_00b2: call instance class [System.Collections]System.Collections.Generic.List`1<string> SpecterInsight.Obfuscation.CSharp.OutputTransforms.ZigCompilerTransformer::GetArguments(string, string)
			IL_00b7: stloc.3
			IL_00b8: ldarg.0
			IL_00b9: ldloc.3
			IL_00ba: call instance class [MedallionShell]Medallion.Shell.Command SpecterInsight.Obfuscation.CSharp.OutputTransforms.ZigCompilerTransformer::Run(class [System.Runtime]System.Collections.Generic.IList`1<string>)
			IL_00bf: stloc.s 4
			IL_00c1: ldloc.s 4
			IL_00c3: callvirt instance void [MedallionShell]Medallion.Shell.Command::Wait()
			IL_00c8: ldloc.s 4
			IL_00ca: callvirt instance class [MedallionShell]Medallion.Shell.CommandResult [MedallionShell]Medallion.Shell.Command::get_Result()
			IL_00cf: callvirt instance bool [MedallionShell]Medallion.Shell.CommandResult::get_Success()
			IL_00d4: brtrue.s IL_00fe

			IL_00d6: ldstr "Zig compilation failed.\n\nOutput:\n{0}\n\nErrors:\n{1}"
			IL_00db: ldloc.s 4
			IL_00dd: callvirt instance class [MedallionShell]Medallion.Shell.CommandResult [MedallionShell]Medallion.Shell.Command::get_Result()
			IL_00e2: callvirt instance string [MedallionShell]Medallion.Shell.CommandResult::get_StandardOutput()
			IL_00e7: ldloc.s 4
			IL_00e9: callvirt instance class [MedallionShell]Medallion.Shell.CommandResult [MedallionShell]Medallion.Shell.Command::get_Result()
			IL_00ee: callvirt instance string [MedallionShell]Medallion.Shell.CommandResult::get_StandardError()
			IL_00f3: call string [System.Runtime]System.String::Format(string, object, object)
			IL_00f8: newobj instance void [System.Runtime]System.Exception::.ctor(string)
			IL_00fd: throw

			IL_00fe: nop
			.try
			{
				.try
				{
					IL_00ff: ldloc.1
					IL_0100: callvirt instance string SpecterInsight.Obfuscation.Classes.TemporaryFile::get_FullName()
					IL_0105: call uint8[] [System.Runtime]System.IO.File::ReadAllBytes(string)
					IL_010a: stloc.s 9
					IL_010c: ldloc.s 9
					IL_010e: brfalse.s IL_0115

					IL_0110: ldloc.s 9
					IL_0112: ldlen
					IL_0113: brtrue.s IL_0120

					IL_0115: ldstr "The compiled Zig output was empty."
					IL_011a: newobj instance void [System.Runtime]System.Exception::.ctor(string)
					IL_011f: throw

					IL_0120: ldloc.s 9
					IL_0122: ldc.i4.1
					IL_0123: newarr SpecterInsight.Obfuscation.Classes.TransformResult
					IL_0128: dup
					IL_0129: ldc.i4.0
					IL_012a: ldarg.1
					IL_012b: stelem.ref
					IL_012c: newobj instance void SpecterInsight.Obfuscation.Classes.ByteTransformResult::.ctor(uint8[], class SpecterInsight.Obfuscation.Classes.TransformResult[])
					IL_0131: stloc.s 10
					IL_0133: leave.s IL_019b
				} // end .try
				catch [System.Runtime]System.Exception
				{
					IL_0135: stloc.s 11
					IL_0137: ldstr "Failed to read Zig compiler output: {0}"
					IL_013c: ldloc.s 11
					IL_013e: callvirt instance string [System.Runtime]System.Exception::get_Message()
					IL_0143: call string [System.Runtime]System.String::Format(string, object)
					IL_0148: newobj instance void [System.Runtime]System.Exception::.ctor(string)
					IL_014d: throw
				} // end handler
			} // end .try
			finally
			{
				IL_014e: ldloc.2
				IL_014f: callvirt instance valuetype [System.Collections]System.Collections.Generic.List`1/Enumerator<!0> class [System.Collections]System.Collections.Generic.List`1<string>::GetEnumerator()
				IL_0154: stloc.s 12
				.try
				{
					IL_0156: br.s IL_016d
					// loop start (head: IL_016d)
						IL_0158: ldloca.s 12
						IL_015a: call instance !0 valuetype [System.Collections]System.Collections.Generic.List`1/Enumerator<string>::get_Current()
						IL_015f: stloc.s 13
						.try
						{
							IL_0161: ldloc.s 13
							IL_0163: call void [System.Runtime]System.IO.File::Delete(string)
							IL_0168: leave.s IL_016d
						} // end .try
						catch [System.Runtime]System.Object
						{
							IL_016a: pop
							IL_016b: leave.s IL_016d
						} // end handler

						IL_016d: ldloca.s 12
						IL_016f: call instance bool valuetype [System.Collections]System.Collections.Generic.List`1/Enumerator<string>::MoveNext()
						IL_0174: brtrue.s IL_0158
					// end loop

					IL_0176: leave.s IL_0186
				} // end .try
				finally
				{
					IL_0178: ldloca.s 12
					IL_017a: constrained. valuetype [System.Collections]System.Collections.Generic.List`1/Enumerator<string>
					IL_0180: callvirt instance void [System.Runtime]System.IDisposable::Dispose()
					IL_0185: endfinally
				} // end handler

				IL_0186: endfinally
			} // end handler
		} // end .try
		finally
		{
			IL_0187: ldloc.1
			IL_0188: brfalse.s IL_0190

			IL_018a: ldloc.1
			IL_018b: callvirt instance void [System.Runtime]System.IDisposable::Dispose()

			IL_0190: endfinally
		} // end handler
	} // end .try
	finally
	{
		IL_0191: ldloc.0
		IL_0192: brfalse.s IL_019a

		IL_0194: ldloc.0
		IL_0195: callvirt instance void [System.Runtime]System.IDisposable::Dispose()

		IL_019a: endfinally
	} // end handler

	IL_019b: ldloc.s 10
	IL_019d: ret
} // end of method ZigCompilerTransformer::OnTransform
