# Benchmark results

Final data: 135 runs (15 tasks × 3 setups × 3 reps). Date: 2026-10-13. Prices: GitHub Copilot per-token prices from [Models and pricing](https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing), checked 2026-10-13 (`benchmark/prices.json`). To reproduce, see [benchmark/README.md](../benchmark/README.md).

## Method

- **Fixture:** a small notes app: Go `net/http` API (in-memory store) plus a Vite + React + TypeScript UI (vitest).
- **Tasks:** 15 across 5 categories (3 each): trivial, simple, standard, complex, hard.
- **Judging:** hidden tests that check only the public HTTP/DOM contract in the prompt. They are copied in after the agent finishes. Every task has a reference patch; the untouched fixture fails and the reference passes.
- **Setups:**
  - **A**: old 6-agent pipeline (captain, architect, engineer, forge, inspector, shipper).
  - **B**: plain single Opus agent (`bench-solo`).
  - **C**: `team-captain` (new, single agent; called `team-solo` during benchmarking), the current default.
- **Reps:** 3 per task and setup, with the mode order rotated per task and rep.
- **Isolation:** fresh temp copy of the fixture per run, empty `XDG_CONFIG_HOME` (no global MCP servers, plugins or agents), agents installed project-locally, external directory access denied, post-run leak check.
- **Cost:** summed over the whole session tree and all models; USD is estimated from token counts × price. Copilot reports cost 0, and the premium-request columns are all 0 (multipliers unset), so USD is the only meaningful cost.

## Cost per run (USD)

| setup | category | runs | pass rate    |   mean |    min |    max |
| ----- | -------- | ---: | ------------ | -----: | -----: | -----: |
| A     | trivial  |    9 | 9/9 (100%)   | 0.2337 | 0.2103 | 0.2811 |
| A     | simple   |    9 | 9/9 (100%)   | 0.4461 | 0.3511 |  0.562 |
| A     | standard |    9 | 9/9 (100%)   | 0.6828 | 0.6543 | 0.7224 |
| A     | complex  |    9 | 9/9 (100%)   |  0.707 | 0.5918 | 0.8178 |
| A     | hard     |    9 | 9/9 (100%)   | 0.6677 | 0.5332 | 0.8127 |
| A     | ALL      |   45 | 45/45 (100%) | 0.5475 | 0.2103 | 0.8178 |
| B     | trivial  |    9 | 9/9 (100%)   | 0.1453 | 0.1127 | 0.1863 |
| B     | simple   |    9 | 9/9 (100%)   | 0.2255 | 0.1794 | 0.2518 |
| B     | standard |    9 | 9/9 (100%)   |  0.393 |  0.303 |  0.502 |
| B     | complex  |    9 | 9/9 (100%)   | 0.4236 | 0.2521 | 0.5344 |
| B     | hard     |    9 | 9/9 (100%)   | 0.3675 | 0.2853 | 0.4341 |
| B     | ALL      |   45 | 45/45 (100%) |  0.311 | 0.1127 | 0.5344 |
| C     | trivial  |    9 | 9/9 (100%)   | 0.1481 | 0.1199 | 0.1785 |
| C     | simple   |    9 | 9/9 (100%)   | 0.1979 | 0.1259 | 0.2464 |
| C     | standard |    9 | 9/9 (100%)   | 0.3693 | 0.2051 | 0.5011 |
| C     | complex  |    9 | 9/9 (100%)   | 0.3561 | 0.2716 | 0.4502 |
| C     | hard     |    9 | 9/9 (100%)   | 0.3347 | 0.2274 | 0.3984 |
| C     | ALL      |   45 | 45/45 (100%) | 0.2812 | 0.1199 | 0.5011 |

Since every run passed, cost per successful task equals the mean.

## Mean cost ratios (USD)

