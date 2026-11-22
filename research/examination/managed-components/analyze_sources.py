"""Build static semantic inventories from ILSpy/PDB reconstructions.

The script never imports or executes target assemblies. It combines the exact
metadata inventory emitted by ManagedComponents.csproj with decompiled C# text.
"""

from __future__ import annotations

import argparse
from collections import Counter, defaultdict
import hashlib
import json
from pathlib import Path
import re


TYPE_RE = re.compile(
    r"(?m)^\s*(?:public|internal|private|protected)?\s*(?:abstract|sealed|static|partial)?\s*"
    r"(?:class|record|struct|interface|enum)\s+([A-Za-z_][A-Za-z0-9_]*(?:<[^>{}]+>)?)"
    r"(?:\s*:\s*([^\n{]+))?"
)
NAMESPACE_RE = re.compile(r"(?m)^namespace\s+([^;{]+)[;{]")
PROPERTY_RE = re.compile(
    r"(?m)^\s*public\s+(?:static\s+)?(?:readonly\s+)?([A-Za-z_][A-Za-z0-9_?<>\[\], .]+?)\s+"
    r"([A-Za-z_][A-Za-z0-9_]*)\s*(?:\{|=>)"
)
METHOD_RE = re.compile(
    r"(?m)^\s*(?:public|private|protected|internal)\s+(?:static\s+)?(?:async\s+)?"
    r"[A-Za-z_][A-Za-z0-9_?<>\[\], .]*\s+([A-Za-z_][A-Za-z0-9_]*)\s*\("
)
API_RE = re.compile(
    r"(?:Globals\.Client|\bclient|\bspecterInsightClient|\bClient)\??\."
    r"([A-Za-z_][A-Za-z0-9_]*(?:Async|GET|POST|PUT|DELETE))\s*\("
)
SIGNAL_RE = re.compile(r"(?:Globals\.Client|\bclient)\??\.(On[A-Z][A-Za-z0-9_]*)\s*\(")
PINVOKE_SOURCE_RE = re.compile(
    r"\[DllImport\(\s*\"([^\"]+)\"(?:\s*,[^\]]*EntryPoint\s*=\s*\"([^\"]+)\")?[^\]]*\)\]"
)
CMDLET_RE = re.compile(r"\[Cmdlet\(\s*\"([^\"]+)\"\s*,\s*\"([^\"]+)\"")
OUTPUT_TYPE_ATTR_RE = re.compile(r"\[OutputType\((.*?)\)\]", re.S)
PARAMETER_RE = re.compile(
    r"\[Parameter(?:\((.*?)\))?\]\s*(?:\[[^\]]+\]\s*)*public\s+(?:readonly\s+)?"
    r"([A-Za-z_][A-Za-z0-9_?<>\[\], .]+?)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:\{|=|;)",
    re.S,
)


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def read_sources(root: Path) -> dict[Path, str]:
    return {path: path.read_text(errors="replace") for path in sorted(root.rglob("*.cs"))}


def obfuscation_group(namespace: str) -> str:
    if ".CSharp" in namespace:
        return "CSharp"
    if ".PowerShell" in namespace or namespace.startswith("Obfuscation.PowerShell"):
        return "PowerShell"
    if ".Vbs" in namespace:
        return "VBS"
    if ".Zig" in namespace:
        return "Zig"
    if ".Links" in namespace:
        return "Windows links"
    if ".Documentation" in namespace:
        return "Documentation"
    if namespace == "<global>":
        return "Compiler-generated support"
    return "Core framework"


def infer_transform_io(name: str, bases: str, text: str) -> tuple[str, str]:
    generic = re.search(r"(?:SingleInputTransform|AbstractTransformer)<\s*([^,>]+)\s*,\s*([^>]+)>", bases)
    if generic:
        return generic.group(1).strip(), generic.group(2).strip()
    if any(x in bases for x in (
        "SourceStringTransform", "SingleInputStringTransformer", "AbstractStringTransformer",
        "CSharpAstTransformer", "VbsAstTransformerBase", "ZigAstTransformerBase",
        "PwshAstTransformer", "PwshTokenTransformer",
    )):
        return "StringTransformResult", "StringTransformResult"
    on_transform = re.search(r"protected\s+override\s+([A-Za-z_][A-Za-z0-9_<>?]+)\s+OnTransform\s*\(([^)]*)\)", text)
    if on_transform:
        args = on_transform.group(2)
        input_type = re.search(r"([A-Za-z_][A-Za-z0-9_<>?]+)\s+\w+", args)
        return (input_type.group(1) if input_type else "implicit/none", on_transform.group(1))
    if "Transformer" in name:
        return "unknown", "unknown"
    return "n/a", "n/a"


