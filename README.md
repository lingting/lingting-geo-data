# lingting-geo-data

[![同步上游数据](https://github.com/lingting/lingting-geo-data/actions/workflows/sync.yml/badge.svg)](https://github.com/lingting/lingting-geo-data/actions/workflows/sync.yml)

`lingting-geo-data` 是一份可直接使用的全球地理数据集。项目从 Unicode CLDR、Google libphonenumber 和 flag-icons 同步数据，并生成统一的国家/地区、联合国 M.49 地理层级、国际电话前缀及旗帜 SVG 文件。

生成结果提交在仓库中，消费方无需运行 Go 程序或访问上游服务；可直接读取 [`generated/`](generated/) 下的 JSON 与 SVG 文件。

## 内容与目录

```text
.
├── generated/                 # 可供消费方直接使用的生成结果
│   ├── regions.json           # 国家/地区综合信息
│   ├── m49.json               # UN M.49 世界—区域—子区域层级
│   ├── phones.json            # 电话前缀到国际区号、地区的映射
│   ├── flags/                 # regions.json 引用的 4:3 SVG 旗帜
│   └── sources.json           # 本次生成使用的上游来源与校验信息
├── sources/                   # 从上游同步的原始数据快照
├── cmd/sync/                  # 数据同步命令入口
└── internal/                  # 同步、解析、生成与校验逻辑
```

## 生成文件格式

### `generated/regions.json`

国家/地区对象数组，按 ISO 3166-1 alpha-2 代码升序排列。每个对象的结构如下：

```json
{
  "iso": "CN",
  "iso3": "CHN",
  "flag": "cn",
  "callingCodes": ["86"],
  "phonePrefixes": ["86"],
  "names": {
    "en": "China",
    "zh": "中国"
  },
  "numeric": "156",
  "m49": {
    "region": "142",
    "subregion": "030"
  }
}
```

| 字段 | 含义 |
| --- | --- |
| `iso` | ISO 3166-1 alpha-2 两字母地区代码。 |
| `iso3` | ISO 3166-1 alpha-3 三字母地区代码。 |
| `flag` | 旗帜文件名（不含 `.svg`）；对应 `generated/flags/<flag>.svg`。香港（`HK`）、澳门（`MO`）和台湾（`TW`）复用 `cn.svg`。 |
| `callingCodes` | 该地区的国际电话区号列表，不含 `+` 号。多个地区可能共享同一国际区号。 |
| `phonePrefixes` | 用于识别地区的电话号码数字前缀列表，不含 `+` 号；缺少更具体前缀时使用国际电话区号。 |
| `names.en` | CLDR 提供的英文地区名称。 |
| `names.zh` | CLDR 提供的中文地区名称。 |
| `numeric` | ISO 3166-1 数字地区代码，保留前导零。 |
| `m49.region` | 所属 UN M.49 大区数字代码；未归入层级时省略。 |
| `m49.subregion` | 所属 UN M.49 子区域数字代码；未归入层级时省略。 |

### `generated/m49.json`

UN M.49 地理层级树，根节点为世界（`001`）。每个节点结构如下：

```json
{
  "code": "142",
  "name": { "en": "Asia", "zh": "亚洲" },
  "children": [],
  "regions": ["CN", "JP"]
}
```

| 字段 | 含义 |
| --- | --- |
| `code` | UN M.49 数字代码。 |
| `name.en` / `name.zh` | CLDR 提供的节点英文/中文名称。 |
| `children` | 下级 M.49 节点；无下级时省略。 |
| `regions` | 直属 ISO 3166-1 alpha-2 地区代码；无直属地区时省略。 |

### `generated/phones.json`

电话前缀对象数组，按 `prefix`、`calling` 倒序及 `region` 升序排列，适合按最长前缀匹配电话号码：

```json
{
  "prefix": 86,
  "calling": 86,
  "region": "CN"
}
```

| 字段 | 含义 |
| --- | --- |
| `prefix` | 用于地区识别的数字前缀，不含 `+` 号；可能为国际区号加地区前导数字。 |
| `calling` | 国际电话区号，不含 `+` 号。 |
| `region` | 对应的 ISO 3166-1 alpha-2 地区代码。 |

### `generated/flags/`

包含 `regions.json` 中 `flag` 字段引用的旗帜 SVG，采用 4:3 比例。文件名为小写 ISO 代码（或复用的 `cn`），例如 `flags/cn.svg`。

### `generated/sources.json`

记录每个同步来源的地址、HTTP ETag、内容 SHA-256 哈希和来源说明，用于追踪数据版本与判断是否需要重新生成：

```json
{
  "sources": {
    "cldr/codeMappings.json": {
      "url": "https://...",
      "etag": "...",
      "sha256": "...",
      "provenance": "Unicode CLDR (Unicode License v3)"
    }
  }
}
```

## 数据来源与许可证

本项目代码以 [MIT License](LICENSE) 发布；`sources/` 与 `generated/` 中的数据和旗帜还受相应上游许可证约束。使用或再分发时，请同时遵守下列许可证：

| 来源 | 用途 | 上游许可证 |
| --- | --- | --- |
| [Unicode CLDR](https://cldr.unicode.org/) | ISO 代码映射、英文/中文地区名称、UN M.49 地理层级。 | [Unicode License v3](https://www.unicode.org/license.txt) |
| [Google libphonenumber](https://github.com/google/libphonenumber) | 国际电话区号与地区电话前缀。 | [Apache License 2.0](https://github.com/google/libphonenumber/blob/master/LICENSE) |
| [flag-icons](https://github.com/lipis/flag-icons) | 4:3 SVG 旗帜。 | [MIT License](https://github.com/lipis/flag-icons/blob/main/LICENSE) |

各次同步所使用的具体上游 URL、ETag 和 SHA-256 哈希见 [`generated/sources.json`](generated/sources.json)。

## 更新数据

项目通过 GitHub Actions 每周一 UTC 03:00 自动同步上游数据；也可在 Actions 页面手动触发 `Sync upstream data` 工作流。

如需在本地生成数据，需要 Go 1.24 或更高版本，并在仓库根目录执行：

```sh
go run ./cmd/sync
```

同步程序会使用 HTTP ETag 检查上游更新，仅在来源或生成结果变化时写入 `sources/` 和 `generated/`。运行成功时输出 `changes detected` 或 `no changes`。

## 注意事项

- 本项目提供的是上游公开资料的统一快照，不保证数据在任意时点都完全准确、完整或适用于特定业务场景。
- 电话前缀仅用于地区推断；共享国家代码、号码可携带和号码规划变更可能导致推断结果不唯一或不准确。
- `m49.json` 中的区域归属与名称遵循 CLDR/UN M.49 数据，不构成对政治地位、主权或边界的任何立场。
