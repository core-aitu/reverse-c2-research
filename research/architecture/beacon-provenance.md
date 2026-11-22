# Beacon. Каким диагностическим методам можно сопоставить строки исходника

У всех пяти уникальных управляемых образов Beacon запись CodeView указывает одно имя файла `SpecterInsight.pdb`. Каждый образ при этом запрашивает свой идентификатор PDB. Во всей локальной поставке найдена одна пара «образ — подходящие внешние символы». Совпадают GUID и stamp, а также SHA-256 PDB, ожидаемый отладочным каталогом PE. Эти признаки подтверждают привязку к конкретным байтам при исследовании локального снимка. Судить по ним об авторе или подлинности выпуска нельзя. [Результаты по образам](../examination/beacon-provenance/result.json)

| Образ в поставке | Начало SHA-256 | Подходящий portable PDB |
|---|---|---|
| `server/SpecterInsight.exe` и побайтно идентичный `templates/any/specter-dotnet-exe.exe` | `d5e7e80c` | `server/SpecterInsight.pdb`, подтверждены идентификатор и контрольная сумма |
| `templates/any/SpecterInsight.exe` | `44b88633` | Среди просмотренных файлов отсутствует |
| `templates/any/specter-dotnet-exe.exe.bak` | `85fb4c9a` | Среди просмотренных файлов отсутствует |
| `templates/any/specter-dotnet-exe.exe.bak2` | `3fff31be` | Среди просмотренных файлов отсутствует |
| `templates/any/specter-dotnet-exe.exe.bak3` | `0c8aaa73` | Среди просмотренных файлов отсутствует |

**Граница поиска.** Обход исходного каталога нашёл 378 обычных путей с расширением PDB, в том числе 362 файла portable PDB с 347 различными идентификаторами. Среди записей отладочного каталога пяти PE отсутствуют встроенные portable PDB. Для четырёх образов имеющийся `server/SpecterInsight.pdb` расходится и по идентификатору, и по контрольной сумме. Поиск не охватывает содержимое архивов и файлы с произвольными расширениями. [Исходный код проверки](../examination/beacon-provenance/Program.cs)

## От образа к строке LogEntry.cs

1. REA независимо прочитал MVID основного образа `e10959e7-1725-4dc7-9ff7-d7dd25e9f0e8`. В отладочном каталоге образа и заголовке PDB совпадают GUID `02f203d1-8a24-4d3d-826b-d3cbacbc2996` и stamp `2629479585`. SHA-256 файла символов равен `3fc9e470652342d34a5a0f5cc4c75d25703e91d2d65e58ffdc9e2a7c4451b174`. Для сравнения с записью `PdbChecksum` отдельно хешировалась копия с нулевым полем идентификатора. Оба значения сохранены. [Реестр прежнего связывания](../evidence/debug-bindings.json) · [Свежая проверка](../examination/beacon-provenance/result.json)
2. Строка 44 таблицы `Document` PDB описывает `Logging/LogEntry.cs`. Хеш документа совпадает с SHA-256 файла `templates/source/SpecterInsight/Logging/LogEntry.cs`. У этой строки отсутствует запись `Embedded Source`. Проверяемый текст поставляется отдельным файлом. [Идентичность документа](../examination/beacon-provenance/result.json)
3. Все **35 видимых точек сопоставления кода со строками** в 13 методах `LogEntry` относятся к этому документу. Проверены границы строк и столбцов в исходнике и границы смещений IL каждого метода. Хеши тел всех 13 методов совпали с предыдущим чтением другим инструментом. Основание относится к этому образу и этим отладочным координатам. [Координаты и хеши](../examination/beacon-provenance/result.json) · [Предыдущие карточки](../validation/continuation/diagnostic-variants.json)

У каждого из пяти конструкторов первая точка попадает на строку 24, где заканчивается объявление свойства `Count` с инициализатором. Последующие точки попадают на соответствующие конструкторы в строках 26–50. Поэтому совпадение первой строки у разных конструкторов в прежнем журнале объясняется самим PDB. Для различения перегрузок служат токены и полная последовательность координат. [Первоначальный журнал](../validation/continuation/diagnostics/logentry-1.jsonl) · [Все координаты](../examination/beacon-provenance/result.json)

## Повторение

Утилита читает PE как данные через `PEReader`. Временная копия проекта позволяет собрать её отдельно от корпуса. Пример выполняется из корня исследовательского репозитория с доступным .NET SDK 10. Использованный SDK имел версию 10.0.404.

```bash
study=$(mktemp -d)
cp research/examination/beacon-provenance/{Provenance.csproj,Program.cs} "$study/"
dotnet build "$study/Provenance.csproj" --nologo -p:NuGetAudit=false
dotnet "$study/bin/Debug/net10.0/Provenance.dll" /home/deleteme/Desktop/SpecterInsight \
    research/validation/continuation/diagnostic-variants.json > "$study/result.json"
cmp "$study/result.json" research/examination/beacon-provenance/result.json
```
