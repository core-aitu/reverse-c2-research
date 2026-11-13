# Как встроенная страница доходит до модели UI

Обычный путь чтения подтверждают две стороны. В UI обнаружены вызовы DocumentationAsync и передача работы диспетчеру. Действие GetAllDocumentation имеет атрибут GET и вызывает GetDocumentation, затем Ok. В асинхронном теле GetDocumentation есть обращение к загрузчику каталога. FND-006. [Атрибуты контроллера](../analysis/server-metadata.json) · [Разобранные места IL](../analysis/method-evidence.json)

В опыте тестовый код соединял компоненты в четырёх местах. Эти соединения показаны явно.

```mermaid
sequenceDiagram
    participant T as Стенд
    participant S as Загрузчик Server
    participant A as ApiClient
    participant H as HTTP-обработчик стенда
    participant U as ViewModel
    T->>S: Передаёт путь к синтетической папке
    S-->>T: Дерево папок и Markdown
    T->>H: Сериализует дерево и задаёт ответ
    T->>A: Вызывает DocumentationAsync
    A->>H: GET /Documentation
    H-->>A: Код состояния и JSON
    A-->>T: Коллекция или ApiException
    T->>U: Добавляет декодированные корни, запрашивает путь
    U-->>T: Узел с Markdown или null
```

Наблюдаемые переходы клиента в этой выборке начинаются с вызова DocumentationAsync. При 200 с массивом задача завершается с результатом. При 200 с null или при 503 задача завершается исключением. Последующий поиск существующего и отсутствующего пути проверен на дереве из успешного ответа. Переходы и исходные данные сохранены в записях FND-007–FND-010. [Журнал](../validation/documentation/run-1.jsonl) · [Исходный код стенда](../validation/documentation/Program.cs)