| category | A / B | A / C | B / C |
| -------- | ----: | ----: | ----: |
| trivial  | 1.61x | 1.58x | 0.98x |
| simple   | 1.98x | 2.25x | 1.14x |
| standard | 1.74x | 1.85x | 1.06x |
| complex  | 1.67x | 1.99x | 1.19x |
| hard     | 1.82x | 2.00x | 1.10x |
| ALL      | 1.76x | 1.95x | 1.11x |

## Per-model breakdown (average per run)

| setup | model             | requests | input | output | reasoning | cache_read | cache_write |      USD |
| ----- | ----------------- | -------: | ----: | -----: | --------: | ---------: | ----------: | -------: |
| A     | claude-haiku-5.5  |      4.2 |    10 |    899 |       130 |      51369 |       17888 | 0.003265 |
| A     | claude-opus-5.5   |      8.1 |    20 |   4795 |       235 |     139662 |       44851 |   0.3529 |
| A     | claude-sonnet-5.5 |      9.1 |    23 |   4686 |       158 |     125589 |       52130 |   0.1914 |
| B     | claude-opus-5.5   |      8.1 |    18 |   5550 |        96 |     180909 |       32357 |    0.311 |
| C     | claude-opus-5.5   |      7.6 |    17 |   4828 |       139 |     165872 |       29731 |   0.2812 |

## Finding: cache writes

Each subagent starts a new session, so its system prompt, tool definitions and handed-over context are written to the cache again. The old pipeline averaged about 115K cache-write tokens per run (17,888 + 44,851 + 52,130 across its three models), against 32,357 for plain Opus and 29,731 for `team-captain` (C). Total cache reads were also higher for the old pipeline (about 317K = 51,369 + 139,662 + 125,589, vs 180,909 for plain Opus and 165,872 for `team-captain` (C)), but they are cheap, so cache writes dominate the cost gap. A's extra cost over plain Opus ($0.5475 - $0.3110 = $0.2365 per run), decomposed by token type from the per-model averages in `report-full.md` and the prices in `benchmark/prices.json` (summed over all models):

| token type  | A cost ($) | B cost ($) |  A - B ($) | share of gap |
| ----------- | ---------: | ---------: | ---------: | -----------: |
| cache_write |     0.3568 |     0.1618 |     0.1950 |        82.4% |
| output      |     0.1432 |     0.1110 |     0.0322 |        13.6% |
| cache_read  |     0.0410 |     0.0362 |     0.0048 |         2.0% |
| reasoning   |     0.0063 |     0.0019 |     0.0044 |         1.9% |
| input       |     0.0001 |     0.0001 |     0.0001 |         0.0% |
| **total**   | **0.5475** | **0.3110** | **0.2365** |     **100%** |

Cache writes explain about 82% of the extra cost; cache reads only about 2%.

## Benchmark v2 (real repos)

117 runs on two internal Go+React projects, $14.97 total. Delegation was used in 0/31 runs when optional. Haiku passed 10/10 at about $0.011/run, Sonnet about $0.11, Opus about $0.30. The v6 verify rule fixed the missed `gofmt` in 8/8 runs. Caveats: 1-2 reps per cell, planted bugs (not organic work), peak context 19-44K tokens (too small for delegation to pay off).

## Caveats

- **Prompt suffix:** A and B on tasks 01-12 ran with the older suffix "Do not ask questions; proceed. Commit locally when done; no git remote exists." C, and A/B on 13-15, got the newer "Do not ask questions or wait for approval; proceed. ...".
- **Estimated cost:** Copilot reports cost 0; USD is computed from token counts and published per-token prices (reasoning priced at the output rate).
- **B vs C is within noise per category:** the overall 1.11x (about 10% cheaper) is consistent in direction, but individual category differences (0.98x to 1.19x) overlap in min/max.
- **Tasks are too easy to separate quality:** all 135 runs passed, so the benchmark shows cost at equal pass rate, not quality differences.
- **Haiku tier simplified:** Haiku 5.5 is priced at its default (<=100K input) tier only.
- **Non-determinism:** 3 reps per cell is small; read min/max, not just the mean.
