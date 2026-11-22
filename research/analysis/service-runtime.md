# Runtime-проверка серверных интеграций

Дата прогона: 2026-11-22. Все сценарии выполнялись последовательно в одноразовом окружении. Внешние рабочие C2, реальные учётные записи и внешние цели не использовались: успешные ветки работали с loopback-сервисами/контейнерами, а hard-coded публичные endpoints перехватывались локальным reject-proxy и не форвардились.

## Итоговая матрица

| № | Интеграция | Runtime-статус | Вход | Сеть | БД / файлы | SignalR | Ошибки, retry и cleanup |
|---:|---|---|---|---|---|---|---|
| 1 | PostgreSQL и фоновые очереди | Passed | `runtime_cache_e2e`, `01020304`, TTL 700 ms | PostgreSQL 16.4, loopback | 6 событий cache lifecycle; 1 запись cache после regeneration | 0: hub намеренно отсутствовал | 1 background regeneration; hash совпал; policy восстановлена |
| 2 | Elasticsearch / Kibana | Passed | benign `PSObject`, индекс `specter-runtime-fixture` | 4 Elastic-запроса и 2 Kibana GET, только Docker loopback | template + index + 1 document; dashboards 0 | 0 | retry 0; `EnsureDashboardsExistAsync` не сделал сетевых вызовов |
| 3 | ACME | Bounded failure | `.invalid` email, `CreateAcmeAccount` | 1 CONNECT к Let's Encrypt перехвачен, 502, forward=0 | account/cert/PFX: 0 | 0 | retry 0; успешная выдача не проверялась |
| 4 | WebDAV | Passed для service layer | файл 22 bytes | 0 | create/read/enumerate/delete; SHA-256 совпал | `WebDavFilesChanged` ×3 | retry 0; весь временный root удалён |
| 5 | SMTP / IMAP / phishing | Passed / IMAP absent | 2 `.invalid` адреса, attachment | EHLO → MAIL → RCPT×2 → DATA → QUIT, loopback | 2 audit rows → 0; EML 658 bytes | 0: hub отсутствовал | retry 0; IMAP-клиента в shipped server нет |
| 6 | AI providers / MCP | Passed с подавленными provider failures | fake keys, local Ollama, harmless PowerShell function | Ollama GET 200; OpenAI/Anthropic CONNECT перехвачены, forward=0 | AI function 1 → 0 | 0 | `ListAllModelsAsync` исключил failed providers; MCP output `hello fixture` |
| 7 | Tor | Bounded failure | SOCKS 39008, hard-coded authorities | 14/14 authority GET перехвачены, 7 unique, forward=0 | 0 | 0 | две серии failover по 7; после failure `SocksPort=39008`, `IsRunning=false` |
| 8 | SSH | Passed | Ed25519, benign command, 24-byte file | реальный OpenSSH на loopback | 2 tasks, 4 events; session/tasks/events очищены; remote file удалён | 10 сообщений | retry 0; download hash совпал |
| 9 | WinRM | Bounded failure | fake NT hash, local `/wsman` | 2 POST: без/с Authorization, loopback | 1 auth-failure event → 0; sessions 0 | `EventNotification` ×1 | incomplete NTLM challenge; реальный Windows endpoint не использовался |
| 10 | tunnels | Partial | fixture session, local port 39012 | listener и 1 connect внутри отдельной network namespace | DB 0: namespace отрезал PostgreSQL | 6 сообщений | listener закрыт; обнаружена оставшаяся `TunnelPair` entry |
| 11 | updates | Safe subpaths passed | client fixture 29 bytes | Dropbox CONNECT перехвачен, forward=0 | client ZIP 151 bytes создан/прочитан/удалён | 0 | retry 0; destructive update script не запускался |
| 12 | loaders | Passed | staged runtime/deps ZIP, Linux x64 | 0 | staged files удалены; assembly/content hashes зафиксированы | 0 | cancellation observed; flat bundle одинаков в двух процессах |

## Коррелируемые наблюдения

### 1. PostgreSQL и фоновые очереди

`PayloadCachingService → PayloadsService → PayloadPipelineService → RunspacePoolService` дважды вернул payload SHA-256 `9f64a747e1b97f131fabb6b447296c9b6f0201e79fb3c5356e6c77e89b6a806a`. Между вызовами истёк TTL и один background worker выполнил regeneration. PostgreSQL получил `payload-generated` ×2, `payload-cached` ×2, `payload-cache-expired` ×1 и `payload-retrieved-from-cache` ×1. [Evidence](../validation/service-runtime/01-postgres-queues.json)

### 2. Elasticsearch и Kibana

Реальный `ElasticService` через capture-proxy выполнил `PUT /_template`, `HEAD`/`PUT` индекса и `POST _doc`. Elasticsearch подтвердил один документ; Kibana 8.15.1 ответила `available`, но dashboard count остался нулевым. Это runtime-подтверждает статический вывод, что `EnsureDashboardsExistAsync` пуст. [Summary](../validation/service-runtime/02-elasticsearch-kibana/summary.json) · [HTTP log](../validation/service-runtime/02-elasticsearch-kibana/http.jsonl)

### 3. ACME

`AcmeService.CreateAcmeAccount` попытался открыть TLS tunnel к hard-coded `acme-v02.api.letsencrypt.org:443`. Reject-proxy вернул 502; сервис не создал строку `AcmeAccount` и файлы сертификата. Проверена только безопасная failure-ветка. [Summary](../validation/service-runtime/03-acme/summary.json) · [Proxy log](../validation/service-runtime/03-acme/http-proxy.jsonl)

### 4. WebDAV

