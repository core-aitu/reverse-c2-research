# Повторение этапа

Использовать рабочую копию с хешами из исходного манифеста, SDK, PostgreSQL и Python из [описания среды](../environment.json). Проверка архивных материалов и матрицы не требует запуска приложения. Для графики использована Xvfb 21.1.25 из пакета Fedora 44. Локальная копия RPM с SHA256SUMS хранится в local-archive/continuation рядом с архивом прежнего этапа. Для извлечения RPM достаточно rpm2cpio и cpio, системная установка не требуется.

Из корня reverse-c2-research задать пути к локальным ресурсам. Переменная work обозначает отдельный временный каталог сборки, stage — каталог **копии** validation/continuation в отдельном дереве research, если требуются новые результаты без изменения исходных протоколов. Приведённый пример запускает код из текущего корпуса и складывает журналы в work.

```bash
stage=research/validation/continuation
python_bin=/absolute/path/to/venv/bin/python
sdk=/absolute/path/to/sdk
dist=/absolute/path/to/distribution-copy
pg=/absolute/path/to/pg16
xvfb=/absolute/path/to/extracted/usr/bin/Xvfb
work=$(mktemp -d)
"$python_bin" -B "$stage/verify.py" "$dist"
"$stage/diagnostics/repeat.sh" "$sdk" "$dist" "$work/logentry" > "$work/logentry.jsonl"
"$stage/desktop/repeat.sh" "$sdk" "$dist" "$xvfb" "$work/screen" > "$work/screen.jsonl"
"$stage/loopback/repeat.sh" "$sdk" "$dist" "$pg" "$work/network" "$work/screen/app" "$xvfb" > "$work/network.jsonl"
cmp "$work/logentry.jsonl" "$stage/diagnostics/logentry-1.jsonl"
cmp "$work/screen.jsonl" "$stage/desktop/desktop-1.jsonl"
cmp "$work/network.jsonl" "$stage/loopback/loop-1.jsonl"
```

Самостоятельный desktop-прогон использует сохранённое синтетическое событие как вход для повторяемой проверки экранов. Во вложенном графическом процессе loopback-прогона событие приходит через настоящий локальный SignalR. Для времени в тестовой записи выбрана фиксированная дата 2 января 2027 года. UI-фильтр отсчитывает окно от часов процесса. Для повторения исторической проверки после этой даты нужна соответствующая лабораторная дата (13 ноября 2026 года) либо новая фикстура с явно отмеченным изменением времени. Каждая итерация получает собственное пространство имён, временную БД и при необходимости отдельный X-сервер. Штатная конфигурация UI для инициализации компонентов применяется внутри временного каталога сборки.

Для повторной статической выборки запустить `"$python_bin" -B "$stage/static.py" "$dist" "$stage"` в отдельной копии research/. Каталог вывода должен оставаться на том же уровне относительно исходных реестров. Аргументы verify.py проверяют журналы на побайтную идентичность, поля DTO, контрольные суммы образцов, итоги по 32 управляемым сборкам и совпадение HTTP-дерева с исходной фикстурой.
