# Этап 8: зашифрованные архивы и Office-вложения

## Граница и метод

Три ZIP скопированы из проверенного `specter-study.tar.zst` в `research/examination/stage8-archives/input/`. Два доступных вложения извлечены в отдельные подкаталоги `extracted/`, а их OOXML-части — в `office-parts/`. Эти каталоги исключены из Git. Microsoft Office не запускался; VBA, PowerShell и вложенные команды не исполнялись. Анализ ограничен центральным каталогом ZIP, XML/relationships, структурой OLE Compound File и статическим декодированием VBA source streams.

Воспроизводимые результаты: [состояние архивов](../examination/stage8-archives/archive-analysis.json), [OOXML/VBA-инвентарь](../examination/stage8-archives/office-analysis.json), [цепочка notebook](../examination/stage8-archives/chain-analysis.json), [анализатор](../examination/stage8-archives/analyze_office.py). REA Evidence: `ev_37a78…afd53`, `ev_d2dce…92b76`, `ev_b3a19…cf04f`, `ev_4ee0…058aa`.

## Состояние паролей и происхождение

| Архив | SHA-256 ZIP | Состояние | Результат |
|---|---|---|---|
| `Joint_NATO_Training_Program_2026_Invitation.zip` | `da47ad640d548054176fe97f59f2985218b483dc65705313aa517efe810042db` | пароль восстановлен | `Ukraine2026`; DOCM извлечён и статически разобран |
| `Court_Summons_Notice_UA_2026.zip` | `e28f1a5e449b69f6e0e9b0fa9cbef6372bc829fe2ce19134a84a76f42fa0143b` | пароль восстановлен | `Ukraine2026`; XLSM извлечён и статически разобран |
| `National_Security_Commission_Meeting_Agenda_2025.zip` | `c2d5083e0a9b73dd682ccaa4fb8b141ea4593dc3eb636893f15dbe67738eae79` | пароль не восстановлен; локальное происхождение установлено | исходный путь `notebooks/3f8a9c2e-7d4b-4f1a-b6e8-5c0d9a2e3f7b/files/…`; содержимое не извлечено |

Все три контейнера имеют один упаковочный профиль: единственный encrypted member, ZipCrypto/Deflate, creator version 4.5, host FAT, NTFS extra field `0x000a`, без data descriptor и комментария. Это поддерживает общее происхождение из одного процесса подготовки, но не устанавливает автора или внешний источник. Для третьего ZIP проверены 101 126 уникальных кандидатов из локальных словарей и 1 345 379 тематических вариантов; пароль не найден. Поиск точного имени и SHA-256 во внешнем индексе совпадений не дал. Проверенная граница происхождения — исходный каталог `/home/deleteme/Desktop/SpecterInsight` и снимок, начатый `2026-11-12T19:50:00.699736Z`; более ранний канал доставки неизвестен.

## `Joint_NATO_Training_Program_2026_Invitation.docm`

Вложение: 21 893 байта, SHA-256 `66b8169ccf8db86524813f22ff1ba3eb88af8d658098009c727ec6cdc40b523f`, CRC32 из внешнего ZIP `bcacc85f`. В `docProps/core.xml` указаны creator/lastModifiedBy `Michael Lester`, created/modified `2026-08-05T23:59:00Z`. Видимое содержимое `word/document.xml` пусто.

`VBA/Module1` содержит `AutoOpen`: создаётся `WScript.Shell`, затем скрыто запускается `powershell.exe -ExecutionPolicy Bypass -EncodedCommand`. Статическое декодирование даёт PowerShell, который отключает проверку TLS-сертификата, восстанавливает `DownloadString` и `System.Net.WebClient`, загружает строку с

`https://192.168.1.101/static/resources?build=PrismexNotebook&kind=ps_script_obfuscated`

и передаёт её в `[scriptblock]::Create(...).Invoke()`. Это восстановленный статический путь; сетевой запрос и загруженный код не выполнялись. SHA-256 декодированных bytes PowerShell — `d6ec34921e5ac00bd80436a4e94a22c87fdabcb3c468895b0be67eebaaa6b96e`.

Все 10 relationships внутренние. Внешних relationships, каталогов `embeddings/` и OLE embedded objects нет. Лежащие рядом в notebook-каталоге `payload_update.ps1` (`a12b7649…4426e`) и `extracted/Joint_NATO_Training_Program_2026_Invitation.ps1` (`8d06036a…d13a`) не являются членами DOCM или внешнего ZIP; их соседство — контекст происхождения, а не доказанная цепочка извлечения.

## `Court_Summons_Notice_UA_2026.xlsm`

Вложение: 6 795 байт, SHA-256 `e9f4de063cfab93117a88989b28012bf025d788a22263d9a7d0b9302f1f97379`, CRC32 `76e12efc`. Workbook содержит один пустой `Sheet1`.

