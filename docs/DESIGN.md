# Design & Architecture

This document covers the design philosophy, evolution, and cost optimization strategies behind the agent team pipeline.

## The Evolution: Before and Now

Most AI coding tools follow one of two patterns. This pipeline introduces a third.

### Stage 1: The All-in-One Agent

A single agent handles everything -- planning, coding, testing, reviewing, and committing -- in one long conversation.

```mermaid
flowchart TD
    A[User Request]
    B["SINGLE AGENT<br/>(Opus / Sonnet)<br/>Plans, explores, codes, tests,<br/>reviews, formats, commits...<br/>all in one context window"]
    C["Done (hopefully)"]

    A --> B --> C
```

**How it works:** The user describes a task. The agent thinks, writes code, runs tests, fixes failures, and commits -- all in a single thread. Every tool call (Claude Code, Cursor, default OpenCode, Copilot agent mode) typically works this way.

**The problems:**

- **Context bloat** -- the agent accumulates everything (exploration output, failed attempts, test logs) in one growing context, burning tokens on stale information
- **No separation of concerns** -- the same agent that writes code also reviews it (self-review catches less)
- **No cost control** -- expensive frontier models are used for mechanical tasks (formatting, git commits) that don't need them
- **Unpredictable workflow** -- the agent decides what to do next with no enforced structure

### Stage 2: The Two-Agent Split

A planner agent designs the approach, then a builder agent implements it.

```mermaid
flowchart TD
    A[User Request]
    B["PLANNER<br/>(Opus)<br/>Explores codebase, designs approach,<br/>creates implementation plan"]
    C[Plan passed down]
    D["BUILDER<br/>(Opus)<br/>Writes code, tests, formats, reviews,<br/>commits -- everything else"]
    E[Done]

    A --> B --> C --> D --> E
```

**How it works:** The first agent focuses purely on understanding the problem and designing a solution. Its plan is passed to a second agent that handles all implementation. This is the pattern used by some multi-file editing tools and "plan then execute" workflows.

**The improvements:**

- Planning and implementation are decoupled -- the builder gets a clear plan instead of figuring it out while coding
- Context is somewhat compressed between agents -- the builder doesn't see raw exploration output

**The remaining problems:**

- **The builder is still an all-in-one** -- it codes, tests, reviews, formats, and commits in a single agent
- **Still no independent review** -- the same agent that writes code judges its own quality
- **No model tiering** -- both agents use the same expensive model, even for mechanical tasks
- **No structured remediation** -- if tests fail, there's no enforced loop, just the builder trying again

### Stage 3: The 6-Agent Pipeline (This System)

Six specialized agents, each with a focused role, enforced ordering, model tiering, and structured remediation loops.

```mermaid
graph TB
 subgraph Execution_Pipeline[" "]
    direction LR
        UserApproval["<b>USER APPROVAL</b><br>approve/adjust"]
        Architect["<b>ARCHITECT</b><br>(your default)<br>plan"]
        Engineer["<b>ENGINEER</b><br>(Sonnet)<br>code"]
        Forge["<b>FORGE</b><br>(Sonnet)<br>test"]
        Inspector["<b>INSPECTOR</b><br>(Sonnet)<br>review"]
        Shipper["<b>SHIPPER</b><br>(Haiku)<br>commit"]
  end
    UserRequest(["User Request"]) --> Captain["<b>CAPTAIN</b><br>(your default)<br>Classifies, delegates, compresses<br>context between every step"]
    Captain --> Architect
    Architect --> UserApproval
    UserApproval --> Engineer
    Engineer --> Forge
    Forge --> Inspector
    Inspector --> Shipper
    Captain --> UserApproval
```

**What changed:**

- **Every concern is separated** -- planning, coding, testing, reviewing, and shipping each have a dedicated agent
- **Independent review** -- the Inspector never sees the Engineer's reasoning, only the code diff
- **Model tiering** -- validation, implementation, and shipping use cheaper models; only planning and orchestration run on your selected model
- **Structured remediation** -- test failures route back to the Engineer, not handled ad-hoc by whoever is running
- **Context compression** -- the Captain compresses output between every step, preventing context snowball
- **Budget guardrails** -- hard caps on invocations prevent runaway retry loops

