# Client и ApiClient

В Main клиент создаёт Avalonia-приложение и передаёт управление StartWithClassicDesktopLifetime. В BuildAvaloniaApp присутствуют вызовы Configure, UsePlatformDetect и UseReactiveUI. Desktop entry связывает этот путь с поставляемой UI.dll. FND-004, FND-014.

В сборке две модели с коротким именем DocumentationViewModel. Одна принадлежит пространству ViewModels, вторая — ViewModels.Controls. Полное имя существенно для воспроизведения исследования. В асинхронных телах обеих найдены вызовы DocumentationAsync и передача работы через диспетчер UI. В варианте Controls дополнительно видны изменения IsLoading. Конструктор этой модели подписывается на таймер и изменение свойства. FND-006.

В изолированном опыте ApiClient разобрал объекты Name, Contents, Children. Корни из его ответа переданы в штатную коллекцию RootEntries модели Controls.DocumentationViewModel. Её метод FindEntryByPath нашёл содержимое существующей страницы и вернул null для отсутствующей. Окно, визуальное дерево и отрисовка Markdown в опыт не входили. FND-008, FND-009.

[Две модели и сигнатуры](../../analysis/named-signatures.jsonl) · [Результат прогона](../../validation/documentation/run-1.jsonl) · [Схема взаимодействия](../../interactions/documentation.md)

## Расширенная проверка

Серверная запись аудита из PostgreSQL прочитана моделью ApiClient. Штатный фильтр UI проверен на совпадениях текста, типа и времени. Исход Failure воспроизводимо получил цвет #e05c5c. Открытие окна и доставка события по сети в стенд не входили. [Путь события](../../interactions/audit-event.md) · [Клиентские проверки](../../validation/presentation-and-data/run-1.jsonl)