`VBA/Module1` также содержит `AutoOpen`, создаёт `WScript.Shell` и формирует скрытый `powershell.exe -ExecutionPolicy Bypass -EncodedCommand`. В восстановленной части PowerShell отключается проверка TLS-сертификата и собираются `DownloadString` и `System.Net.WebClient`. VBA source stream намеренно или вследствие повреждения имеет неканонический chunk header `0x8ffc` с signature bits `0`, поэтому стандартно восстановить весь source stream нельзя. Из первого объявленного chunk получены 3 841 байт source и 3 334 символа Base64; SHA-256 частично декодированных bytes — `38de924308281ecde28aacb39f1ebf5c1c1d3447be920c727b16144c21de926f`. Полный URL и соответствие source с compiled p-code остаются неизвестными; вывод об исполнении не делается.

Все три relationships внутренние. Внешних relationships и embedded objects нет.

## `National_Security_Commission_Meeting_Agenda_2025.xlsm`

Внешний ZIP объявляет один member размером 7 496 байт и CRC32 `b9051de7`. NTFS extra field фиксирует создание, доступ и изменение member около `2026-07-10T00:52:16.364Z` (DOS/local time внешнего ZIP — `2026-07-09 20:52:16`). Пароль не найден, поэтому SHA-256 plaintext, OOXML parts, VBA, relationships и embedded objects для этого вложения неизвестны. Отсутствие результата не интерпретируется как отсутствие макросов или объектов.

### Где архив используется

**Наблюдение.** `ServerPaths.GetNotebookFilesFolder` строит путь `notebooks/<notebookId>/files`. При создании notebook-сессии `PowerShellNotebooksController` передаёт этот путь в `EnsureNotebookSessionInitializedAsync`, а сервис задаёт `$NotebookFolder` и меняет на него текущий каталог PowerShell. Поэтому размещение ZIP в `notebooks/3f8a…/files` точно соответствует штатному файловому пространству notebook `3f8a…`, а не произвольному каталогу с похожим именем. Создание файла именно notebook-cell остаётся выводом, а не прямым наблюдением.

Серверный `NotebookFilesController` перечисляет файлы верхнего уровня и предоставляет авторизованные GET/DELETE маршруты `NotebookFiles/{notebookId}` и `NotebookFiles/{notebookId}/{filename}`. Клиентский `NotebookFilesService` скачивает выбранный файл в локальный notebook-каталог; `PowerShellSessionViewModel` показывает список, вызывает скачивание и открывает локальную папку. Это доказывает предусмотренную цепочку выдачи ZIP оператору, но не доказывает фактическое скачивание именно этого файла.

**Ограничение.** UUID `3f8a…` отсутствует в текущем `dependencies/specter-notebooks.json` (SHA-256 `982b9b…6eaf`), где осталось три других notebook. Это не означает, что notebook никогда не существовал: загрузчик built-in notebook только добавляет отсутствующие строки, а удаление notebook удаляет строку из PostgreSQL, но не каталог `notebooks/<id>/files`. Исторический row/cells БД в снапшот не вошли. Ограниченный поиск по всему снимку не нашёл независимого текстового упоминания UUID или имени архива за пределами пути и ZIP metadata.

### Реконструированная цепочка

**Высоковероятная часть:** `notebook cell → XLSM-producing pipeline → New-ExcelWorkbook → generic_zip_encrypted/New-EncryptedZip → WriteAllBytes($NotebookFolder/…zip)`. `NewEncryptedZip` в исследованной Server.dll передаёт bytes, filename и password в DotNetZip `AddEntry` и сохраняет ZipCrypto по умолчанию. Наблюдаемый одновложенный профиль National Security ZIP совпадает с двумя соседними архивами; для NATO тот же `generic_zip_encrypted → ps_word_document → WriteAllBytes($NotebookFolder)` непосредственно записан в поставляемом notebook.

**Неустановленная развилка:** из-за шифрования нельзя выбрать точный XLSM source pipeline. Кандидаты поставки включают `ps_excel_workbook`, `generic_excel_workbook`, `download_run_excel_workbook`, `download_inject_excel_workbook` и `excel_xlm_command`. Сходство с расшифрованным Court XLSM поддерживает `ps_excel_workbook`, но не доказывает его для National Security.

**Доставка:** доказана только возможность `NotebookFilesController → NotebookFilesService → локальная копия оператора`. В PRISMEX notebook после аналогичной генерации следует `PhishingAttachment → Send-PhishingEmail`, однако для `3f8a…` соответствующая cell, письмо, API/audit event или network record отсутствуют. Поэтому ветка SMTP для последнего ZIP остаётся гипотезой, а не частью установленной цепочки.

Из явных паролей текущих notebook/pipeline (`Ukraine2026`, `Summer2025!`, `1qaz!QAZ`) ни один не подошёл; это согласуется с прежним отрицательным поиском по словарным и тематическим кандидатам.

## Хеши VBA

