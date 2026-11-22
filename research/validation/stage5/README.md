# Этап 5 — PostgreSQL, SignalR и Avalonia UI

Дата наблюдения: **22 ноября 2026 г.** Артефакты: Server `a6c454…e46a5`, UI `b26312…10d6`, ApiClient `5c658f…45422`.

## Итог

В лабораторной чистой PostgreSQL 16.4 штатный `DatabaseService` успешно применил 56/56 DDL-инициализаторов. Получены 36 таблиц, 10 enum, 81 индекс и три установленных расширения (`citext`, `pg_trgm`, `plpgsql`). Отношений с именами migration/version и отдельного журнала миграций нет: поставка использует идемпотентные `CREATE ... IF NOT EXISTS`, а не версионированный migration ledger. Полный каталог с колонками, defaults, constraints и определениями индексов сохранён в [postgres-catalog.json](evidence/postgres-catalog.json).

Штатный процесс `SpecterInsight.UI.dll` запущен в Xvfb. Пройдены EULA, успешный login через локальный TLS+SignalR стенд, открытие меню навигации и logout. Все 24 workflow-контрола отдельно созданы и отрисованы с правильным типом ViewModel; для каждого сохранены размер визуального дерева, длина PNG и SHA-256: [workflow-smoke.json](evidence/workflow-smoke.json).

Сквозное событие `fixture-live-window` прошло по цепочке:

`PostgreSQL event row → штатный EventLoggingService → настоящий HubLifetimeManager/SignalR → штатная подписка ApiClient → EventLogViewModel.OnEventNotification → видимый текст открытого EventLogControl`.

Одна строка с тем же ID найдена в PostgreSQL; ViewModel содержит ID, а окно содержит `Synthetic audit fixture-live-window`. Журнал: [correlated-chain.jsonl](evidence/correlated-chain.jsonl).

## Матрица runtime-проверки

| Проверка | Результат | Свидетельство |
|---|---|---|
| PostgreSQL catalog | Passed для чистого стенда: 36 таблиц, 10 enum, 81 индекс, 3 extensions | `postgres-catalog.json` |
| Фактически применённые инициализаторы | 56/56 passed; migration relation отсутствует | `postgres-catalog.json:create_methods` |
| Запуск и rendering Avalonia | Passed; stock-процесс оставался жив, Xvfb получил кадры | `01-eula.png`, `03-navigation.png` |
| Все 24 workflow | 24/24 rendered; Control visible, non-empty visual tree и PNG | `workflow-smoke.json` |
| Navigation | Passed; видны Operations, Infrastructure, AI, Documentation, Intel, Administration | `03-navigation.png` |
| Login | Passed на TLS+SignalR стенде; Authenticate, negotiate и WebSocket наблюдались | `ui-server-network.jsonl`, `03-navigation.png` |
| Logout | Passed; штатное окно вернулось к login dialog | `06-logout.png` |
| Reconnect | **Failed**: после остановки узла UI стал Disconnected; после запуска узла повторного Authenticate/negotiate не было | `04-disconnected.png`, `05-reconnect-failed.png`, `ui-server-network.jsonl` |
| Локальные настройки | Passed; EULA и client settings созданы и перечитаны | `settings-summary.json` |
| Кеш | In-process cache services сделали authenticated REST reads; при пустых ответах product cache files не появились | `ui-server-network.jsonl`, `settings-summary.json` |
| HTTP errors | Passed: 503 сохраняет прежние Event/Documentation данные; connection error и JSON errors показаны UI | `02-login-http-error.png`, `correlated-chain.jsonl` и прежний desktop-прогон |
| SignalR errors | Disconnect отображён; автоматический reconnect не состоялся | `04-disconnected.png`, `05-reconnect-failed.png` |
| Коррелируемая цепочка до окна | Passed | `correlated-chain.jsonl` |

## Границы

Каталог относится к новой БД, созданной методами именно исследуемой Server.dll. Снимка исторической установки в поставке нет, поэтому прошлые ручные изменения и история внешней БД не устанавливаются. Runtime smoke 24 workflow доказывает создание, привязку ViewModel и rendering, но не утверждает успешное выполнение каждой серверной бизнес-операции. Login/logout и reconnect проверены на локальном стенде протокола, а не на производственной учётной записи.

Два одинаковых кадра состояния Disconnected до и после перезапуска узла имеют SHA-256 `7de35f…35dd7`; сетевой журнал после restart содержит только контрольный curl, но не повторный запрос UI. Это наблюдаемый отрицательный результат reconnect, а не предположение.

## Повторение

Исходники стендов находятся рядом: [Program.cs](Program.cs), [WorkflowSmoke.cs](WorkflowSmoke.cs), [FixtureServer.cs](FixtureServer.cs). Нужны .NET 10 SDK, PostgreSQL 16.4 с `citext` и `pg_trgm`, Xvfb и проверенная копия `copy/client` + `copy/server`. Для сквозной цепочки повторно используется [continuation/loopback](../continuation/loopback/Program.cs); её свежий журнал сохранён здесь без изменения старого протокола.
