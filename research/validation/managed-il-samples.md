# Выборочная IL-проверка SpecterInsight.Obfuscation

Проверен файл `client/SpecterInsight.Obfuscation.dll` с SHA-256
`7c48d7c851c4ef4c531d616699366e307b155099a84f31bb8b9e8596db57fb4b` и MVID
`df19b452-8b62-4993-9661-352ef5913dbf`. Листинги получены `ilspycmd 11.2.0.10050`
непосредственно из PE, а tokens, число инструкций и нормализованные SHA-256 — независимым
`System.Reflection.Metadata`-декодером из `research/examination/managed-components/`.

| Token | Критическая граница | Инструкции | Нормализованный IL SHA-256 |
|---|---|---:|---|
| [`0x060009d9`](managed-il/0x060009d9.il) | сбор входов и вызов typed transform | 53 | `2526af44675afcdf68820fdf126ab1426f7a53aa574eaac4daf808570b601ced` |
| [`0x06000ab2`](managed-il/0x06000ab2.il) | C# → bflat/native compiler path | 83 | `d3ec7d258eb7b31bf84474735190b0b31d6fae0cbbc03cee932250a92d052d8a` |
| [`0x06000ad3`](managed-il/0x06000ad3.il) | Roslyn C# → managed PE | 146 | `b40c78db23b9da362e15640c60f99f7f0ad9a2d23c2e1559f6f8d4189c05d94f` |
| [`0x06000aee`](managed-il/0x06000aee.il) | Zig source/files → binary | 142 | `58993144de566c6ae9aa7d8e76d7c9ed1efec26b9cf2b7c40869486f097a3543` |
| [`0x06000c73`](managed-il/0x06000c73.il) | PowerShell AST transform | 130 | `a1064de5616c736dd88ef77a3c6cf5347b4e122e4a342eb129b554ef43526a8a` |

REA независимо подтвердил полный metadata view выбранных transform-типов (78 методов, 380 call
edges): `ev_e7bf87d4c690c5cedb2487ebae05f6fbd15114e8cbcc21804f1d369ac9f5481a`.
Проверка подтверждает статический control/data flow этих границ; внешние compiler-процессы не
запускались.