def constructor_signatures(name: str, text: str) -> list[str]:
    plain = name.split("<", 1)[0]
    pattern = re.compile(rf"(?m)^\s*public\s+{re.escape(plain)}\s*\((.*?)\)\s*(?::|\{{)", re.S)
    result = []
    for match in pattern.finditer(text):
        signature = re.sub(r"\s+", " ", match.group(1)).strip()
        if len(signature) <= 1000:
            result.append(signature)
    return result


def build_obfuscation(metadata: dict, source_root: Path, server_root: Path) -> dict:
    sources = read_sources(source_root)
    groups: dict[str, dict[str, int]] = defaultdict(lambda: {"types": 0, "methods": 0})
    for row in metadata["NamespaceCounts"]:
        group = obfuscation_group(row["namespace"])
        groups[group]["types"] += row["types"]
        groups[group]["methods"] += row["methods"]

    server_sources = read_sources(server_root)
    transforms = []
    for path, text in sources.items():
        namespace_match = NAMESPACE_RE.search(text)
        namespace = namespace_match.group(1).strip() if namespace_match else "<global>"
        for match in TYPE_RE.finditer(text):
            name, bases = match.group(1), (match.group(2) or "").strip()
            if "Transform" not in name and "Transform" not in bases:
                continue
            input_type, output_type = infer_transform_io(name, bases, text)
            props = [
                {"type": t.strip(), "name": n}
                for t, n in PROPERTY_RE.findall(text)
                if n not in {"Parents", "Contents", "Content", "ContentType"}
            ]
            links = []
            token = name.split("<", 1)[0]
            word = re.compile(rf"\b{re.escape(token)}\b")
            for server_path, server_text in server_sources.items():
                if word.search(server_text):
                    links.append(str(server_path.relative_to(server_root)))
            transforms.append({
                "type": f"{namespace}.{name}" if namespace != "<global>" else name,
                "base": bases or None,
                "group": obfuscation_group(namespace),
                "input": input_type,
                "output": output_type,
                "constructors": constructor_signatures(name, text),
                "configuration_properties": props,
                "pipeline_and_compiler_links": links,
                "source": str(path.relative_to(source_root)),
                "source_sha256": sha256(path),
            })
    return {
        "identity": {k: metadata[k] for k in ("Path", "Sha256", "Name", "Version", "Mvid", "TypeCount", "MethodCount")},
        "functional_groups": [
            {"group": name, **counts}
            for name, counts in sorted(groups.items(), key=lambda item: (-item[1]["types"], item[0]))
        ],
        "transform_count": len(transforms),
        "transforms": sorted(transforms, key=lambda item: item["type"]),
        "format_contract": {
            "inputs": ["string", "StringTransformResult", "ByteTransformResult", "EmbeddedFiles<string,byte[]>", "PayloadContent"],
            "outputs": ["StringTransformResult", "ByteTransformResult"],
            "declared_payload_formats": [
                "None", "Any", "PowerShell", "CSharp", "DotNetExe", "DotNetDll",
                "DotNetService", "DotNetNativeExe", "WindowsCommand",
            ],
            "compiler_paths": {
                "Roslyn": "C# source -> CSharpCompilerTransformer -> managed PE bytes",
                "BFlat": "C# source -> temporary .cs -> bflat build -> native PE/ELF bytes",
                "Zig": "Zig source plus EmbeddedFiles -> zig build-exe/build-lib -> binary bytes",
                "ZigShellcode": "Zig binary -> zig objcopy .text -> raw shellcode bytes",
                "CSharpToPowerShell": "C# source -> Add-Type text or embedded managed assembly -> PowerShell source",
            },
        },
    }