Service layer создал `fixture/sub/hello.txt`, прочитал те же 22 bytes, построил tree/list и рекурсивно удалил fixture. Входной и прочитанный SHA-256: `6922018b292e2d72a1fd8bdfd26acefa6ef09bf7b68053c8154d8e6611277fa1`. Listener-side `OPTIONS/PROPFIND/GET/HEAD` не поднимались. [Evidence](../validation/service-runtime/04-webdav.json)

### 5. SMTP, IMAP и phishing

`PhishingService` передал одно MIME-сообщение локальному SMTP server; transcript содержит полный SMTP envelope и две команды `RCPT TO`. После успешного DATA появились две строки `phishing-email-sent`, затем были удалены. По shipped managed-коду IMAP runtime отсутствует: IMAP-поля только хранятся в metadata email account. [Summary](../validation/service-runtime/05-smtp-imap-phishing/summary.json) · [Transcript](../validation/service-runtime/05-smtp-imap-phishing/transcript.jsonl) · [EML](../validation/service-runtime/05-smtp-imap-phishing/message.eml)

### 6. AI providers и MCP

`ListAllModelsAsync` параллельно вызвал три provider path. Ollama mock вернул `fixture-a` и `fixture-b`; OpenAI и Anthropic были заблокированы proxy. Итоговый список содержал только Ollama, то есть provider exceptions подавляются. `McpToolRegistrationService` зарегистрировал одну сохранённую функцию и через execution core/`AiToolHost(AutoApproveAll)` выполнил harmless definition с результатом `hello fixture`. Synthetic MCP session не существует в `AiSessions`, поэтому best-effort tool-call persistence была подавлена до SignalR broadcast. [Summary](../validation/service-runtime/06-ai-mcp/summary.json) · [Ollama log](../validation/service-runtime/06-ai-mcp/ollama.jsonl) · [Provider proxy](../validation/service-runtime/06-ai-mcp/proxy.jsonl)

### 7. Tor

Конструктор `TorService` сам запускает background consensus load; затем явный `StartAsync` повторяет тот же failover. Поэтому наблюдались две серии по семь authorities. После общего failure сервис был остановлен и disposed, но `SocksPort` сохранил запрошенное значение 39008 при `IsRunning=false`. [Summary](../validation/service-runtime/07-tor/summary.json) · [Network log](../validation/service-runtime/07-tor/network.jsonl)

### 8. SSH

Реальный `SshSessionService` прошёл private-key authentication к одноразовому OpenSSH, выполнил benign command, загрузил файл chunked-base64 методом, скачал его и удалил на remote loopback host. `RemoveSession` установил `EndedAt`; удаление session каскадно удалило две task rows. [Summary](../validation/service-runtime/08-ssh/summary.json) · [sshd log](../validation/service-runtime/08-ssh/sshd.log)

### 9. WinRM

Проверен custom PtH HTTP path: первый POST получил `WWW-Authenticate: NTLM`, второй содержал Authorization header. Mock намеренно не выдал полноценный Type2 challenge; `AuthenticationException` породил одну DB event и одну SignalR notification, затем DB event была удалена. Значения Authorization не журналировались. [Summary](../validation/service-runtime/09-winrm/summary.json) · [Network log](../validation/service-runtime/09-winrm/network.jsonl)

### 10. Tunnels

`TunnelService` открыл `0.0.0.0:39012`, но процесс находился в отдельной network namespace, где существовал только loopback. После synthetic connect появилась одна `TunnelPair`; forwarding ожидаемо завершился failure из-за отсутствия реального implant session. `StopTunnelAsync` удалил initiator/listener, но `Tunnels.Length` остался 1 — dictionary entry живёт до завершения процесса. [Evidence](../validation/service-runtime/10-tunnels.json)

### 11. Updates

`EnsureClientZip` создал `client-7.0.0.0.zip`, а `ReadClientZip` вернул 151 bytes, SHA-256 `52597c6354d4cf6b5d186e28baeeea37d538b8ca72bbb5903cd6a0e948469d08`. Version endpoint был заблокирован до внешней сети. `RunUpdateDropbox/RunUpdateScript` не запускался: эта ветка убивает текущий процесс, распаковывает архив поверх root и перезапускает server. [Summary](../validation/service-runtime/11-updates/summary.json) · [Network log](../validation/service-runtime/11-updates/network.jsonl)

### 12. Loaders

`LoaderBundleService`, `LoaderDepsService` и `LoaderAssemblyService` обработали staged ZIPs, вернули shared cached arrays и применили deps-first precedence для одноимённой assembly. Pre-cancelled staged read выбросил `OperationCanceledException`. Flat bundle имел 18,756,584 bytes и одинаковый SHA-256 `63698adfeb5d12bb2fb7d01466831ec350a18345420739bebea7619f5f55851a` в двух свежих процессах. На matching Linux host flat path пакует локально установленный runtime, а не staged runtime ZIP. [Evidence](../validation/service-runtime/12-loaders.json)

## Граница доказанности

Полностью наблюдены loopback PostgreSQL/cache, Elastic/Kibana indexing/status, WebDAV service layer, SMTP send/audit, Ollama listing, MCP registration/execution core, SSH command/file lifecycle, safe update ZIP path и loader services. Открыты только ветки, требующие реальной публичной/платформенной инфраструктуры или разрушительного эффекта: ACME issuance, Tor bootstrap/circuit, успешный Windows WinRM/PSRP, implant-backed tunnel forwarding, listener-side WebDAV HTTP, network MCP handshake и самоприменение update archive. Они перечислены только в [gaps.md](../validation/gaps.md).
