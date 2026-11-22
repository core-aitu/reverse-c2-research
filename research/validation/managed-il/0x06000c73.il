// IL code: SpecterInsight.Obfuscation
.method family final hidebysig virtual 
	instance class SpecterInsight.Obfuscation.Classes.StringTransformResult OnTransform (
		class SpecterInsight.Obfuscation.Classes.StringTransformResult input
	) cil managed 
{
	// Method begins at RVA 0x5260c
	// Header size: 12
	// Code size: 328 (0x148)
	.maxstack 5
	.locals init (
		[0] class SpecterInsight.Obfuscation.Classes.StringTransformResult,
		[1] int32,
		[2] class [System.Management.Automation]System.Management.Automation.Language.Ast,
		[3] class [System.Management.Automation]System.Management.Automation.Language.Ast[],
		[4] class [System.Management.Automation]System.Management.Automation.Language.Ast[],
		[5] class [System.Runtime]System.Text.StringBuilder,
		[6] int32,
		[7] int32,
		[8] class [System.Management.Automation]System.Management.Automation.Language.Ast,
		[9] string
	)

	IL_0000: ldarg.0
	IL_0001: ldarg.0
	IL_0002: call instance class [System.Text.RegularExpressions]System.Text.RegularExpressions.Regex[] SpecterInsight.Obfuscation.PowerShell.PwshAstTransformer::CompileFilters()
	IL_0007: stfld class [System.Text.RegularExpressions]System.Text.RegularExpressions.Regex[] SpecterInsight.Obfuscation.PowerShell.PwshAstTransformer::_filters
	IL_000c: ldarg.1
	IL_000d: stloc.0
	IL_000e: ldc.i4.0
	IL_000f: stloc.1
	IL_0010: br IL_013a
	// loop start (head: IL_013a)
		IL_0015: ldarg.0
		IL_0016: ldloc.0
		IL_0017: callvirt instance string SpecterInsight.Obfuscation.Classes.StringTransformResult::get_Contents()
		IL_001c: call instance class [System.Management.Automation]System.Management.Automation.Language.Ast SpecterInsight.Obfuscation.PowerShell.PwshAstTransformer::GetRootAst(string)
		IL_0021: stloc.2
		IL_0022: ldarg.0
		IL_0023: ldloc.2
		IL_0024: ldarg.0
		IL_0025: ldfld class [System.Text.RegularExpressions]System.Text.RegularExpressions.Regex[] SpecterInsight.Obfuscation.PowerShell.PwshAstTransformer::_filters
		IL_002a: call instance class [System.Management.Automation]System.Management.Automation.Language.Ast[] SpecterInsight.Obfuscation.PowerShell.PwshAstTransformer::GetSupportedNodes(class [System.Management.Automation]System.Management.Automation.Language.Ast, class [System.Text.RegularExpressions]System.Text.RegularExpressions.Regex[])
		IL_002f: stloc.3
		IL_0030: ldarg.0
		IL_0031: ldloc.2
		IL_0032: call instance class [System.Management.Automation]System.Management.Automation.Language.Ast[] SpecterInsight.Obfuscation.PowerShell.PwshAstTransformer::ExtractAllNodes(class [System.Management.Automation]System.Management.Automation.Language.Ast)
		IL_0037: stloc.s 4
		IL_0039: ldloc.3
		IL_003a: ldlen
		IL_003b: brfalse IL_0146

		IL_0040: ldarg.0
		IL_0041: ldloc.0
		IL_0042: callvirt instance string SpecterInsight.Obfuscation.Classes.StringTransformResult::get_Contents()
		IL_0047: ldloc.2
		IL_0048: ldloc.s 4
		IL_004a: callvirt instance void SpecterInsight.Obfuscation.PowerShell.PwshAstTransformer::PreTransform(string, class [System.Management.Automation]System.Management.Automation.Language.Ast, class [System.Management.Automation]System.Management.Automation.Language.Ast[])
		IL_004f: newobj instance void [System.Runtime]System.Text.StringBuilder::.ctor()
		IL_0054: stloc.s 5
		IL_0056: ldc.i4.0
		IL_0057: stloc.s 6
		IL_0059: ldc.i4.0
		IL_005a: stloc.s 7
		IL_005c: br IL_00f4
		// loop start (head: IL_00f4)
			IL_0061: ldloc.3
			IL_0062: ldloc.s 7
			IL_0064: ldelem.ref
			IL_0065: stloc.s 8
			IL_0067: ldloc.s 8
			IL_0069: callvirt instance class [System.Management.Automation]System.Management.Automation.Language.IScriptExtent [System.Management.Automation]System.Management.Automation.Language.Ast::get_Extent()
			IL_006e: callvirt instance int32 [System.Management.Automation]System.Management.Automation.Language.IScriptExtent::get_StartOffset()
			IL_0073: ldloc.s 6
			IL_0075: blt.s IL_00ee

			IL_0077: ldloc.s 8
			IL_0079: callvirt instance class [System.Management.Automation]System.Management.Automation.Language.IScriptExtent [System.Management.Automation]System.Management.Automation.Language.Ast::get_Extent()
			IL_007e: callvirt instance int32 [System.Management.Automation]System.Management.Automation.Language.IScriptExtent::get_StartOffset()
			IL_0083: ldloc.s 6
			IL_0085: ble.s IL_00a6

			IL_0087: ldloc.s 5
			IL_0089: ldloc.0
			IL_008a: callvirt instance string SpecterInsight.Obfuscation.Classes.StringTransformResult::get_Contents()
			IL_008f: ldloc.s 6
			IL_0091: ldloc.s 8
			IL_0093: callvirt instance class [System.Management.Automation]System.Management.Automation.Language.IScriptExtent [System.Management.Automation]System.Management.Automation.Language.Ast::get_Extent()
			IL_0098: callvirt instance int32 [System.Management.Automation]System.Management.Automation.Language.IScriptExtent::get_StartOffset()
			IL_009d: ldloc.s 6
			IL_009f: sub
			IL_00a0: callvirt instance class [System.Runtime]System.Text.StringBuilder [System.Runtime]System.Text.StringBuilder::Append(string, int32, int32)
			IL_00a5: pop

			IL_00a6: ldarg.0
			IL_00a7: ldloc.0
			IL_00a8: callvirt instance string SpecterInsight.Obfuscation.Classes.StringTransformResult::get_Contents()
			IL_00ad: ldloc.s 8
			IL_00af: callvirt instance string SpecterInsight.Obfuscation.PowerShell.PwshAstTransformer::Transform(string, class [System.Management.Automation]System.Management.Automation.Language.Ast)
			IL_00b4: stloc.s 9
			IL_00b6: ldarg.0
			IL_00b7: ldfld class [System.Collections]System.Collections.Generic.HashSet`1<string> SpecterInsight.Obfuscation.PowerShell.PwshAstTransformer::_map
			IL_00bc: ldloc.s 9
			IL_00be: callvirt instance bool class [System.Collections]System.Collections.Generic.HashSet`1<string>::Add(!0)
			IL_00c3: pop
			IL_00c4: ldloc.s 5
			IL_00c6: ldloc.s 9
			IL_00c8: callvirt instance class [System.Runtime]System.Text.StringBuilder [System.Runtime]System.Text.StringBuilder::Append(string)
			IL_00cd: pop
			IL_00ce: ldloc.s 8
			IL_00d0: callvirt instance class [System.Management.Automation]System.Management.Automation.Language.IScriptExtent [System.Management.Automation]System.Management.Automation.Language.Ast::get_Extent()
			IL_00d5: callvirt instance int32 [System.Management.Automation]System.Management.Automation.Language.IScriptExtent::get_StartOffset()
			IL_00da: ldloc.s 8
			IL_00dc: callvirt instance class [System.Management.Automation]System.Management.Automation.Language.IScriptExtent [System.Management.Automation]System.Management.Automation.Language.Ast::get_Extent()
			IL_00e1: callvirt instance string [System.Management.Automation]System.Management.Automation.Language.IScriptExtent::get_Text()
			IL_00e6: callvirt instance int32 [System.Runtime]System.String::get_Length()
			IL_00eb: add
			IL_00ec: stloc.s 6

			IL_00ee: ldloc.s 7
			IL_00f0: ldc.i4.1
			IL_00f1: add
			IL_00f2: stloc.s 7

			IL_00f4: ldloc.s 7
			IL_00f6: ldloc.3
			IL_00f7: ldlen
			IL_00f8: conv.i4
			IL_00f9: blt IL_0061
		// end loop

		IL_00fe: ldloc.s 6
		IL_0100: ldloc.0
		IL_0101: callvirt instance string SpecterInsight.Obfuscation.Classes.StringTransformResult::get_Contents()
		IL_0106: callvirt instance int32 [System.Runtime]System.String::get_Length()
		IL_010b: bge.s IL_012b

		IL_010d: ldloc.s 5
		IL_010f: ldloc.0
		IL_0110: callvirt instance string SpecterInsight.Obfuscation.Classes.StringTransformResult::get_Contents()
		IL_0115: ldloc.s 6
		IL_0117: ldloc.0
		IL_0118: callvirt instance string SpecterInsight.Obfuscation.Classes.StringTransformResult::get_Contents()
		IL_011d: callvirt instance int32 [System.Runtime]System.String::get_Length()
		IL_0122: ldloc.s 6
		IL_0124: sub
		IL_0125: callvirt instance class [System.Runtime]System.Text.StringBuilder [System.Runtime]System.Text.StringBuilder::Append(string, int32, int32)
		IL_012a: pop

		IL_012b: ldarg.0
		IL_012c: ldloc.s 5
		IL_012e: ldloc.3
		IL_012f: ldloc.0
		IL_0130: callvirt instance class SpecterInsight.Obfuscation.Classes.StringTransformResult SpecterInsight.Obfuscation.PowerShell.PwshAstTransformer::PostTransform(class [System.Runtime]System.Text.StringBuilder, class [System.Management.Automation]System.Management.Automation.Language.Ast[], class SpecterInsight.Obfuscation.Classes.StringTransformResult)
		IL_0135: stloc.0
		IL_0136: ldloc.1
		IL_0137: ldc.i4.1
		IL_0138: add
		IL_0139: stloc.1

		IL_013a: ldloc.1
		IL_013b: ldarg.0
		IL_013c: callvirt instance int32 SpecterInsight.Obfuscation.PowerShell.PwshAstTransformer::get_MaxPasses()
		IL_0141: blt IL_0015
	// end loop

	IL_0146: ldloc.0
	IL_0147: ret
} // end of method PwshAstTransformer::OnTransform