MODULE_EFFECTS = {
    "AD": "Операции с Kerberos/AD tickets, LDAP/domain-запросы, enrollment сертификатов и credential material.",
    "CredentialManager": "Чтение, запись и удаление Windows Credential Manager; генерация паролей.",
    "Direct": "Прямые NT syscalls для процессов, потоков, sections и virtual memory; shellcode injection.",
    "Dumper": "Получение process/minidump, извлечение shadow copy и разбор credential material.",
    "EventLog": "Запрос Windows Event Log и lifecycle live subscriptions.",
    "Firewall": "Создание, перечисление и удаление правил Windows Firewall и WFP filters.",
    "GPO": "Создание и изменение AD/GPO, links, security filters, scheduled tasks и policies.",
    "common": "Обзор файлов/процессов, screenshots, разбор PE, timestomp и приостановка потоков/процессов.",
    "credentials": "Доступ к credentials/tokens/browser/SAM, keylogging, UAC bypass и запуск процессов с tokens.",
    "inveigh": "LLMNR/NBNS/mDNS/DNS/DHCPv6 spoofing и захват SMB/HTTP/LDAP credentials.",
    "lateral": "Remote execution, services/tasks/DCOM, process injection, registry и TCP redirectors.",
    "levelup": "Поиск и выполнение техник локального privilege escalation.",
    "persistence": "Изменения persistence через registry, IFEO, COM и ClickOnce.",
    "reach": "Поиск и выполнение техник remote exploitation.",
    "recon": "Host/domain/network/file/WMI reconnaissance и scanners.",
    "smb": "SMB exploitation и PsExec-style remote execution.",
}
DEPENDENCY_NAMES = {
    "Interop.NetFwTypeLib", "Interop.TaskScheduler", "Microsoft.GroupPolicy.Management",
    "PSCredentialManager.Api", "PSCredentialManager.Common", "System.DirectoryServices.AccountManagement",
}
THIRD_PARTY_CARRIERS = {"CredentialManager", "inveigh", "levelup", "reach"}


def build_modules(metadata_rows: list[dict], modules_root: Path) -> dict:
    by_file = {Path(row["Path"]).stem: row for row in metadata_rows}
    records = []
    for directory in sorted((p for p in modules_root.iterdir() if p.is_dir()), key=lambda p: p.name.lower()):
        sources = read_sources(directory)
        cmdlets = []
        source_pinvokes = []
        for path, text in sources.items():
            cmdlet = CMDLET_RE.search(text)
            if cmdlet:
                inputs = []
                for options, type_name, name in PARAMETER_RE.findall(text):
                    inputs.append({
                        "name": name,
                        "type": type_name.strip(),
                        "mandatory": bool(re.search(r"Mandatory\s*=\s*true", options, re.I)),
                        "value_from_pipeline": bool(re.search(r"ValueFromPipeline\s*=\s*true", options, re.I)),
                    })
                outputs = sorted({
                    output_type
                    for attribute in OUTPUT_TYPE_ATTR_RE.findall(text)
                    for output_type in re.findall(r"typeof\(([^)]+)\)", attribute)
                })
                cmdlets.append({
                    "name": f"{cmdlet.group(1)}-{cmdlet.group(2)}",
                    "class": path.stem,
                    "inputs": inputs,
                    "declared_output_types": outputs,
                    "writes_objects": text.count("WriteObject("),
                    "writes_errors": text.count("WriteError("),
                    "source": str(path.relative_to(directory)),
                })
            for module, entry in PINVOKE_SOURCE_RE.findall(text):
                source_pinvokes.append({"module": module, "entry_point": entry or None, "source": str(path.relative_to(directory))})
        row = by_file[directory.name]
        if directory.name in DEPENDENCY_NAMES:
            role = "dependency-library"
        elif directory.name in THIRD_PARTY_CARRIERS:
            role = "third-party-module-carrier"
        else:
            role = "product-module-carrier"
        records.append({
            "file": directory.name + ".dll",
            "assembly": directory.name,
            "role": role,
            "classification_basis": (
                "Нет экспортируемых PSCmdlet-типов; сборка используется module carriers как зависимость."
                if role == "dependency-library" else
                "Экспортирует PSCmdlet-типы; в поставленной сборке сохранена явная third-party product/namespace identity."
                if role == "third-party-module-carrier" else
                "Экспортирует PSCmdlet-типы под identity модуля поставки; внутри может присутствовать upstream-derived code."
            ),
            "sha256": row["Sha256"],
            "mvid": row["Mvid"],
            "types": row["TypeCount"],
            "methods": row["MethodCount"],
            "cmdlet_count": len(cmdlets),
            "exported_cmdlets": sorted(cmdlets, key=lambda item: item["name"]),
            "pinvokes": row["PInvokes"],
            "source_pinvoke_declarations": source_pinvokes,
            "references": row["References"],
            "system_effects": MODULE_EFFECTS.get(directory.name, "Dependency types/interop declarations; эффекты возникают через callers, а не через экспортируемую cmdlet surface."),
        })
    return {
        "file_count": len(records),
        "role_counts": dict(Counter(record["role"] for record in records)),
        "cmdlet_count": sum(record["cmdlet_count"] for record in records),
        "modules": records,
    }


