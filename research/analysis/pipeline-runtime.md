# Изолированная runtime-проверка payload pipeline

## Границы проверки

Проверка выполнена 22 ноября 2026 года на Fedora Linux x64 с .NET 10.0.13. Все входы — минимальные инертные fixtures: локальная библиотека, программа без побочных эффектов, freestanding `_start` с бесконечным циклом, локальный PowerShell-текст и детерминированный массив байт. Рабочий C2, сетевые цели, реальные implant settings и внешние URL не использовались. Полученные PE, ELF, SRDI и raw `.text` не запускались.

Проверяемые реализации загружались из поставки:

| Артефакт | SHA-256 / версия |
|---|---|
| `SpecterInsight.Obfuscation.dll` | `7c48d7c851c4ef4c531d616699366e307b155099a84f31bb8b9e8596db57fb4b` |
| `SpecterInsight.Server.dll` | `a6c454cdfc80e5d9bd3906747d30edbe4fcf69ab2e3c2b52b301b6f4b60e46a5` |
| `dependencies/zig/zig` | `2317bbb91798556d9d0f38aabdac23db83f0979b25f767259ae474546724087c`, Zig `0.16.0` |
| `dependencies/bflat/bflat` | `b923c01abee8f909bcecdaca899c312ce4ae4a78101c31365c574b865a8e3b2e`, BFlat `8.0.2+1b61fe…` |
| `dependencies/specter-pipelines.json` | `f29c183391c77efa3d654abd94cbe543616b55d7cf88b4e212c5e3495d447287` |

Полный машинный журнал: [runtime-report.json](../validation/pipeline-runtime/runtime-report.json). Отдельно сохранены [хеши результатов](../validation/pipeline-runtime/artifact-hashes.txt), [определение форматов](../validation/pipeline-runtime/file-types.txt), [окружение](../validation/pipeline-runtime/environment.txt), [зависимости BFlat](../validation/pipeline-runtime/bflat-dependencies.txt), [cache end-to-end](../validation/pipeline-runtime/cache-e2e-report.json) и [проверка ссылки `ps_sct_file`](../validation/pipeline-runtime/ps-sct-reference.txt).

## Матрица результатов

Два значения SHA-256 в одной ячейке относятся к двум последовательным прогонам с одинаковым логическим входом.

