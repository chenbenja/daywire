# Daywire — concept report (v5)

**Status:** research / pre-POC  
**Date:** 5 Nov 2026  
**Chosen name:** Daywire (earlier working titles: *life-context bridge*; *Lifebus* was a candidate)

> **Pitch:** Your AI finally knows what happened today — and the raw data never left your phone.

Daywire is an open-source, **phone-first** life-context bridge: a native Android app (optional home hub) that turns notifications, calls, photos, location and calendar into *redacted* semantic events, then feeds them to whichever assistant you already use over MCP, with proactive nudges back through Telegram or the lock screen.

**Verdict: conditional go.** Ship the open-source Android app; keep a Go hub optional for heavy vision and long speech-to-text. Launch on GitHub Releases and F-Droid first; treat the Play Store as a later fight. Do not fold this into a prior private video-search codebase.

**Scores (judgement, not measurements):**

| Lens | Score /10 | Why |
|---|---:|---|
| Open-source project | **6.5** | Phone-first is the real installable product; OpenClaw-lane GTM fits; Play Store and 2027 sideloading verification are the drag |
| Company later via open core | **4.5** | A paid relay plus a trusted official APK is a clearer Nabu Casa-style path once the app exists |

---

## Table of contents

1. [Phone-first architecture](#1-phone-first-architecture)
2. [GTM — OpenClaw lane](#2-gtm--openclaw-lane)
3. [Market validation](#3-market-validation)
4. [Platform feasibility](#4-platform-feasibility)
5. [Name research](#5-name-research)
6. [Reuse from a prior private video-search project](#6-reuse-from-a-prior-private-video-search-project)
7. [4-week POC](#7-4-week-poc)
8. [Safety by design](#8-safety-by-design)
9. [Scores and kill gates](#9-scores-and-kill-gates)
10. [Appendix A — version history (v2–v5)](#appendix-a--version-history-v2v5)
11. [Appendix B — condensed earlier research](#appendix-b--condensed-earlier-research)
12. [Appendix C — sources and labels](#appendix-c--sources-and-labels)

---

## 1. Phone-first architecture

```
Android app ──► redacted events ──► MCP + Telegram
   │                    │
   │ intake, redact,    └──► optional Go hub (vision, long STT, timeline)
   │ STT, extract
   ▼
 only redacted events leave the phone
```

| Layer | Responsibility |
|---|---|
| **Android app** | Intake (NotificationListenerService, call-recording folder via Storage Access Framework, photos, location, calendar), redaction, on-device STT and task extraction |
| **Redacted events** | Typed semantic events (`commitment`, `visit`, `ask`, …). Raw audio, message bodies and images stay on device unless the user opts into hub offload |
| **Optional Go hub** | Heavy vision, long-call speech-to-text, cross-device timeline |
| **MCP + Telegram** | Any assistant (Claude, Grok, ChatGPT, Gemini, local) pulls context; proactive nudges go out redacted |

**On-device models (sourced benchmarks; see Appendix C):**

- Speech-to-text: sherpa-onnx or Whisper-class
- Task extraction: Gemma 3n or small Qwen (LiteRT / llama.cpp)
- Embeddings: EmbeddingGemma (under 200 MB RAM per Google docs)

**Borrowed apps** (Home Assistant Companion, Tasker, OwnTracks, iOS Shortcuts) are **validation-only shortcuts**, not the product.

---

## 2. GTM — OpenClaw lane

### How comparable projects grew

| Project | How it grew (sourced) | Lesson |
|---|---|---|
| OpenClaw | `curl \| bash` installer ([docs.openclaw.ai/install](https://docs.openclaw.ai/install)); chat apps people already use; BYO model; “100k stars and 2M visitors in a week” (Steinberger via Bitsight); **393,916** stars on 5 Nov 2026 (GitHub API) | One command + a chat surface + a screenshotable demo |
| Ollama | Show HN Jul 2023 (284 points); 28.9k→105.1k stars in 2024 (Runa Capital); ~185.8k (GitTrend); $88M raised, 8.9M monthly users (TechCrunch Jul 2026) | One CLI that removes setup pain |
| Immich | Prototype on r/selfhosted; MIT→AGPL; 30k+ stars added in 2024 (Immich 2024 recap); ~118.1k (GitTrend); joined FUTO 1 May 2024 | Privacy self-hosting community + AGPL + brand |
| Home Assistant | Adapter ecosystem + Open Home Foundation + Nabu Casa paid cloud (OHF structure page) | Integrations and a paid convenience layer |
| Omi | Show HN 117 points (2024-08); 13,892 stars; $2M (Jan 2025) | Hardware + open source |
| OmniBot | 2,086 stars since Mar 2026 (GitHub API) | On-device agent apps grow slower than chat-surface tools |

### Launch sequence

1. **Pre-launch** — Private APK to ~10 power users (r/LocalLLaMA, r/selfhosted, GrapheneOS). Measure battery, useful-nudge rate, setup time.
2. **Day 0** — Public GitHub (AGPL app + hub, licence TBD until decided), 60 s demo video (lock-screen nudge from a real call), Show HN + X thread. Hook line above.
3. **Week 1–2** — r/selfhosted, r/LocalLLaMA, GrapheneOS forums, Home Assistant Discord (optional HA bridge). Adapter bounty list.
4. **Distribution** — GitHub Releases + Obtainium + F-Droid first. Android developer verification: enforced **30 Sep 2026** in Brazil, Indonesia, Singapore, Thailand for participating stores; **global in 2027** ([Android Developers](https://developer.android.com/developer-verification)). Unverified apps need ADB or the advanced flow (Developer options, restart, 24 h wait). Register as a verified developer before 2027 for ordinary installs.
5. **Play Store** — Treat as hostile. `NotificationListenerService` needs a declared core purpose (Play sensitive-permissions policy). Accessibility **cannot** be used for call recording ([Play Accessibility policy](https://support.google.com/googleplay/android-developer/answer/16558241)). Call-recording folder access via SAF is fine. Expect review friction; F-Droid is the honest channel.
6. **Upside later** — Paid Tailscale-style relay / family sync (Nabu Casa model); acqui-hire into a personal-agent lab (OpenClaw / ggml pattern); or a foundation if it outgrows a solo maintainer.

**Honest odds:** this raises adoption odds versus an app-only or hub-only plan. An OpenClaw-*scale* breakout is still unlikely; proactive value needs days of context, while OpenClaw delighted in minute one. The first useful nudge is the thing to prove.

---

## 3. Market validation

**Demand signals** (real posts, not invented quotes):

- r/ClaudeCode — builders wiring hourly inbox/calendar ingest so Claude knows their life
- r/ClaudeAI — Shortcuts injecting location/calendar into every call
- r/LocalLLaMA — projects like VISOR wiring calendar and phone control into llama.cpp
- r/Lifelogging — on-device iPhone apps pitching local memory
- HN: Personal Context MCP (**28** points, Sep 2026); Open Memory Protocol (**33**, Jun 2026)

These are builders hacking around the gap, not surveys.

### Competitors

| Competitor | Status | Why Daywire is different |
|---|---|---|
| OmniBot (2,086 stars) | Full Android agent; on-demand tools | Infrastructure for *any* assistant; semantic events + timed nudge path |
| Rouse Context (13 stars) | Android MCP server via relay | Pull-only; Daywire adds extraction and downlink |
| Dytto | iOS journal + cloud API | Cloud; Daywire stays local |
| Google Personal Intelligence | Gemini-only, US beta since 20 Jan 2026, not EEA/UK | Vendor lock-in |
| ChatGPT Dots / Pulse | Pulse sunset 2026; Dots via RCS/iMessage (Pro US preview) | Cloud connectors, not phone sensors |
| Grok Bot | Evident: calendar, contacts, alarms, location | On-demand inside one vendor |
| Omi / Limitless / Bee | Hardware; Limitless→Meta Dec 2025; Bee→Amazon Jul 2025 | Daywire builds no hardware |

---

## 4. Platform feasibility

| Surface | What works | What does not | Notes |
|---|---|---|---|
| Android (Samsung) | NotificationListenerService; `Recordings/Call` via SAF; Health Connect; location; calendar | Accessibility-based call recording banned on Play | Galaxy AI Transcript Assist needs Samsung account; language list lacks Hebrew (Samsung support) |
| Android (Pixel) | Same APIs; Call Notes = on-device Gemini Nano (Pixel 9+, not Israel, English summaries) | Call recordings may stay dialer-private → share-sheet fallback | |
| Android (Xiaomi / others) | Same APIs if the dialer writes a shared folder | Aggressive battery killers; OEM paths vary | **Unverified** per-OEM paths |
| Background limits | `dataSync` FGS: 6 h / 24 h (Android 15 docs); WorkManager under Doze | Continuous LLM inference | Burst while charging |
| On-device models | Gemma 3n E2B: 15.6–28.4 tok/s decode (LiteRT HF card); Gemma 4 E2B: 46.9–52.1 tok/s on S26 Ultra; EmbeddingGemma under 200 MB | Vision captions: minutes/photo on Pixel 8 Pro CPU (single-user report) | Vision + long STT → optional hub |
| iOS | Location, EventKit, HealthKit, PhotoKit, Shortcuts POST, Share sheet, Live Activities, Foundation Models (background rate-limited) | **No** other apps’ notifications; **no** call-audio API; recordings stay in Notes | Thin companion, not the flagship |
| Desktop | macOS/Windows/Linux screen OCR (Screenpipe pattern); calendar/IMAP | | Good Week-1 source while Android matures |
| Meta glasses | Device Access Toolkit 1.0 (from 30 Sep 2026): camera, 8 kHz mic, speakers, IMU; session while app open | Always-on capture | Later adapter |

---

## 5. Name research

**Chosen: Daywire.** Checks below were run 5 Nov 2026. Nothing was registered by the research process.

| Name | Rationale | GitHub | Domain dig/RDAP | Trademark notes |
|---|---|---|---|---|
| **Daywire** ✓ | Day’s events on a wire to your AI | 2 empty repos | daywire.app / .dev: no NS (likely free at check) | No USPTO hit found this round |
| Lifebus | “Home Assistant for life context” | 1 repo @ 7 stars (noties/Lifebus) | lifebus.dev / .ai: no NS | USPTO **#8211622** Class 10 medical (Apr 2026). Different class; branding risk |
| Kithline | Kith = people you know | 0 repos | kithline.dev / .ai: no NS | No USPTO hit found |
| Daybridge | Bridge from day to assistant | 18 low-star repos | daybridge.dev: no NS | TM unverified |
| Contextbus | Literal | 1 zero-star repo | contextbus.ai **registered** (Mar 2026) | CONTEXT AI marks (Use Context Inc.; Fulcra Dynamics) |
| Throughline | Thread of your day | 360 repos | throughline.ai **registered** (2023) | USPTO **#99665384** pending, therapy AI (Tela Labs) |
| Contextd | Daemon of context | 355 repos | Not checked | Crowded |
| Eventide | Evening of the day’s events | 449 repos | eventide.dev **registered** | Avoid |
| Lifeport | Port for life signals | 32 repos | lifeport.dev: no NS | TM unverified |
| Lifecontext | Descriptive | 12 repos incl. LifeContextOS (42★) | Not checked | Weak mark |
| Redacted | Privacy joke | 1,363 repos | redacted.dev **registered** | Avoid |
| Kith | Short | Taken (KitHack 2.1k) | Not checked | Avoid |

Play Store collisions: **unverified** (store-search HTML returned no usable titles).

---

## 6. Reuse from a prior private video-search project

A prior private video-search / CCTV project (Python FastAPI, OpenVINO CLIP, optional EmbeddingGemma 2, Whisper/OCR fusion) was inspected for reusable patterns. **Recommendation: dedicated Daywire repo; keep that project separate.**

| Asset | Reuse? | How |
|---|---|---|
| EG2 embedding + RRF text-fusion *ideas* | Yes, as patterns | Reimplement in Go or Kotlin against the same model cards; do not import that Python package |
| Keyframe / change-gate / OpenVINO CLIP pipeline | No | CCTV-specific |
| Alert watches, FA/h calibration, UCF eval | No | Wrong metric for life context |
| SQLite + float16 vector index pattern | Yes, as a design | Small event store on phone and hub |
| Docker / model-export packaging | Partial | Hub packaging only |
| Planned Go control plane | Yes | Daywire is the better first Go product |

Coupling a life-log to a CCTV codebase would confuse licence, contributors and story.

---

## 7. 4-week POC

Assume a solo founder new to Kotlin. Minimize Android surface area: put schema, store, MCP, redaction and Telegram in **Go**; expose a thin **Kotlin** shell (`NotificationListenerService`, SAF folder picker, WorkManager) over a local socket or **gomobile**. Flutter is a later option for one UI codebase.

| Week | Deliverable |
|---|---|
| 1 | Go core: event schema, encrypted SQLite, MCP (`query_events`, `subscribe`, `propose_nudge`), Telegram bot, localhost-only doctor. Demo with calendar + IMAP + desktop OCR |
| 2 | Thin Kotlin shell: notification allow-list, location visits, SAF for `Recordings/Call`, redacted text events into the Go core |
| 3 | On-device extraction: sherpa-onnx STT; Gemma 3n / Qwen-small via LiteRT or llama.cpp; EmbeddingGemma. **Measure battery** on a real phone |
| 4 | Magic loop + packaging: hang up → redacted event → Telegram nudge + Claude MCP answer. GitHub Release APK + Obtainium docs. 60 s demo film |

### POC metrics (proposed targets, not measurements)

| Metric | Target |
|---|---|
| Time to first useful nudge | Under 15 min for 8/10 testers |
| Battery over 24 h with default sensors | Under 5% (**must measure**) |
| Raw audio/images leaving the phone | 0 without explicit opt-in |
| Two assistants answering one cross-source question | Pass |

---

## 8. Safety by design

Lessons from OpenClaw failures (30k+ exposed instances per Bitsight; 341 malicious ClawHub skills per Koi / The Hacker News):

1. Bind to `127.0.0.1` by default; remote access only via Tailscale or Cloudflare Tunnel with OAuth/PKCE
2. Signed, reviewed adapters; **no** install-time shell steps
3. Per-assistant scopes (sources, fields, time window)
4. Redact before anything leaves the phone (including Telegram nudges)
5. Downlink is **data**, not tools; actions need confirmation
6. Append-only audit log; encrypted store (OS keychain key); short default retention

---

## 9. Scores and kill gates

| Lens | Score /10 |
|---|---:|
| Open-source project | 6.5 |
| Company later via open core | 4.5 |

**Go if**

- Battery under 5%/day, measured
- 8/10 strangers get a useful nudge
- No raw data leaves without opt-in
- F-Droid or Obtainium install works without Play

**Kill if**

- Play / OEM policies make notification or folder access unreliable on target phones
- On-device extraction quality too low for useful nudges
- No contributor or tester interest after launch

---

## Appendix A — version history (v2–v5)

Working title during these iterations was *life-context bridge*; the product name **Daywire** was chosen after v5.

### v2 (R49) — positioning vs OmniBot / Rouse / Dytto

- Read OmniBot, Rouse Context and Dytto docs; compared with Google Personal Intelligence
- Positioning: *The others expose your phone to an AI. This turns your whole day into private, meaningful events…*
- Differentiators: semantic events, cross-device timeline, timed downlink, policy-as-code consent/audit, adapter ecosystem
- Sources ranked beyond notifications (calendar, desktop screen, location, photos)
- Score: **6.0** OSS (up from 5.5)

### v3 (R50) — exits, licence, on-device feasibility, OpenClaw playbook

- Exit pattern: buyers buy **teams and position**, not code (Bun→Anthropic; ggml→HF; OpenClaw creator→OpenAI; Immich→FUTO; Limitless→Meta; Bee→Amazon)
- Licence recommendation: **AGPL hub**, permissive adapter SDK, trademark, signed official app, paid relay later
- Phone can do text extraction (~16–52 tok/s) and embeddings; vision stays on hub
- OpenClaw timeline and security incidents documented
- Score: OSS **6.0**; company later **4.0**
- Conditional go for a 4-week POC

### v4 (R51) — zero phone-app launch options

- Mapped OpenClaw drivers (`curl|bash`, chat surface, BYO model) to a hub-first path
- Scored delivery options; recommended Layer 0 hub + Telegram + MCP, then borrowed apps, then native APK
- WhatsApp Business API bans general-purpose AI providers since 15 Jan 2026 → **Telegram first**
- Android developer verification timeline documented
- Score: OSS **6.5**

### v5 (R52) — phone-first + Daywire GTM

- Architecture flipped to **phone-first** (this document)
- Both app and hub open source; OpenClaw-lane GTM
- Name shortlist → **Daywire** chosen by the founder
- Prior video-search project: separate repo
- Score: OSS **6.5**; company later **4.5**

---

## Appendix B — condensed earlier research

### OpenClaw facts (verified)

| Fact | Value | Source |
|---|---|---|
| What it is | Self-hosted personal agent; DM it from chat apps | CNBC, docs.openclaw.ai |
| Repo created | 24 Nov 2025 | GitHub API |
| Renames | Clawdbot → Moltbot (27 Jan 2026, Anthropic TM) → OpenClaw (29–30 Jan) | Securonix / Bitsight |
| Stars | 145k by 2 Feb; 200k by 16 Feb; **393,916** on 5 Nov 2026 | CNBC / THN / GitHub API |
| Deal | Creator joined OpenAI (14–15 Feb 2026); project → 501(c)(3) foundation 8 Jul 2026; MIT; **no acquisition price disclosed** | Reuters, CNBC, openclaw.ai blog |
| Security | 30k+ exposed instances; 341/2857 malicious ClawHub skills | Bitsight; Koi / THN |

### Open vs not (v3 recommendation, still current)

| Option | Copy protection | Verdict |
|---|---|---|
| Permissive MIT/Apache | None without a strong trademark | Only with TM |
| **AGPL** | Hosted resellers must publish changes | **Recommended for hub/app core** |
| Open core (paid relay later) | Paid part is the moat | Add later |
| FSL/BSL | Triggers forks (OpenTofu, Valkey) | No |
| Closed | Low trust for this audience | No |

### Hybrid compute (v3 extraction budget — calculated)

Assumptions labelled as such; costs from cited benchmarks.

| Workload (assumed / day) | Unit cost (sourced) | Daily compute (arithmetic) | Where |
|---|---|---|---|
| 100 text events × ~1,100 tokens | S24U GPU ~7.6 s/event; X300 NPU ~4.1 s | ~13 min GPU / ~7 min NPU | Phone, batched while charging |
| 300 embeddings | Under 22 ms each on EdgeTPU (Google) | Under 7 s (EdgeTPU figure) | Phone |
| 30 photo captions | Minutes/photo on Pixel 8 Pro CPU | Not viable on phone | Hub |
| Call / voice audio | Not measured this research | Unknown | Hub by default |

Battery %/day: **no public per-task energy numbers found** — POC must measure.

---

## Appendix C — sources and labels

**Fetched / API-checked this research series:** GitHub API star counts; OpenClaw install docs; Android developer-verification pages; Play Accessibility / sensitive-permissions policy pages; Meta Wearables DAT docs; Gemma 3n / 4 LiteRT Hugging Face model cards; EmbeddingGemma Google docs; Home Assistant Companion sensor docs; TechCrunch / Reuters / CNBC on acquisitions and Pulse/WhatsApp AI rules; USPTO records via public trademark-search sites; dig/RDAP for domain NS presence.

**Labelled unverified or general API knowledge:** per-OEM Xiaomi dialer paths; Play Store title collisions; some iOS API surface details (EventKit/HealthKit/PhotoKit capabilities as commonly documented); Tasker / KDE Connect / PWA capability rows; Exact OpenClaw influencer/YouTube launch channels.

**Not invented:** all star counts, tok/s figures, funding amounts and dates above come from the cited sources. Scores and “value per effort” rankings are **judgement**, not measurements.