WORKFLOWS = [
    ("Operations", "Listeners", "ListenersControl", "ListenersViewModel"),
    ("Operations", "Certificates", "CertificatesControl", "CertificatesViewModel"),
    ("Operations", "Implants", "ImplantsControl", "ImplantsViewModel"),
    ("Operations", "Payload Pipelines", "PayloadPipelinesControl", "PayloadPipelinesViewModel"),
    ("Operations", "Sessions", "SessionsControl", "SessionsViewModel"),
    ("Operations", "Scripts", "ScriptsControl", "ScriptsViewModel"),
    ("Operations", "Wordlists", "WordlistsControl", "WordlistsViewModel"),
    ("Operations", "Modules", "ModulesControl", "ModulesViewModel"),
    ("Operations", "Events", "EventLogControl", "EventLogViewModel"),
    ("Operations", "Workflows", "PowerShellNotebookWorkspaceControl", "PowerShellNotebookWorkspaceViewModel"),
    ("Operations", "WinRM Sessions", "WinRmSessionsControl", "WinRmSessionsViewModel"),
    ("Infrastructure", "Tunnels", "TunnelsControl", "TunnelsViewModel"),
    ("AI", "Profiles", "AiAgentProfilesControl", "AiAgentProfilesViewModel"),
    ("AI", "Functions", "AiAgentFunctionsControl", "AiAgentFunctionsViewModel"),
    ("AI", "Sessions", "AiAgentSessionsControl", "AiSessionsViewModel"),
    ("Documentation", "Home", "DocumentationHomeControl", "DocumentationHomeViewModel"),
    ("Documentation", "Docs", "DocumentationControl", "DocumentationViewModel"),
    ("Documentation", "Index", "DocumentationIndexControl", "DocumentationIndexViewModel"),
    ("Intel", "Network Map", "NetworkMapControl", "NetworkMapViewModel"),
    ("Intel", "Files", "ArtifactsControl", "ArtifactsViewModel"),
    ("Administration", "Users", "UsersControl", "UsersViewModel"),
    ("Administration", "Updates", "SoftwareUpdatesControl", "SoftwareUpdatesViewModel"),
    ("Administration", "License", "LicenseControl", "LicenseViewModel"),
    ("Administration", "Settings", "SettingsControl", "SettingsViewModel"),
]


