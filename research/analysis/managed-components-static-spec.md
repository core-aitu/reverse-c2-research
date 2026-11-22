# Статическая спецификация собственных managed-компонентов

Статус корпуса: **static-complete** для перечисленных бинарных образов и реконструкций. Результат не подменяет runtime-проверку внешних toolchains, ОС, сети и реального UI.

| Область | Закрытый знаменатель | Спецификация |
|---|---:|---|
| SpecterInsight.Obfuscation | 837 TypeDef / 7 673 MethodDef; 148 transform-related типов; 5 IL samples | [obfuscation.md](obfuscation.md) |
| Modules | 22/22 DLL; 191 PSCmdlet; все P/Invoke и AssemblyRef | [modules.md](modules.md) |
| Managed Beacon variants | 4/4 no-PDB variants против `d5e7…569d`; полный normalized-IL diff | [beacon-variants.md](beacon-variants.md) |
| Client UI | 24/24 workflow; 152 Service/Utility/ViewModel files | [client-ui.md](client-ui.md) |

Машиночитаемые основания: [managed-metadata.json](managed-metadata.json) и [managed-semantics.json](managed-semantics.json). Воспроизводимые анализаторы находятся в `research/examination/managed-components/`.

Единственный реестр вопросов, которые не закрываются этой статической спецификацией: [gaps.md](../validation/gaps.md).