| Pipeline | Вход | Выход | SHA-256 | Ограничения | Воспроизводимость |
|---|---|---|---|---|---|
| C# compiler | `Fixture.Value() => 42`, library | PE/.NET, 5 120 байт | `c6af0c11823d78ad3396c97d9620b4d2a8a3172250bee6a9a2a53baf6fb57a91`; `f4306fc9685b40a1c0dbb6712dd1d633d47f9369a0a34bf50a3cf87ae00c0056` | Shipped Roslyn transformer; DLL не загружалась | Нет, хеши различаются |
| BFlat / NativeAOT | C# `Main` возвращает `0` | ELF x64, 3 770 408 байт | `2ba028463dc64142565246c9fe407fdf57c796a78145d7c81a81aca80de60833`; `4b4acc5228235f11a622429411ae3479b921ea43e334d19ac34e3663b98e8078` | В `/tmp` извлечены Fedora `libc++`/`libc++abi`; у shipped `lld`/`llvm-objcopy` восстановлен executable bit; ELF не запускался | Да функционально, нет побайтово |
| Zig Linux | `pub fn main() void {}` | ELF x64, 4 176 байт | `53cdd870aee55789db86524576dc5f320fb7299e1d504e1ad68282a4766a7ed4` в обоих прогонах | ELF не запускался | Да, побайтово |
| Zig Windows DLL | inert export `Run`, возвращающий `true` | PE32+ DLL, 4 608 байт | `b790c6dba6abec2c199f11c300f53a824578001b4dd0691987e4a71b9458dabb`; `3844790216629e9f3e7472f82ba2c19da20d95b6d9660dd396c2ab2192463e2a` | DLL использовалась только как вход SRDI | Нет, хеши различаются |
| Zig raw-section extractor | freestanding `_start`, только бесконечный цикл | raw `.text`, 6 байт: `55 48 89 e5 eb fe` | `423d0f940572195de35a22e8ba0f88e4deb358e980007119ccd19eea6321d8b1` в обоих прогонах | Shipped linker script и `zig objcopy`; результат не исполнялся, PE/ELF magic отсутствует | Да, побайтово |
| PowerShell remove-comments | локальный benign script | UTF-8 PowerShell, 46 байт | `548d0bb4e9ed0801adfb2f2a51be33dbccf39acc6772dbb443da72cbdb011ca2` в обоих прогонах | Результат не исполнялся | Да, побайтово |
| PowerShell string-base64 | тот же benign script | UTF-8 PowerShell, 672/654 байта | `f4bb54ff0bc154ff81c80a88a9c9e518b7a843dfd1934308f69ddc4bf2ddd93e`; `5ff6558c4a7adfe8e3378a81f48b13d8fa856c29f39d38041a392f455264d50a` | Random offset и имена helper/variables; результат не исполнялся | Нет, ожидаемая рандомизация |
| SRDI | первая inert Zig x64 DLL, export `Run`, user data `fixture` | raw wrapper, 6 676 байт | `b79b7e9da3337cb7f3f888326b2e6478851ea65a9f52fe78b006fd86cd691143` в обоих прогонах | Wrapper не загружался и не исполнялся | Да для идентичного входа |
| PE template injection x86 | shipped x86 EXE + inert pattern 4 096 байт | PE32, 130 048 байт | `633e815995c4d386a8fa61a847c48237e0a1a46a8db98f29efcfbaf66241461d`; `37962cee50f3edcea2f589d6b6bdfbb2952681a2dd867d83bb3b68310dbb4fa3` | Проверены точные marker/update primitives; AsmResolver rewrite и исполнение исключены | Нет: новая substitution map |
| PE template injection x64 | shipped x64 EXE + inert pattern 4 096 байт | PE32+, 133 632 байт | `ed44e850c7b65b8254f9dfe7b85254261eb4e16076c43953c6ccb37f2bde870b`; `5459fd6c1c4892630d4ef53e3d184c3d1f282b7ed507e79fef2f5fde32930c59` | Те же границы | Нет: новая substitution map |

## Поведенческие проверки

### Multi-output и cancellation

Benign PowerShell expression `'first'; 'second'; [byte[]](1,2,3)` вернул пять объектов: две строки и три отдельных `System.Byte`. Первый объект — `first`. Это подтверждает runtime-границу `PayloadPipelineService`: `First()`/`FirstOrDefault()` игнорирует все последующие объекты, а `byte[]`, записанный в pipeline без unary comma, разворачивается по элементам.

Для `Start-Sleep -Seconds 30` cancellation была запрошена через 250 мс тем же механизмом `PowerShell.Stop()`. Runspace перешёл в `Stopped` за 262 мс, exception и error-stream отсутствовали. Это подтверждает остановку данного benign сценария, но не является доказательством завершения произвольных дочерних процессов pipeline.

### SRDI

Два вызова на независимых копиях одного PE дали одинаковый результат. Одновременно подтверждён побочный эффект: `ConvertToShellcode` меняет первые два байта переданного массива DLL с `4d 5a` на `00 00`. Поэтому повторное использование того же массива как исходного PE небезопасно; тест использовал отдельную копию на каждый вызов.

### Marker capacity и pointer-sized tail

Payload marker находится в `.rdata`. Измеренная непрерывная граница от marker до конца raw-секции составляет 120 776 байт для x86 и 120 904 байта для x64. Это физическая граница секции, а не объявленная в исходнике семантическая ёмкость.

На копиях обоих шаблонов:

- копирование ровно до конца `.rdata` завершилось успешно;
- копирование `capacity + 1` также не вызвало исключения, потому что backing file длиннее;
- первый байт следующей секции фактически изменился с `0x00` на тестовый `0x5A`;
- `CompileAsync` не выполняет предварительную проверку границы секции.

