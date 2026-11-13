# Исследование SpecterInsight 7.0.0

[Итоговый технический отчёт](reports/final.md) · [Расширенная проверка](reports/extended-findings.md) · [Воспроизводимость](reports/reproducibility.md) · [Открытые вопросы](validation/gaps.md)

| Раздел | С чего начать |
|---|---|
| Исходный материал | [Паспорт снимка](evidence/snapshot.json), [манифест SHA-256](evidence/manifest.jsonl.gz), [наблюдения и источники](evidence/observations.jsonl) |
| Инструменты и измерения | [Инвентаризация](examination/inventory.py), [метаданные сборок](analysis/assemblies.jsonl), [проверенные тела IL](analysis/method-evidence.json), [охват по образам](validation/coverage.json) |
| Архитектура | [Представления](architecture/views.md), [графы пакетов](architecture/package-graphs.json), [путь документа](interactions/documentation.md), [модель хранения](analysis/storage.md) |
| Компоненты | [Server и Host](components/server/reference.md), [Client](components/client/reference.md), [Specter](components/specter/reference.md), [вспомогательные компоненты](components/auxiliary/reference.md) |
| Протоколы | [Management API](protocols/management.md), [свидетельства об обмене Specter](protocols/specter-observations.md) |
| Проверка и публикация | [Реестр выводов](validation/findings.jsonl), [среда и суммы](validation/environment.json), [шаблоны записей](templates/records.md) |

**Как проверить вывод.** Найти FND в реестре выводов, затем перечисленные OBS в реестре наблюдений. Запись OBS задаёт исходный файл ART, выход инструмента и место внутри него. Внутри одного снимка ART определяется путём. Идентичность содержимого между снимками проверяется по SHA-256. Для метода дополнительно сверяются MVID, токен и смещение IL. У результатов динамики ключами служат имя сценария и, для поиска в UI, путь документа.

**Статусы.** Verified означает, что формулировку подтверждает указанное свидетельство в заданных границах. Inferred обозначает интерпретацию, Unverified — открытый вопрос, Contradicted — гипотезу, которой противоречит наблюдение. Вид свидетельства фиксируется отдельно. Verified для статического объявления не переносится на выполнение в реальной установке.

**Методика.** Представления фиксируют вопросы исследователей, границы и основания связей с учётом ISO/IEC/IEEE 42010:2022. Сбор, извлечение, анализ и отчёт разделены по рекомендациям NIST SP 800-86. Для сохранения происхождения и повторяемости использованы рекомендации SWGDE. Структура research/, идентификаторы и формат JSONL — соглашения этого исследования. Для текущего корпуса не создавался SARIF-экспорт. [ISO 42010](https://www.iso.org/standard/74393.html) · [NIST SP 800-86](https://csrc.nist.gov/pubs/sp/800/86/final) · [SWGDE Collection](https://www.swgde.org/documents/published-complete-listing/18-f-002-2-0/) · [SWGDE Examinations](https://www.swgde.org/documents/published-complete-listing/18-f-001-2/) · [OASIS SARIF 2.1.0](https://docs.oasis-open.org/sarif/sarif/v2.1.0/os/sarif-v2.1.0-os.html)
