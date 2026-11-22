// IL code: SpecterInsight.Obfuscation
.method public final hidebysig virtual 
	instance class SpecterInsight.Obfuscation.Classes.TransformResult Transform (
		class SpecterInsight.Obfuscation.Classes.TransformResult[] inputs
	) cil managed 
{
	.param [1]
		.custom instance void [System.Runtime]System.ParamArrayAttribute::.ctor() = (
			01 00 00 00
		)
	// Method begins at RVA 0x48fc4
	// Header size: 12
	// Code size: 129 (0x81)
	.maxstack 3
	.locals init (
		[0] class [System.Collections]System.Collections.Generic.List`1<!IN>,
		[1] class SpecterInsight.Obfuscation.Classes.TransformResult[],
		[2] int32,
		[3] class SpecterInsight.Obfuscation.Classes.TransformResult,
		[4] !IN
	)

	IL_0000: ldarg.0
	IL_0001: ldarg.1
	IL_0002: call instance void class SpecterInsight.Obfuscation.Transforms.AbstractTransformer`2<!IN, !OUT>::AssertArgumentLengthIsValid(class [System.Runtime]System.Collections.IList)
	IL_0007: newobj instance void class [System.Collections]System.Collections.Generic.List`1<!IN>::.ctor()
	IL_000c: stloc.0
	IL_000d: ldarg.1
	IL_000e: ldlen
	IL_000f: brfalse.s IL_006f

	IL_0011: ldarg.1
	IL_0012: stloc.1
	IL_0013: ldc.i4.0
	IL_0014: stloc.2
	IL_0015: br.s IL_0069
	// loop start (head: IL_0069)
		IL_0017: ldloc.1
		IL_0018: ldloc.2
		IL_0019: ldelem.ref
		IL_001a: stloc.3
		IL_001b: ldloc.3
		IL_001c: isinst !IN
		IL_0021: unbox.any !IN
		IL_0026: stloc.s 4
		IL_0028: ldloc.s 4
		IL_002a: box !IN
		IL_002f: brfalse.s IL_003b

		IL_0031: ldloc.0
		IL_0032: ldloc.s 4
		IL_0034: callvirt instance void class [System.Collections]System.Collections.Generic.List`1<!IN>::Add(!0)
		IL_0039: br.s IL_0065

		IL_003b: ldstr "An input object of type {0} is not compatible with required input type {1}."
		IL_0040: ldloc.3
		IL_0041: callvirt instance class [System.Runtime]System.Type [System.Runtime]System.Object::GetType()
		IL_0046: callvirt instance string [System.Runtime]System.Type::get_FullName()
		IL_004b: ldtoken !IN
		IL_0050: call class [System.Runtime]System.Type [System.Runtime]System.Type::GetTypeFromHandle(valuetype [System.Runtime]System.RuntimeTypeHandle)
		IL_0055: callvirt instance string [System.Runtime]System.Type::get_FullName()
		IL_005a: call string [System.Runtime]System.String::Format(string, object, object)
		IL_005f: newobj instance void [System.Runtime]System.ArgumentException::.ctor(string)
		IL_0064: throw

		IL_0065: ldloc.2
		IL_0066: ldc.i4.1
		IL_0067: add
		IL_0068: stloc.2

		IL_0069: ldloc.2
		IL_006a: ldloc.1
		IL_006b: ldlen
		IL_006c: conv.i4
		IL_006d: blt.s IL_0017
	// end loop

	IL_006f: ldarg.0
	IL_0070: ldloc.0
	IL_0071: callvirt instance !0[] class [System.Collections]System.Collections.Generic.List`1<!IN>::ToArray()
	IL_0076: call instance !1 class SpecterInsight.Obfuscation.Transforms.AbstractTransformer`2<!IN, !OUT>::Transform(!0[])
	IL_007b: box !OUT
	IL_0080: ret
} // end of method AbstractTransformer`2::Transform