def source_semantics(path: Path, text: str) -> dict:
    local_ops = sorted(set(re.findall(r"\b(File|Directory|Path|Environment|Process)\.([A-Za-z_][A-Za-z0-9_]*)", text)))
    caches = sorted(set(re.findall(r"\b([A-Za-z_][A-Za-z0-9_]*CacheService)\b", text)))
    states = sorted(set(re.findall(r"\b((?:Is|Has|Can|Last)[A-Z][A-Za-z0-9_]*(?:Error|Loading|Busy|Enabled|Selected|Refreshing|Status)?|Error|StatusText)\b", text)))
    actions = sorted(set(name for name in METHOD_RE.findall(text) if name.startswith(("On", "Refresh", "Load", "Create", "Delete", "Remove", "Save", "Open", "Run", "Start", "Stop", "Import", "Export", "Update", "Search"))))
    return {
        "source": str(path),
        "sha256": hashlib.sha256(text.encode()).hexdigest(),
        "types": [match.group(1) for match in TYPE_RE.finditer(text)],
        "rest_operations": sorted(set(API_RE.findall(text))),
        "signalr_subscriptions": sorted(set(SIGNAL_RE.findall(text))),
        "local_operations": [f"{owner}.{method}" for owner, method in local_ops],
        "cache_services": caches,
        "user_actions": actions,
        "state_and_error_properties": states,
        "has_try_catch": "try" in text and "catch" in text,
        "notifies_errors": "NotificationService" in text or "LastError" in text,
    }


def find_named(sources: dict[Path, str], name: str) -> list[tuple[Path, str]]:
    return [(path, text) for path, text in sources.items() if path.name == name + ".cs"]


def build_ui(ui_root: Path) -> dict:
    sources = read_sources(ui_root)
    semantic_files = []
    for path, text in sources.items():
        relative = path.relative_to(ui_root)
        review_group = next((part for part in relative.parts if part in {"Services", "Utilities", "ViewModels"}), None)
        if review_group:
            record = source_semantics(relative, text)
            record["review_group"] = review_group
            semantic_files.append(record)

    workflows = []
    for category, title, control, viewmodel in WORKFLOWS:
        selected = find_named(sources, control) + find_named(sources, viewmodel)
        combined = "\n".join(text for _, text in selected)
        for cache_name in sorted(set(re.findall(r"\b([A-Za-z_][A-Za-z0-9_]*CacheService)\b", combined))):
            for item in find_named(sources, cache_name):
                if item not in selected:
                    selected.append(item)
        combined = "\n".join(text for _, text in selected)
        semantics = source_semantics(Path(f"{control}+{viewmodel}"), combined)
        workflows.append({
            "category": category,
            "title": title,
            "screen": control,
            "viewmodel": viewmodel,
            "source_files": [str(path.relative_to(ui_root)) for path, _ in selected],
            "rest_operations": semantics["rest_operations"],
            "signalr_subscriptions": semantics["signalr_subscriptions"],
            "local_operations": semantics["local_operations"],
            "cache_services": semantics["cache_services"],
            "user_actions": semantics["user_actions"],
            "state_and_error_properties": semantics["state_and_error_properties"],
            "error_handling": {
                "has_try_catch": semantics["has_try_catch"],
                "notifies_errors": semantics["notifies_errors"],
            },
        })
    return {
        "workflow_count": len(workflows),
        "workflows": workflows,
        "semantic_review": {
            "scope": "Every ILSpy C# file under Services, Utilities and ViewModels.",
            "file_count": len(semantic_files),
            "directory_counts": dict(Counter(record["review_group"] for record in semantic_files)),
            "files": semantic_files,
        },
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("metadata", type=Path)
    parser.add_argument("obfuscation_sources", type=Path)
    parser.add_argument("module_sources", type=Path)
    parser.add_argument("server_sources", type=Path)
    parser.add_argument("ui_sources", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    metadata = json.loads(args.metadata.read_text())
    result = {
        "schema": "specter-managed-source-semantics-v1",
        "evidence_kind": "static-decompiler-derived",
        "limitations": [
            "ILSpy output is a high-level reconstruction, not original source and not runtime evidence.",
            "Module ownership labels describe shipped package roles and retained namespace/product provenance; they do not establish legal authorship.",
            "Workflow dependency lists include the screen, ViewModel and directly referenced cache services; other shared service behavior is indexed separately in semantic_review.",
        ],
        "obfuscation": build_obfuscation(metadata["obfuscation"], args.obfuscation_sources, args.server_sources),
        "modules": build_modules(metadata["modules"], args.module_sources),
        "client_ui": build_ui(args.ui_sources),
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, indent=2, ensure_ascii=False) + "\n")


if __name__ == "__main__":
    main()