### Side-by-Side Comparison

|                             | All-in-One       | Two-Agent        | 6-Agent Pipeline                      |
| --------------------------- | ---------------- | ---------------- | ------------------------------------- |
| **Agents**                  | 1                | 2                | 6                                     |
| **Separation of concerns**  | None             | Plan vs Build    | Full (plan, code, test, review, ship) |
| **User approval gate**      | No               | No               | Yes (after plan, before code)         |
| **Independent review**      | No (self-review) | No (self-review) | Yes (Inspector is separate)           |
| **Model tiering**           | No               | No               | Yes (3 tiers)                         |
| **Context management**      | Grows unbounded  | Split once       | Compressed at every step              |
| **Remediation loops**       | Ad-hoc           | Ad-hoc           | Structured (max 2 cycles)             |
| **Cost control**            | None             | None             | Budget guardrails per task            |
| **Workflow predictability** | Low              | Medium           | High (enforced pipeline)              |

## How We Reduced Token/Cost Usage

We applied **9 strategies** across the system, consolidated from an initial 12-agent design down to 6.

### 1. Agent Consolidation (12 -> 8 -> 6 agents)

Merged 6 agents into others: Architect stayed as Architect, Formatter -> Forge, Security -> Inspector, Docs -> Engineer, Explorer -> Architect, Tester -> Forge. Fewer agents = fewer invocations = fewer tokens.

### 2. Model Tiering

Not every task needs the most expensive model. The Engineer, Forge, and Inspector use Sonnet (mid-tier), Shipper uses Haiku 4.5 (light-tier). Only the Captain and Architect run on your selected model -- they omit a `model:` key entirely, so they inherit whatever you have configured in OpenCode (typically Opus). This alone cuts cost significantly -- validation, implementation, and coordination tasks don't all need frontier reasoning.

```text
Inherited (your default) -> Captain, Architect            [reasoning-heavy]
Mid       (Sonnet 5)     -> Engineer, Forge, Inspector    [structured tasks]
Light     (Haiku 4.5)    -> Shipper                       [mechanical tasks]
```

Leaving the two reasoning-heavy agents unpinned is deliberate: it keeps the premium tier a user choice rather than a hardcoded repo cost, and it means the pipeline adapts automatically when you switch your default model.

### 3. Context Compression

After each subagent returns, the Captain compresses output to **2-4 bullet points** before passing to the next agent. This prevents "context snowball" -- where each agent gets the full verbose output of every previous agent, ballooning token usage across the pipeline.

**Example:**

- Architect returns 200 lines of analysis -> compressed to: file paths, 3 implementation steps, 1 risk note
- Forge returns full test output -> compressed to: "14 tests passed, 0 failed"

### 4. Conditional Exploration Depth

The Architect does a lightweight 3-step exploration for simple tasks, but a full 7-step deep dive for standard/complex tasks. No point spending tokens exploring the entire codebase for a bug fix.

### 5. Stricter Output Caps

| Agent     | On Success                          | On Failure                  |
| --------- | ----------------------------------- | --------------------------- |
| Forge     | Test counts + status only           | First 10 lines of errors    |
| Inspector | Critical/high = full detail         | Medium = finding + fix only |
| Inspector | Low/suggestions = max 15 words each | Empty sections omitted      |

### 6. Trivial Self-Handle

For truly trivial edits (typo, config change, rename), the Captain makes the edit directly instead of spinning up the full pipeline -- but still always invokes the Forge for testing.

### 7. Conditional Session Persistence

The resume protocol (checkpoint files for interrupted work) only activates for standard/complex tasks. Trivial and simple tasks complete fast enough that persistence is overhead.

### 8. Pipeline Budget Guardrails

Each task classification has a hard cap on subagent invocations (including retries):

| Classification | Max Invocations | Typical Usage               |
| -------------- | --------------- | --------------------------- |
| Trivial        | 3               | forge + shipper + 1 retry   |
| Simple         | 6               | 4 steps + 2 retries         |
| Standard       | 8               | 5 steps + 3 retries         |
| Complex        | 12              | 5 steps + remediation loops |