| Документ | Объект | Байты | SHA-256 |
|---|---|---:|---|
| DOCM | `word/vbaProject.bin` | 16 896 | `c87ceaf027ec6ccffa5b67f8091057d480e137f61a5d3093aaa5fa9fee56398b` |
| DOCM | `PROJECT` | 406 | `cfe6551b6b30cd80db9d21d6b404a21ef685a55847112c4e53119aafc581d18a` |
| DOCM | `PROJECTwm` | 65 | `0271f2e38221aa2a4f90c6a56d864663f20a96960160dd83ad7e8bdbae71d3e0` |
| DOCM | `VBA/Module1` | 8 917 | `759b0519686c8bfb808bbea835df8734696786e05e558c893f8406ce28a31e3d` |
| DOCM | recovered `VBA/Module1` source | 5 247 | `5fd8f5cb48d59de8d6af4705fd00d493eee765c0c058422e75e6c7c4dd9e5152` |
| DOCM | `VBA/ThisDocument` | 938 | `f20e26efcf6da247f1a26ce55fd8fbca3aec807b470edb15c81722de4b6656ce` |
| DOCM | recovered `VBA/ThisDocument` source | 285 | `76f615001478cc8d4cce6fe76f75b2ca62880736645e205dfce15fc20f92e650` |
| DOCM | `VBA/_VBA_PROJECT` | 2 583 | `61b9a171477b5a59b00f66688b6efeaff8df79308ca2b3559e4ee2a370058a34` |
| DOCM | `VBA/dir` | 563 | `12044ac187799e2fe3e566cb668fa1db2b001d5ce4600df86eac0cd7699e18a5` |
| XLSM | `xl/vbaProject.bin` | 14 336 | `c1076828a7fc8c7ff84b044367a7b005d2032ea2488568bdd6a2d0cf4f636f69` |
| XLSM | `PROJECT` | 192 | `2d58ef27ff1d643d3ae6a740e0c5c6353a106645872d1f575e83b78927a9c5ce` |
| XLSM | `PROJECTwm` | 85 | `3ea8d99fabcdb71f68d427de243154e53fa23d78295617a78337b872ead44a61` |
| XLSM | `VBA/Module1` | 5 100 | `de7a0564209c545c7d57c7ee7f255193f662323a6cde87e0050fcdfa9d4c0472` |
| XLSM | partially recovered `VBA/Module1` source | 3 841 | `b003391c845c14bc55cd1bfead7805c55f99a8f43733c28c3a4cf70573296b42` |
| XLSM | `VBA/Sheet1` | 991 | `9b9efce39615268e26d484776615f83aaab0abda83c7555eff5933da38a32f83` |
| XLSM | recovered `VBA/Sheet1` source | 299 | `8e442a8e542ca01c7d87af7e9ad015863e0a19232a647712a6e31b3911beb372` |
| XLSM | `VBA/ThisWorkbook` | 1 188 | `e0b2e5d328f087bbef66d938b6b64bc56cb0068a59e7fa5bd8a9ae1f67475b43` |
| XLSM | recovered `VBA/ThisWorkbook` source | 305 | `14de0425a62586687c3d59b7d3d7dc60268f989ab7e07a61403525064d98502a` |
| XLSM | `VBA/_VBA_PROJECT` | 2 570 | `55b026d06f0d273ae74be65513abb16fbe448883407af17d906d514c76df53fa` |
| XLSM | `VBA/dir` | 567 | `5a6714c31cd36f61a67d4c99bf531fa403dca1d5a5f731db172b8023165db010` |

## Relationships и embedded objects

| Документ | Relationships part | SHA-256 | Записи |
|---|---|---|---:|
| DOCM | `_rels/.rels` | `e19238d7a71fa7a2490776252686f70e2de6238c87cd509b5e3a3cc07c2ea4df` | 3 |
| DOCM | `word/_rels/document.xml.rels` | `49d2bc305daf1fcca84d6a282e52d1dfd2d79e9ca9f96a3a435d058ae2d8f755` | 6 |
| DOCM | `word/_rels/vbaProject.bin.rels` | `c97833e6456aa2bfe9be614f9c3ae41a8ef764b1cc3af92c6a6f273c62309122` | 1 |
| XLSM | `_rels/.rels` | `0f1a595970d369fa6b683d76ca7fb977bd6cb64e7d8fd890722532683b765125` | 1 |
| XLSM | `xl/_rels/workbook.xml.rels` | `635db5ce19fe86b425948e5e83a0288584c6b759d1eb0f025fbd1b29645359d0` | 2 |

Все targets разрешаются внутри соответствующего OOXML-контейнера; `TargetMode="External"` отсутствует. Число embedded objects для доступных DOCM/XLSM — **0**; поэтому отдельных хешей embedded objects нет. Полный пофайловый список OOXML parts и их SHA-256 сохранён в `office-analysis.json`.

## Итог по вложениям

| Вложение | Статус |
|---|---|
| NATO DOCM | static-complete для OOXML, relationships и VBA source; автостарт и download/execute chain восстановлены без исполнения |
| Court XLSM | static-partial: OOXML/relationships полны, VBA project хеширован, source `Module1` восстановлен частично из-за неканонического chunk header |
| National Security XLSM | encrypted-unavailable: известны только объявленные имя, размер и CRC32; локальное происхождение контейнера установлено, пароль и plaintext неизвестны |
