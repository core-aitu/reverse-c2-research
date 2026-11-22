# Managed Beacon variants — статическое сравнение

Статус: **static-complete** для пяти перечисленных образов. База — `d5e7e80c6e67…569d`; четыре варианта без matched PDB сопоставлены по полной method identity и SHA-256 нормализованного IL.

## Идентичность и совпадения методов

| Образ | MVID | TypeDef / MethodDef | Matched | Неизменный IL | Изменённый IL | Только база / вариант |
|---|---|---:|---:|---:|---:|---:|
| `d5e7e80c6e67…` (base) | `e10959e7-1725-4dc7-9ff7-d7dd25e9f0e8` | 101 / 710 | — | — | — | — |
| `44b88633520b…` | `aace5ac7-76e1-468b-baaa-2904a1512a2c` | 101 / 707 | 703 | 583 | 120 | 7 / 4 |
| `85fb4c9ac4ee…` | `305c9607-ab93-4a7f-a38f-38c745dfbf0b` | 101 / 706 | 702 | 571 | 131 | 8 / 4 |
| `3fff31bed46c…` | `62b0d9b6-0f41-489c-a8cc-18284bb5cf90` | 101 / 707 | 703 | 578 | 125 | 7 / 4 |
| `0c8aaa733148…` | `75a77f22-8006-46e6-90f7-300784a901ad` | 101 / 705 | 705 | 592 | 113 | 5 / 0 |

Нормализация сохраняет opcodes, декодированные сигнатуры metadata operands, branch targets и строки; она устраняет различия токенов/раскладки и потому пригодна для семантического diff, но не заменяет runtime.

## Что изменено

Текстовая сверка 78 реконструированных C#-файлов на вариант локализовала различия в 9–10 файлах: `Agent.cs`, `LongTimer.cs`, `ProcessInfoBasic.cs`, `Runspaces.cs`, `Settings.cs`, `HttpConnection.cs`, `Program.cs`, `Utility.cs`; `RunScriptTask.cs` отличается у `44b886…`, `85fb4c…` и `3fff31…`, но не у `0c8aaa…`. Остальные 68–69 файлов совпадают текстуально.

| Область | Результат сравнения |
|---|---|
| Конфигурация | Во всех вариантах одинаковы схема `SpecterInsight.Resources.Configuration.json`, environment override и defaults: `http://localhost`, `Fallback`, interval/window 500 ms, UA, `ValidateCert=false`, build `default`, 30 дней, 3 попытки, пустые scripts. Отличается реализация чтения/инициализации, не сам контракт полей. |
| Транспорт | Набор `HttpConnection`/`SslConnection` и wire command types сохранён. Изменения сосредоточены в RSA/certificate parsing, request/debug/error paths и регистрации; это implementation variants, не новый транспортный протокол. |
| Команды | Файлы cmdlets, messaging types и command set совпадают; новых команд по статическому diff не найдено. Различия `RunScriptTask` относятся к обработке/выводу задачи. |
| Платформы | Варианты расходятся в `Runspaces`, `ProcessInfoBasic`, `Utility` и initialization paths: Windows/Linux process discovery, PowerShell runspace compatibility и регистрация netcore implant. `44b886…` и `3fff31…` имеют одинаковые `Agent`/`Runspaces`; `85fb4c…` использует отдельный runspace variant; `0c8aaa…` добавляет подробную console diagnostics и упрощает runspace path. |

Полные множества `changed_methods`, `only_baseline_methods` и `only_candidate_methods` для каждого образа сохранены в [managed-metadata.json](managed-metadata.json). Там же находятся hashes каждого MethodDef.

Граница результата: сопоставление утверждает равенство/различие статического IL и контрактов; сеть, планировщики, runspaces и platform branches не исполнялись.
