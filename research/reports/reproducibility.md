# Воспроизводимость результатов

| Объект | Критерий эквивалентности | Что проверено |
|---|---|---|
| Файловый снимок | Совпадают записи для всех путей, включая SHA-256 файлов. | Повторный обход исходного дерева и рабочей копии воспроизвёл распакованные манифест и реестр дубликатов побайтно. Контрольные суммы результатов записаны в environment.json. |
| Структура управляемых сборок | Для одинакового SHA-256 совпадают MVID, количество TypeDef и MethodDef. | Суммы диапазонов индекса сверены для 32 различных образов. У Host отдельный разбор REA подтвердил MVID и токен входа. |
| Контракт чтения документации | Совпадают метод и путь запроса, дерево значений, результат поиска и тип исключения для заданных входов. | Два основных и один контрольный прогон в отдельных пространствах имён дали одинаковые журналы. Все восемь условий выполнены в каждом. |
| Архитектура работающей установки | Коррелируют процессы, события, состояние БД и действия пользователя. | По этому критерию целая установка осталась непроверенной. Для части связей доступны описания поставщика и статические свидетельства. |

[Среда, времена UTC и контрольные суммы](../validation/environment.json) · [Контрольный прогон](../validation/documentation/run-control.jsonl)

## Как повторить

Начать в корне исследуемой поставки. Использовать снимок с теми же файлами, рабочую копию вне дерева поставки и отдельный каталог для результатов. Скрипт инвентаризации исключает каталог верхнего уровня research. Пути ниже обозначают локальные каталоги исследователя. Подставить свои значения перед запуском.

```bash
source_dir="$PWD"
copy_dir=/absolute/path/to/distribution-copy
output_dir=/absolute/path/to/new-results
sdk_dir=/absolute/path/to/dotnet-sdk-10.0.404
python_bin=/absolute/path/to/pinned-environment/bin/python
mkdir -p "$output_dir"/{evidence,analysis,architecture,examination,validation,build}
"$python_bin" research/examination/inventory.py "$source_dir" "$copy_dir" "$output_dir/evidence"
export DOTNET_CLI_HOME="$output_dir/build/cli" DOTNET_CLI_TELEMETRY_OPTOUT=1 DOTNET_NOLOGO=1
"$sdk_dir/dotnet" build research/examination/metadata/Metadata.csproj --artifacts-path "$output_dir/build/metadata" -o "$output_dir/build/app"
"$sdk_dir/dotnet" "$output_dir/build/app/Metadata.dll" "$copy_dir" "$output_dir/evidence/manifest.jsonl.gz" "$output_dir/analysis"
"$python_bin" research/examination/observations.py "$copy_dir" "$output_dir"
"$python_bin" research/examination/server_metadata.py "$copy_dir" "$output_dir"
"$sdk_dir/dotnet" build research/examination/structure/Structure.csproj --artifacts-path "$output_dir/build/structure" -o "$output_dir/build/structure-app"
"$sdk_dir/dotnet" "$output_dir/build/structure-app/Structure.dll" "$copy_dir" "$output_dir"
"$python_bin" research/examination/catalog.py "$copy_dir" "$output_dir"
"$python_bin" research/examination/independent_il.py "$copy_dir" "$output_dir"
"$python_bin" -B research/examination/variant_comparison.py "$copy_dir" "$output_dir"
"$python_bin" research/examination/package_integrity.py "$copy_dir" "$output_dir"
research/validation/documentation/repeat.sh "$sdk_dir" "$copy_dir" "$output_dir/build/replay" > "$output_dir/validation/replay.jsonl"
cmp research/validation/documentation/run-1.jsonl "$output_dir/validation/replay.jsonl"
```

Версии зависимостей Python, SDK, среды исполнения и bubblewrap записаны в environment.json. Для сборки используется SDK .NET 10.0.404, для выполнения — Microsoft.NETCore.App 10.0.15. В Linux-стенде исходные DLL и SDK монтируются только для чтения. У прогона отдельное сетевое пространство, а HttpMessageHandler обрабатывает запросы в памяти. Стенд самостоятельно сериализует дерево и заполняет коллекцию ViewModel. Проверки маршрутизатора, реальной HTTP-сериализации ASP.NET и визуального рендеринга требуют иных наблюдений.

**Граница воспроизведения.** Модель документа, значения в ответе и ошибки для указанных входов воспроизводимы на хешированных сборках. Из этих результатов не следует эквивалентность сторонней реализации всей платформе. Файлы с одинаковой строкой версии, но другим SHA-256 нужно учитывать как отдельные образцы.


## Дополнительные опыты

Использована PostgreSQL 16.4 из исходного архива проекта. Проверенная SHA-256 архива — `971766d645aa73e93b9ef4e3be44201b4f45b5477095b049125403f9f3386d6f`. Локальная сборка для Linux выполнена с параметрами `--without-readline --without-zlib --without-icu --without-llvm` и `CFLAGS="-O1 -std=gnu17"`. Каталог установки передаётся параметром pg_dir. [Исходный архив PostgreSQL](https://ftp.postgresql.org/pub/source/v16.4/postgresql-16.4.tar.bz2) · [Контрольная сумма издателя](https://ftp.postgresql.org/pub/source/v16.4/postgresql-16.4.tar.bz2.sha256)

```bash
pg_dir=/absolute/path/to/postgresql-16.4
research/validation/postgres/repeat.sh "$sdk_dir" "$copy_dir" "$pg_dir" "$output_dir/build/store" > "$output_dir/validation/store.jsonl"
research/validation/presentation-and-data/repeat.sh "$sdk_dir" "$copy_dir" "$output_dir/build/ui-data" "$output_dir/validation/store.jsonl" > "$output_dir/validation/ui-data.jsonl"
cmp research/validation/postgres/run-1.jsonl "$output_dir/validation/store.jsonl"
cmp research/validation/presentation-and-data/run-1.jsonl "$output_dir/validation/ui-data.jsonl"
```

Каждый запуск создаёт новый кластер в приватном временном каталоге. Настоящие Server.dll и Npgsql взаимодействуют с ним по Unix-сокету. Доступ к порту TCP и рабочим конфигурациям продукта отсутствует. Для клиентского прогона зафиксирована передаваемая JSON-запись из серверного журнала. Для изначально двух запусков каждой серии и последующей сверки важна идентичность содержимого журналов. SHA-256 сохранены в environment.json. [Полная постановка опыта](extended-findings.md)

Для автономного поиска стенд использует библиотеки и модель из этой же рабочей копии. Вызов `research/validation/local-search/repeat.sh "$sdk_dir" "$copy_dir" "$output_dir/build/search"` выводит десять JSON-записей. Геобаза проверена командой `mmdblookup --verbose --file resources/GeoLite2-City.mmdb --ip 192.0.2.1`, версия утилиты 1.13.3. [Журнал поиска](../validation/local-search/run-1.jsonl) · [Вывод mmdblookup](../examination/geoip-metadata.txt)
