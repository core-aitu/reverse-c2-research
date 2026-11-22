# План полного реверсинга

Этот документ фиксирует закрытые статические этапы и не ведёт параллельный backlog. Статус `static-complete` означает, что для выбранного бинарного образа восстановлены относящиеся к этапу статические пути и форматы. Он не означает наблюдение реального исполнения. Единственный реестр незакрытых вопросов — [gaps.md](validation/gaps.md).

## Закрытые статические этапы

| Этап | Статус | Доказательство |
|---|---|---|
| Managed Beacon `d5e7…569d` | static-complete | PDB связан с PE; все 65 документов совпали с `templates/source`; описаны lifecycle, timers, task execution, settings и оба транспорта. |
| C2 wire protocol | static-complete | Клиентские исходники и серверная ILSpy/REA-реконструкция согласуются по framing, девяти командам, HTTP-маршрутам, сериализации и подписям. |
| Server: C2 ingress/dispatch | static-complete | `SslC2Protocol`, `RawSslImplantServer`, `KestrelImplantServer`, `ImplantHandlerService` привязаны к SHA-256/MVID Server.dll. |
| Payload pipeline | static+isolated-runtime-complete | 117/117 элементов разобраны через AST; на инертных fixtures проверены C#, BFlat/NativeAOT, Zig, SRDI, PowerShell transforms, PE injection и cache regeneration. Полученные бинарники не исполнялись; `ps_sct_file` — dangling reference текущего manifest. |
| Server: проверка предупреждений ILSpy | static-complete | 44/44 файла, 67 MethodDef и 184 места `IL_xxxx` сопоставлены с исходным IL; все смещения валидны, два независимых декодера согласуются по 10 271 телу Server.dll. |
| Client UI: проверка предупреждений ILSpy | static-complete | 3/3 файла и 18 мест с предупреждениями проверены по исходному IL. |
| Management REST | static-complete | Статическая матрица закрыта для 202/202 generated REST-операций: каждая клиентская операция сопоставлена с server action. |
| Server services | static-complete + isolated-runtime-partial | Статически разобраны 73/73 implementations. Последовательно выполнены 12 изолированных integration-сценариев с коррелируемыми логами; внешние/платформенные успешные ветки остаются только в `gaps.md`. [Runtime-отчёт](analysis/service-runtime.md). |
| SpecterInsight.Obfuscation | static-complete | 837/837 TypeDef и 7 673/7 673 MethodDef разделены по функциональным группам; 148 transform-related типов связаны с форматами, конфигурацией, pipeline/compiler paths; 5 критических методов подтверждены по IL. |
| Modules | static-complete | Разобраны 22/22 DLL: 191 PSCmdlet, входы/результаты, все P/Invoke и AssemblyRef, системные эффекты; module carriers отделены от 6 dependency-only файлов, явные third-party identities отмечены отдельно. |
| Managed Beacon variants | static-complete | Четыре варианта без PDB сопоставлены с `d5e7…569d` по method identity и normalized IL; опубликованы изменённые/совпадающие методы и различия configuration/transport/commands/platform paths. |
| Client UI workflows | static-complete | Матрица закрыта для 24/24 workflow; semantic review завершён для 11 Services, 13 Utilities и 128 ViewModels (152/152 файла). |
| Этап 5: PostgreSQL, SignalR и Avalonia UI | runtime-complete для лабораторной цепочки | Каталог чистого стенда: 36 таблиц, 10 enum, 81 индекс; 24/24 workflow rendered; login/navigation/logout пройдены; `fixture-live-window` дошёл до открытого окна. Automatic reconnect после рестарта узла не прошёл и записан как FND-057. |
| Этап 8: зашифрованные архивы | static-complete в доступной границе | Для двух ZipCrypto восстановлен `Ukraine2026`; DOCM/XLSM извлечены изолированно, OOXML/VBA/relationships хешированы и разобраны без исполнения. Для третьего ZIP пароль не найден, но подтверждён исходный notebook-путь; plaintext явно оставлен неизвестным. |

## Критерий завершения

Этап закрывается только при наличии: точной идентичности артефакта, воспроизводимого выхода инструмента, документа с наблюдениями/выводами/неизвестным и обновлённого реестра OBS/FND. Автоматически сгенерированный C# сам по себе этап не закрывает.

Подробная [статическая спецификация managed-компонентов](analysis/managed-components-static-spec.md) содержит воспроизводимые JSON-индексы и предметные отчёты. Незакрытые runtime-проверки и ограничения перечислены только в [gaps.md](validation/gaps.md).
