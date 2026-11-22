"""Read selected administrative CIL bodies without loading assemblies."""
import hashlib
import json
from pathlib import Path
import sys

import dnfile
from dncil.cil.body.reader import read_method_body_from_bytes
from dncil.clr.token import Token

class Metadata:
    def __init__(self, path):
        self.pe = dnfile.dnPE(str(path), clr_lazy_load=True)
        self.tables = self.pe.net.mdtables
        self.parents = {edge.NestedClass.row_index: edge.EnclosingClass.row_index for edge in self.tables.NestedClass}
        self.owner = {}
        self.field_owner = {}
        self.names = {}
        for i, t in enumerate(self.tables.TypeDef, start=1):
            self.names[i] = ".".join(filter(None, (str(t.TypeNamespace), str(t.TypeName))))
            self.owner.update({m.row_index: i for m in t.MethodList})
            self.field_owner.update({f.row_index: i for f in t.FieldList})
        self.identity = {"file": path.name, "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
                         "mvid": str(self.tables.Module.rows[0].Mvid)}

    def typename(self, index):
        if index in self.parents:
            return self.typename(self.parents[index]) + "+" + self.names[index]
        return self.names[index]

    def symbol(self, token):
        tbl, index = token >> 24, token & 0xffffff
        if tbl == 0x70:
            return str(self.pe.net.user_strings.get(index).value)
        row = self.tables.tables[tbl].rows[index-1]
        if tbl == 43:
            return self.symbol(row.Method.table.number << 24 | row.Method.row_index)
        if tbl == 10:
            return self.symbol(row.Class.table.number << 24 | row.Class.row_index) + "::" + str(row.Name)
        if tbl == 6:
            return self.typename(self.owner[index]) + "::" + str(row.Name)
        if tbl == 4:
            return self.typename(self.field_owner[index]) + "::" + str(row.Name)
        if tbl == 1:
            return ".".join(filter(None, (str(row.TypeNamespace), str(row.TypeName))))
        if tbl == 2:
            return self.typename(index)
        if tbl == 27:
            return "TypeSpec:" + row.Signature.value.hex()
        return hex(token)

    def body(self, number):
        method = self.tables.MethodDef.rows[number-1]
        body = read_method_body_from_bytes(self.pe.get_data(method.Rva, 1 << 16))
        code = body.raw_bytes[body.header_size:body.header_size + body.code_size]
        operations = []
        for i in body.instructions:
            op, operand = i.opcode.name, i.operand
            entry = {"il": i.offset - body.header_size, "op": op}
            if isinstance(operand, Token):
                entry.update(token=f"0x{operand.value:08x}", symbol=self.symbol(operand.value))
            elif op in ('br','br.s','brtrue','brfalse','brfalse.s','brtrue.s','leave','leave.s') or i.is_cond_br():
                entry["destination"] = operand - body.header_size
            elif operand is not None:
                entry["value"] = str(operand)
            operations.append(entry)
        return {"type": self.typename(self.owner[number]), "method": str(method.Name),
                "token": f"0x{0x6000000 | number:08x}", "signature": method.Signature.value.hex(),
                "il_offset_in_file": self.pe.get_offset_from_rva(method.Rva) + body.header_size,
                "il_bytes": len(code), "il_sha256": hashlib.sha256(code).hexdigest(),
                "eh": [{"try": [e.try_start,e.try_end],"handler":[e.handler_start,e.handler_end],
                        "catch": self.symbol(e.catch_type.value) if e.catch_type else None} for e in body.exception_handlers],
                "operations": operations}

def inspect(path):
    data = Metadata(path)
    result = dict(data.identity, actions=[], startup=[])
    for number, owner in data.owner.items():
        typename = data.typename(owner)
        method = str(data.tables.MethodDef.rows[number-1].Name)
        controller = typename.split(".")[-1]
        if (controller == "EventsController" and method in ("GetAll", "GetLatest") or
                controller == "SettingsController" and method == "GetDefault" or
                controller.startswith(("EventsController+<GetAll>", "EventsController+<GetLatest>", "SettingsController+<GetDefault>")) and method == "MoveNext"):
            result["actions"].append(data.body(number))
        is_host = path.stem.endswith(".Host") and method in ("Main", "<Main>", "MoveNext")
        is_startup = typename == "SpecterInsight.Server.Startup" and method in ("ConfigureServices","Configure")
        is_builder = ((typename == "SpecterInsight.Server.KestrelManagementServer" and method == "Build") or
            (typename.startswith("SpecterInsight.Server.KestrelManagementServer+<Build>") and method == "MoveNext"))
        if is_host or is_startup or is_builder:
            body = data.body(number)
            operations = body.pop("operations")
            # Restrict the projection to framework wiring and the host's build/run path.
            calls = [o for o in operations if o["op"] in ("call", "callvirt", "newobj", "ldftn")]
            indicators = ("::UseStartup", "::ConfigureWebHostDefaults", "SpecterInsight.Server.Startup::")
            body["startup_direct_edges"] = [o for o in calls if any(s in o["symbol"] for s in indicators)]
            allowed = ("::CreateBuilder", "::AddControllers", "::AddMvc", "::AddRazorPages", "::AddSignalR",
                       "::AddAuthentication", "::AddAuthorization", "::AddDbContext", "::AddNewtonsoftJson",
                       "::AddRouting", "::UseRouting", "::UseAuthentication", "::UseAuthorization",
                       "::UseEndpoints", "::MapControllers", "::MapControllerRoute", "::AddSwaggerGen",
                       "::UseDeveloperExceptionPage", "::UseExceptionHandler","::UseHttpsRedirection",
                       "::UseStaticFiles", "::UseCors", "::AddCors", "::RunAsync", "::Build", "::Main")
            body["framework_calls"] = [o for o in calls if any(o["symbol"].endswith(s) for s in allowed) or
                (is_host and o["symbol"].endswith("::GetResult"))]
            body["call_sites_examined"] = len(calls)
            result["startup"].append(body)
    return result

if __name__ == "__main__":
    directory = Path(sys.argv[1])
    result = [inspect(directory / "server" / f"SpecterInsight.Server{s}.dll") for s in (".Host","")]
    print(json.dumps(result, ensure_ascii=False, indent=2))
