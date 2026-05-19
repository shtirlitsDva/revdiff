# Markdown Table Samples

Demo file for revdiff's table reformatting feature. Toggle with `T`.

## Tiny — single column

| Status |
|--------|
| OK     |
| FAIL   |
| PEND   |

## Tiny — two columns

| Key | Value |
|-----|-------|
| a   | 1     |
| b   | 2     |

## Small — typical config table

| Field | Type | Default | Notes |
|-------|------|---------|-------|
| `--theme` | string | `""` | overrides individual `--color-*` flags |
| `--no-colors` | bool | `false` | strips ANSI; mutually-exclusive with `--theme` |
| `--config` | string | `~/.config/revdiff/config` | overrides config file path |
| `--keys` | string | `~/.config/revdiff/keybindings` | overrides keybindings path |

## Inline markdown inside cells

| Element | Sample | Renders as |
|---------|--------|------------|
| **bold** | `**hello**` | **hello** |
| *italic* | `*hello*` | *italic hello* |
| `code` | `` `func()` `` | `func()` |
| link | `[click](https://example.com)` | [click](https://example.com) |
| **mixed** | `**bold with `code`**` | **bold with `code`** |
| escape | `a \| b` | a \| b |

## Alignment markers (GFM)

| Left | Center | Right | Default |
|:-----|:------:|------:|---------|
| a    | b      | c     | d       |
| 1234 | 5678   | 9012  | 3456    |
| longer-left | mid | longer-right | longer-default |
| x    | y      | z     | w       |

## Big — wide and tall

| Index | Name | Email | Department | Role | Manager | Hire Date | Office | Phone |
|------:|------|-------|------------|------|---------|-----------|--------|------:|
| 1 | Alice Anderson | alice.anderson@example.com | Engineering | Staff Engineer | Bob Bell | 2018-03-12 | NYC-3F | 555-0101 |
| 2 | Bob Bell | bob.bell@example.com | Engineering | Eng Manager | Carol Chen | 2015-07-04 | NYC-3F | 555-0102 |
| 3 | Carol Chen | carol.chen@example.com | Engineering | Director | Dan Davies | 2012-01-22 | NYC-4F | 555-0103 |
| 4 | Dan Davies | dan.davies@example.com | Product | VP Product | — | 2010-09-01 | NYC-5F | 555-0104 |
| 5 | Eva Estrada | eva.estrada@example.com | Design | Senior Designer | Faye Foster | 2019-11-18 | LA-2F | 555-0205 |
| 6 | Faye Foster | faye.foster@example.com | Design | Design Lead | Gina Goh | 2016-05-30 | LA-2F | 555-0206 |
| 7 | Gina Goh | gina.goh@example.com | Design | Director | Dan Davies | 2014-08-15 | LA-3F | 555-0207 |
| 8 | Henry Holt | henry.holt@example.com | Sales | AE | Iris Ivers | 2020-02-10 | CHI-1F | 555-0308 |
| 9 | Iris Ivers | iris.ivers@example.com | Sales | Sales Manager | Jack Jacobs | 2017-12-05 | CHI-1F | 555-0309 |
| 10 | Jack Jacobs | jack.jacobs@example.com | Sales | VP Sales | — | 2013-04-19 | CHI-2F | 555-0310 |

## Ragged — mismatched column counts

| A | B |
|---|---|
| 1 | 2 |
| 3 | 4 | 5 |
| 6 |
| 7 | 8 | 9 | 10 |

## Pipes inside backticks (must not split)

| Code | Note |
|------|------|
| `a | b` | regex alternation |
| `if x \|\| y` | shell OR |
| `grep \| awk` | classic pipe-chain example |

## Inside fenced code (must NOT format)

```markdown
| this | should |
|------|--------|
| stay | raw    |
```

After the fence, this one **should** format:

| renders | normally |
|---------|----------|
| yes     | indeed   |

## Without a separator row (not a real table — leave raw)

| nope | this |
| isn't | a |
| valid | table |

## After indented code (must NOT format the indented part)

    | indented | code |
    |----------|------|
    | leave    | raw  |

End of samples.