`BinaryParameters.TryUpdatePayload` оставил последние `IntPtr.Size == 8` байт параметрического блока неизменными в x86 и x64 шаблонах. На x64 Linux builder размер хвоста определяется архитектурой процесса, а переданный `Architecture` в этом методе на размер копирования не влияет.

### Cache races и timer

Runtime reflection использовала настоящий private `PayloadCacheKey` и `PayloadCacheValue`. Подтверждено:

- `emptyKey.Equals(extraParameterKey) == true`, но обратное сравнение ложно;
- при 24 одновременных miss по схеме сервиса `TryGetValue → generate → TryAdd` выполнены 24 генерации, только один `TryAdd` успешен, а callers получили 24 различных candidate outputs;
- настоящий 180-мс timer вызвал callback один раз примерно через 182 мс;
- `ResetExpiration`, вызванный через 100 мс, перенёс callback примерно на 281 мс от создания.

Отдельный end-to-end probe использовал одноразовую PostgreSQL 16.4 только на loopback и инертный четырёхбайтовый pipeline. Наблюдалась реальная цепочка `PayloadCachingService → PayloadsService → PayloadPipelineService.RunPipelineAsync → RunspacePoolService`: после первого заполнения timer удалил запись и фоновая задача вернула её в cache. В БД зафиксированы два `payload-generated`, два `payload-cached` и один `payload-cache-expired`; SHA-256 до и после регенерации одинаков — `9f64a747e1b97f131fabb6b447296c9b6f0201e79fb3c5356e6c77e89b6a806a`. Затем policy переключена на десять минут без regeneration, контейнер остановлен и автоматически удалён.

Конструктор `PayloadPipelineService` в probe намеренно не вызывался, чтобы исключить его несвязанный process-patch routine. Поля зависимостей были установлены reflection, но использовались настоящие `RunPipelineAsync`, методы `PayloadsService`, cache timer, event logging и PostgreSQL-запросы. SignalR hub отсутствовал; `EventLoggingService` штатно поглотил эту ошибку, а запись в БД осталась активной.

## Неразрешённые ссылки pipeline

`ps_generic_cradle` отсутствует как имя definition, но документация существующего `ps_generic_stager` сама продолжает использовать это имя, а его script block реализует ожидаемый URL-cradle contract. Наиболее согласованное объяснение — устаревшее имя/alias `ps_generic_stager`; отдельного определения в поставке не найдено.

Для `ps_sct_file` получен конечный результат по поставленному образу: это dangling static reference. В `specter-pipelines.json` имеется ноль definitions с таким именем и ровно один consumer — `ps_cmstp_file`; локальная БД/дамп, backup, server update либо иной авторитетный definition отсутствуют. `generic_sct_file` — отдельный параметризованный pipeline и не доказывает идентичность отсутствующему definition. Никакая предполагаемая SCT-композиция не создавалась и не запускалась. Восстановление исторического замысла возможно только по другому релизу/снимку БД и относится к provenance, а не к незавершённой runtime-проверке этого образа.

## Воспроизведение

Стенд находится в [research/examination/pipeline-runtime](../examination/pipeline-runtime). Он компилирует только локальные fixtures, пишет результаты в caller-selected `/tmp` и не запускает полученные PE/ELF/raw bytes:

```text
bash research/examination/pipeline-runtime/run-safe-tests.sh /tmp/specter-pipeline-runtime
```

Для BFlat `SPECTER_BFLAT_LIBRARY_PATH` указывает на временно извлечённые `libc++.so.1` и `libc++abi.so.1`; пакеты и хеши перечислены в evidence. End-to-end cache probe находится в `cache-e2e/` и требует только caller-provided connection string к одноразовой локальной PostgreSQL.

Авторитетный список незакрытых вопросов ведётся только в [gaps.md](../validation/gaps.md); этот отчёт фиксирует результаты и ограничения наблюдений, но не является вторым реестром.
