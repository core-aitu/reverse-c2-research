"""Render the managed-components static inventory as reviewable Markdown."""

from __future__ import annotations

import argparse
from collections import Counter
import json
from pathlib import Path


def cell(value: object, empty: str = "—") -> str:
    if isinstance(value, list):
        value = ", ".join(str(item) for item in value)
    text = str(value).replace("|", "\\|").replace("\n", " ").strip()
    return text or empty


def short_sha(value: str) -> str:
    return f"`{value[:12]}…{value[-4:]}`"


def write(path: Path, lines: list[str]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("\n".join(lines).rstrip() + "\n")


def render_obfuscation(meta: dict, sem: dict, out: Path) -> None:
    ob = sem["obfuscation"]
    lines = [
        "# SpecterInsight.Obfuscation — статическая спецификация",
        "",
        "Статус: **static-complete** для образа " + short_sha(ob["identity"]["Sha256"]) +
        f" (MVID `{ob['identity']['Mvid']}`). Метаданные содержат **{ob['identity']['TypeCount']} TypeDef** и "
        f"**{ob['identity']['MethodCount']} MethodDef**; суммы функциональных групп совпадают с этими знаменателями.",
        "",
        "Основа: прямое чтение CLI metadata/IL, ILSpy-реконструкция и выборочная проверка критических методов по IL. "
        "REA подтвердил идентичность и метаданные образа: `ev_1cf1ec9e2279ea24e858e6bd0a5091c86c1ad328bfa4454239ff28ddda2499d7`; "
        "выбранные transform-типы: `ev_e7bf87d4c690c5cedb2487ebae05f6fbd15114e8cbcc21804f1d369ac9f5481a`.",
        "",
        "## Функциональные группы",
        "",
        "| Группа | Типы | Методы | Назначение |",
        "|---|---:|---:|---|",
    ]
    purposes = {
        "VBS": "парсер/AST VBScript, transforms и cmdlets",
        "PowerShell": "AST/token/source transforms, генераторы, cmdlets и обходы",
        "CSharp": "Roslyn AST/source/output transforms, компиляторы и evasion",
        "Zig": "AST/source/output transforms, build/objcopy и cmdlets",
        "Core framework": "контракт pipeline, результаты, payload formats, extensions и общие классы",
        "Compiler-generated support": "служебные типы компилятора без предметного API",
        "Documentation": "описатели и документационные модели",
        "Windows links": "создание/обработка Windows shortcut/link",
    }
    for row in ob["functional_groups"]:
        lines.append(f"| {row['group']} | {row['types']} | {row['methods']} | {purposes[row['group']]} |")
    lines += [
        f"| **Итого** | **{sum(x['types'] for x in ob['functional_groups'])}** | "
        f"**{sum(x['methods'] for x in ob['functional_groups'])}** | |",
        "",
        "## Контракт transforms и форматы",
        "",
        "`BaseTransformer` задаёт узел ориентированного pipeline; `AbstractTransformer<TIn,TOut>` принимает результаты "
        "родительских узлов и возвращает typed result. Строковые узлы используют `StringTransformResult` "
        "(`string` плюс `EmbeddedFiles<string, byte[]>`), бинарные — `ByteTransformResult` (`byte[]`). "
        "Объявленные payload formats: `None`, `Any`, `PowerShell`, `CSharp`, `DotNetExe`, `DotNetDll`, "
        "`DotNetService`, `DotNetNativeExe`, `WindowsCommand`.",
        "",
        "| Путь | Вход → выход |",
        "|---|---|",
    ]
    for name, description in ob["format_contract"]["compiler_paths"].items():
        lines.append(f"| `{name}` | {description} |")
    lines += [
        "",
        "Связь с сервером подтверждена ссылками на transform-типы из `PayloadPipelineService`, "
        "`ObfuscationService`, PowerShell runspace/session services, compiler cmdlets и Specter-specific builders. "
        "Поле `pipeline_and_compiler_links` полного JSON-каталога содержит точные server-файлы для каждого типа.",
        "",
        f"## Каталог {ob['transform_count']} transform-related типов",
        "",
        "| Тип | Группа | База | Вход | Выход | Конфигурация | Server links |",
        "|---|---|---|---|---|---|---:|",
    ]
    for t in ob["transforms"]:
        config = [p["name"] + ":" + p["type"] for p in t["configuration_properties"]]
        if t["constructors"]:
            config += ["ctor(" + c + ")" for c in t["constructors"]]
        lines.append(
            f"| `{cell(t['type'])}` | {t['group']} | `{cell(t['base'])}` | `{cell(t['input'])}` | "
            f"`{cell(t['output'])}` | {cell(config)} | {len(t['pipeline_and_compiler_links'])} |"
        )
    tokens = {
        "0x060009d9": "AbstractTransformer.Transform(TransformResult[])",
        "0x06000ab2": "BFlatCompilerTransformer.OnTransform",
        "0x06000ad3": "CSharpCompilerTransformer.OnTransform",
        "0x06000aee": "ZigCompilerTransformer.OnTransform",
        "0x06000c73": "PwshAstTransformer.OnTransform",
    }
    by_token = {m["Token"]: m for m in meta["obfuscation"]["Methods"]}
    lines += [
        "",
        "## Выборочная проверка непосредственно по IL",
        "",
        "| Token | Метод | Инструкции | SHA-256 нормализованного IL | Листинг |",
        "|---|---|---:|---|---|",
    ]
    for token, label in tokens.items():
        row = by_token[token]
        lines.append(
            f"| `{token}` | `{label}` | {row['InstructionCount']} | `{row['NormalizedIlSha256']}` | "
            f"[`{token}.il`](../validation/managed-il/{token}.il) |"
        )
    lines += [
        "",
        "Граница результата: это статическая спецификация выбранного PE. Внешние compiler/toolchain процессы, "
        "сгенерированные payload и поведение на конкретной ОС не запускались.",
        "",
        "Полные данные: [managed-semantics.json](managed-semantics.json) · "
        "[managed-metadata.json](managed-metadata.json) · "
        "[методика IL-проверки](../validation/managed-il-samples.md)",
    ]
    write(out / "obfuscation.md", lines)


def render_modules(sem: dict, out: Path) -> None:
    modules = sem["modules"]
    lines = [
        "# Modules — статическая спецификация 22 managed-файлов",
        "",
        "Статус: **static-complete**. Серверный `ModuleManagerService` загружает `modules/*.dll`, "
        "экспортирует типы-наследники `Cmdlet` с `[Cmdlet]` и разрешает зависимости по `AssemblyRef`. "
        f"В 22 файлах найдено **{modules['cmdlet_count']}** экспортируемых PSCmdlet в 16 carriers; ещё 6 файлов — dependency-only.",
        "",
        "Роль `product-module-carrier` означает роль в поставке, а не утверждение о юридическом авторстве каждого "
        "вложенного фрагмента. `third-party-module-carrier` сохраняет явную внешнюю product/namespace identity; "
        "`dependency-library` не экспортирует PSCmdlet.",
        "",
        "## Сводная матрица",
        "",
        "| Файл | Роль | TypeDef / MethodDef | Cmdlets | P/Invoke | Системные эффекты |",
        "|---|---|---:|---:|---:|---|",
    ]
    for m in modules["modules"]:
        lines.append(
            f"| `{m['file']}` | {m['role']} | {m['types']} / {m['methods']} | {m['cmdlet_count']} | "
            f"{len(m['pinvokes'])} | {m['system_effects']} |"
        )
    lines += ["", "## Пофайловая поверхность", ""]
    for m in modules["modules"]:
        cmdlets = [x["name"] for x in m["exported_cmdlets"]]
        params = sorted({p["name"] + ":" + p["type"] for c in m["exported_cmdlets"] for p in c["inputs"]})
        outputs = sorted({o for c in m["exported_cmdlets"] for o in c["declared_output_types"]})
        write_modes = Counter()
        for c in m["exported_cmdlets"]:
            if c["writes_objects"]:
                write_modes["WriteObject"] += 1
            if c["writes_errors"]:
                write_modes["WriteError"] += 1
        pinvoke_modules = Counter(x["Module"] for x in m["pinvokes"])
        pinvoke_entries = [f"{x['Module']}!{x['EntryPoint']}" for x in m["pinvokes"]]
        lines += [
            f"### `{m['file']}`",
            "",
            f"- Identity: SHA-256 `{m['sha256']}`, MVID `{m['mvid']}`; роль `{m['role']}`. {m['classification_basis']}",
            f"- Экспортируемые функции ({len(cmdlets)}): {cell([f'`{x}`' for x in cmdlets])}.",
            f"- Входы: {cell([f'`{x}`' for x in params])}.",
            f"- Результаты: declared OutputType {cell([f'`{x}`' for x in outputs])}; "
            f"каналы записи {cell([f'{k} у {v} cmdlet' for k, v in write_modes.items()])}.",
            f"- P/Invoke ({len(m['pinvokes'])}): модули {cell([f'`{k}` ({v})' for k, v in sorted(pinvoke_modules.items())])}; "
            + (f"entries {cell([f'`{x}`' for x in pinvoke_entries])}." if len(pinvoke_entries) <= 20 else "полный список entries находится в `managed-semantics.json`."),
            f"- Системные эффекты: {m['system_effects']}",
            f"- AssemblyRef: {cell([f'`{x}`' for x in m['references']])}.",
            "",
        ]
    lines += [
        "Полные сигнатуры входных параметров каждого cmdlet, declared output types и все P/Invoke entries: "
        "[managed-semantics.json](managed-semantics.json).",
        "",
        "Граница результата: эффекты выведены из статических путей и API/import surface; фактические изменения "
        "конкретной системы не наблюдались.",
    ]
    write(out / "modules.md", lines)


def render_beacons(meta: dict, out: Path) -> None:
    data = meta["managed_beacons"]
    variants = data["variants"]
    comparisons = data["comparisons"]
    by_sha = {v["Sha256"]: v for v in variants}
    lines = [
        "# Managed Beacon variants — статическое сравнение",
        "",
        "Статус: **static-complete** для пяти перечисленных образов. База — " + short_sha(data["baseline_sha256"]) +
        "; четыре варианта без matched PDB сопоставлены по полной method identity и SHA-256 нормализованного IL.",
        "",
        "## Идентичность и совпадения методов",
        "",
        "| Образ | MVID | TypeDef / MethodDef | Matched | Неизменный IL | Изменённый IL | Только база / вариант |",
        "|---|---|---:|---:|---:|---:|---:|",
    ]
    baseline = by_sha[data["baseline_sha256"]]
    lines.append(f"| `{baseline['Sha256'][:12]}…` (base) | `{baseline['Mvid']}` | {baseline['TypeCount']} / {baseline['MethodCount']} | — | — | — | — |")
    for c in comparisons:
        v = by_sha[c["candidate_sha256"]]
        n = c["counts"]
        lines.append(
            f"| `{v['Sha256'][:12]}…` | `{v['Mvid']}` | {v['TypeCount']} / {v['MethodCount']} | "
            f"{n['matched_by_identity']} | {n['unchanged_normalized_il']} | {n['changed_normalized_il']} | "
            f"{n['only_baseline']} / {n['only_candidate']} |"
        )
    lines += [
        "",
        "Нормализация сохраняет opcodes, декодированные сигнатуры metadata operands, branch targets и строки; "
        "она устраняет различия токенов/раскладки и потому пригодна для семантического diff, но не заменяет runtime.",
        "",
        "## Что изменено",
        "",
        "Текстовая сверка 78 реконструированных C#-файлов на вариант локализовала различия в 9–10 файлах: "
        "`Agent.cs`, `LongTimer.cs`, `ProcessInfoBasic.cs`, `Runspaces.cs`, `Settings.cs`, `HttpConnection.cs`, "
        "`Program.cs`, `Utility.cs`; `RunScriptTask.cs` отличается у `44b886…`, `85fb4c…` и `3fff31…`, но не у `0c8aaa…`. "
        "Остальные 68–69 файлов совпадают текстуально.",
        "",
        "| Область | Результат сравнения |",
        "|---|---|",
        "| Конфигурация | Во всех вариантах одинаковы схема `SpecterInsight.Resources.Configuration.json`, environment override и defaults: `http://localhost`, `Fallback`, interval/window 500 ms, UA, `ValidateCert=false`, build `default`, 30 дней, 3 попытки, пустые scripts. Отличается реализация чтения/инициализации, не сам контракт полей. |",
        "| Транспорт | Набор `HttpConnection`/`SslConnection` и wire command types сохранён. Изменения сосредоточены в RSA/certificate parsing, request/debug/error paths и регистрации; это implementation variants, не новый транспортный протокол. |",
        "| Команды | Файлы cmdlets, messaging types и command set совпадают; новых команд по статическому diff не найдено. Различия `RunScriptTask` относятся к обработке/выводу задачи. |",
        "| Платформы | Варианты расходятся в `Runspaces`, `ProcessInfoBasic`, `Utility` и initialization paths: Windows/Linux process discovery, PowerShell runspace compatibility и регистрация netcore implant. `44b886…` и `3fff31…` имеют одинаковые `Agent`/`Runspaces`; `85fb4c…` использует отдельный runspace variant; `0c8aaa…` добавляет подробную console diagnostics и упрощает runspace path. |",
        "",
        "Полные множества `changed_methods`, `only_baseline_methods` и `only_candidate_methods` для каждого образа "
        "сохранены в [managed-metadata.json](managed-metadata.json). Там же находятся hashes каждого MethodDef.",
        "",
        "Граница результата: сопоставление утверждает равенство/различие статического IL и контрактов; сеть, "
        "планировщики, runspaces и platform branches не исполнялись.",
    ]
    write(out / "beacon-variants.md", lines)


def render_ui(sem: dict, out: Path) -> None:
    ui = sem["client_ui"]
    lines = [
        "# Client UI — 24 workflow и semantic review",
        "",
        f"Статус: **static-complete**. Матрица содержит {ui['workflow_count']}/24 navigation workflow; semantic review "
        f"охватывает {ui['semantic_review']['file_count']} ILSpy-файла: "
        + ", ".join(f"{k} {v}" for k, v in ui["semantic_review"]["directory_counts"].items()) + ".",
        "",
        "REST-операции — имена generated ApiClient methods; SignalR — клиентские subscriptions. Локальные операции "
        "и cache services включают screen, ViewModel и прямо используемые cache services. Пустая ячейка означает, "
        "что зависимость не найдена в этом статическом scope, а не что действие невозможно во время исполнения.",
        "",
        "## Матрица workflow",
        "",
        "| Категория / экран | ViewModel | REST / SignalR | Файлы и cache | Действия | Loading/error |",
        "|---|---|---|---|---|---|",
    ]
    for w in ui["workflows"]:
        remote = ["REST: " + ", ".join(w["rest_operations"])] if w["rest_operations"] else []
        if w["signalr_subscriptions"]:
            remote.append("SignalR: " + ", ".join(w["signalr_subscriptions"]))
        local = []
        if w["local_operations"]:
            local.append("local: " + ", ".join(w["local_operations"]))
        if w["cache_services"]:
            local.append("cache: " + ", ".join(w["cache_services"]))
        handling = ", ".join(w["state_and_error_properties"])
        handling += f"; try/catch={'yes' if w['error_handling']['has_try_catch'] else 'no'}"
        handling += f", notify={'yes' if w['error_handling']['notifies_errors'] else 'no'}"
        lines.append(
            f"| {w['category']} / **{w['title']}** (`{w['screen']}`) | `{w['viewmodel']}` | {cell(remote)} | "
            f"{cell(local)} | {cell(w['user_actions'])} | {cell(handling)} |"
        )
    lines += [
        "",
        "## Полный semantic review Service/Utility/ViewModel",
        "",
        "| Файл | Типы | REST / SignalR | Local/cache | Состояния и ошибки |",
        "|---|---|---|---|---|",
    ]
    for f in ui["semantic_review"]["files"]:
        remote = f["rest_operations"] + f["signalr_subscriptions"]
        local = f["local_operations"] + f["cache_services"]
        status = f["state_and_error_properties"] + [
            f"try/catch={'yes' if f['has_try_catch'] else 'no'}",
            f"notify={'yes' if f['notifies_errors'] else 'no'}",
        ]
        lines.append(
            f"| `{f['source']}` | {cell([f'`{x}`' for x in f['types']])} | {cell(remote)} | {cell(local)} | {cell(status)} |"
        )
    lines += [
        "",
        "Полная запись для каждого файла дополнительно содержит SHA-256, user actions и все извлечённые зависимости: "
        "[managed-semantics.json](managed-semantics.json). Статическая матрица 202/202 REST operations и 25 SignalR "
        "subscriptions остаётся каноническим протокольным индексом; этот документ показывает их использование в UI workflow.",
        "",
        "Граница результата: фактическая отрисовка, timing и пользовательская последовательность событий не наблюдались.",
    ]
    write(out / "client-ui.md", lines)


def render_index(sem: dict, out: Path) -> None:
    lines = [
        "# Статическая спецификация собственных managed-компонентов",
        "",
        "Статус корпуса: **static-complete** для перечисленных бинарных образов и реконструкций. "
        "Результат не подменяет runtime-проверку внешних toolchains, ОС, сети и реального UI.",
        "",
        "| Область | Закрытый знаменатель | Спецификация |",
        "|---|---:|---|",
        "| SpecterInsight.Obfuscation | 837 TypeDef / 7 673 MethodDef; 148 transform-related типов; 5 IL samples | [obfuscation.md](obfuscation.md) |",
        "| Modules | 22/22 DLL; 191 PSCmdlet; все P/Invoke и AssemblyRef | [modules.md](modules.md) |",
        "| Managed Beacon variants | 4/4 no-PDB variants против `d5e7…569d`; полный normalized-IL diff | [beacon-variants.md](beacon-variants.md) |",
        "| Client UI | 24/24 workflow; 152 Service/Utility/ViewModel files | [client-ui.md](client-ui.md) |",
        "",
        "Машиночитаемые основания: [managed-metadata.json](managed-metadata.json) и "
        "[managed-semantics.json](managed-semantics.json). Воспроизводимые анализаторы находятся в "
        "`research/examination/managed-components/`.",
        "",
        "Единственный реестр вопросов, которые не закрываются этой статической спецификацией: "
        "[gaps.md](../validation/gaps.md).",
    ]
    write(out / "managed-components-static-spec.md", lines)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("metadata", type=Path)
    parser.add_argument("semantics", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    meta = json.loads(args.metadata.read_text())
    sem = json.loads(args.semantics.read_text())
    render_obfuscation(meta, sem, args.output)
    render_modules(sem, args.output)
    render_beacons(meta, args.output)
    render_ui(sem, args.output)
    render_index(sem, args.output)


if __name__ == "__main__":
    main()