When the budget is exhausted, the pipeline stops and asks the user -- preventing runaway retry loops that silently burn tokens.

### 9. Optional Memory-Assisted Exploration

When persistent memory tools are available (e.g. [Engram](https://github.com/Gentleman-Programming/engram) MCP plugin), the Captain queries prior session context before invoking the Architect. If relevant observations exist (file paths, conventions, architecture decisions from previous sessions), they're passed to the Architect as supplementary context.

The Architect uses this to **reduce exploration depth** — skipping re-reading files and conventions already known — while still verifying that referenced paths exist. This saves tokens on follow-up tasks in the same codebase without risking stale context.

This is fully optional. Without memory tools, the pipeline behaves exactly as before.

## Efficiency Awareness (Inspired by GreenAgent)

[GreenAgent](https://github.com/edgarasLegusVisma/greenagent) is a project that classifies LLM workflow steps as **useful work**, **overhead**, or **potential waste** -- tracking tokens, cost, energy, and carbon per step.

Direct integration wasn't possible (GreenAgent wraps direct API calls; OpenCode manages its own calls internally), but we adopted its **conceptual framework** into the pipeline:

### Step Categorization

Every pipeline step is labeled with a GreenAgent-style category:

| Category        | Steps              | Purpose                         |
| --------------- | ------------------ | ------------------------------- |
| `[overhead]`    | Plan+Explore, Git  | Necessary but not direct output |
| `[useful-work]` | Implement          | Produces the deliverable        |
| `[validation]`  | Build+Test, Review | Verifies the deliverable        |

### Efficiency Reporting

The pipeline's final report includes an efficiency line:

```text
Pipeline: 4/5 steps | 6 invocations (budget: 8) | useful-work: 2, validation: 2, overhead: 2
```

### Retrospective Classification

If a task was classified as "standard" but turned out to only touch 1-2 files with no design needed, the report notes it could have been simpler. This creates a feedback loop for improving future classification accuracy.

```text
Retrospective: task could have been classified as simple (single-file change, no design needed).
```

### How This Saves Waste

- **Budget guardrails** directly address the "potential waste" category by stopping runaway retry loops before they burn unnecessary tokens
- **Model tiering** ensures expensive models are only used where reasoning complexity demands it
- **Context compression** prevents the exponential context growth that is one of the largest hidden costs in multi-agent systems
- **Conditional depth** avoids spending exploration tokens on tasks that don't need it

## Other Notable Design Decisions

### Language/Framework Agnostic

All agent prompts use universal concepts (manifest files, build tools, test runners) rather than framework-specific language. Project-level skills and config handle technology specifics. The same agent team works across Laravel, React, Rust, Python -- any codebase.

### Independent Review

We deliberately kept the Inspector as a separate agent rather than merging it into the Engineer. Self-review is a known anti-pattern -- an independent reviewer catches what the author misses, even when the "author" is an AI.

### Remediation Loops with Delegation Discipline

When tests fail or the reviewer finds critical issues, fixes route through the Engineer (never the Captain). The Captain has three layers of reinforcement to prevent it from "helpfully" fixing code itself:

1. Opening instruction: "NEVER attempt to resolve it yourself"
2. Technical Failure Handling: "you are an orchestrator, not a worker"
3. Remediation Loop: "NEVER fix code yourself -- always delegate to @team-engineer"

### Hidden Subagents

All 5 subagents are hidden from the `@` autocomplete menu. They only appear when invoked by the Captain via the Task tool. This keeps the UI clean and prevents accidental direct invocation.

### Session Persistence

For longer tasks, the Captain maintains a `.opencode/resume.md` checkpoint file. If a session is interrupted, the user can say "resume" and pick up exactly where they left off -- no re-exploring the codebase or re-deriving the plan.

## Results

The combination of these strategies means:

- **Predictable costs** -- budget guardrails prevent surprise token usage
- **Higher code quality** -- independent review + mandatory testing catches issues early
- **Faster iteration** -- remediation loops fix test failures automatically instead of dumping errors on the user
- **Transferable workflow** -- language-agnostic design works across any project type
- **Visibility** -- efficiency reporting shows exactly where tokens are spent, enabling continuous optimization
