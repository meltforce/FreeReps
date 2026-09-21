# Decisions

Decisions taken about FreeReps, with the reasoning that led to them. One section
per decision, newest first.

A decision belongs here once it has been made — including decisions to *not* do
something, which are the ones most likely to be re-derived from scratch
otherwise. Open work lives in [`ROADMAP.md`](ROADMAP.md); postmortems live in
[`INCIDENTS.md`](INCIDENTS.md).

Structure per entry: **decision**, **reasoning**, **trigger to re-open**, and a
**revisions** log when the decision has changed. A revised decision is edited in
place with the old form recorded under revisions — the entry is not duplicated.

The entries dated before 2026-08-04 were reconstructed on that date from
`CLAUDE.md`, from `app/readiness_assessment.md` (removed in the same change,
last content at commit `2918cef`) and from the commit history. Their reasoning
is as recorded there; where the record named no alternative, none is claimed.

---

## 2026-09-21 — The workout list names the provider that recorded a session, not the path it arrived on

**Decided:** 2026-09-21

**Decision.** `QueryWorkouts` returns, with the surviving row, every source of
its 5-minute window in `Sources`. The list labels a row that has no source name
of its own with the named source of its window; the delivery path moves into the
cell's title. `recordedBy` in `server/web/src/utils/sourceLabel.ts` holds the
rule, `sourceLabel` keeps naming a single source.

**Reasoning.** A session recorded in the Oura app reaches FreeReps twice, over
the Oura API and as the copy the app writes into HealthKit, which Health Auto
Export forwards without a source name. The priority for the `activity` category
ranks Apple Health first, so the row the list shows is the copy — and it was
labelled "Apple Health", which names the hub rather than the recorder. Measured
over 2026-07-23 to 2026-09-22: 52 such pairs, in each of which the energy
figures are identical, so the copy is the Oura workout in every one of them.

A named source is the recorder, because Apple Health is the only source that
arrives without a name — `provider.go` stores `source = ''` for everything from
Health Auto Export, and the `sources[]` entry of the payload that names the
device is dropped (see ROADMAP.md). The rule therefore needs no new data.

**Alternatives rejected.** Ranking Oura first for the `activity` category would
label the session correctly and cost the rest: the category also governs 18
activity metrics including `step_count`, and the row that would then win carries
no GPS track — 42 of the 52 pairs have one on the Apple Health row — and fewer
heart rate rows in 15 of them, none in 4. Rewriting the stored source of the
copy would destroy the distinction the priority rests on, and with it the
ability to tell the two deliveries apart at all.

**Trigger to re-open.** A payload that names the writing app, which would make
the origin a property of the row rather than of its window — the ROADMAP item on
the discarded device name is the same data. Two unrelated workouts inside one
5-minute window would also break the rule, as they already break the
deduplication it rests on.

---

## 2026-09-21 — An Oura workout gets its heart rate from the sample series, averaged per minute

**Decided:** 2026-09-21

**Decision.** The Oura sync derives a workout's heart rate from the
`heart_rate` metrics of the same user, source and interval, writes one row per
minute into `workout_heart_rate`, and sets the workout's summary from those
minutes. Migration `000033_oura_workout_heart_rate` does the same for the Oura
workouts already stored. The rule is not Oura-specific in the code —
`FillWorkoutHeartRateFromMetrics` takes the source as a parameter — but the
sync calls it only for Oura.

**Reasoning.** The Oura API's workout object carries no heart rate. Its fields,
read from a live response on 2026-09-21, are `id`, `activity`, `calories`,
`day`, `distance`, `end_datetime`, `intensity`, `label`, `source` and
`start_datetime`. The samples exist on `/v2/usercollection/heartrate`, which
reports each one with a source of its own, `workout` among them, and the sync
already stores them as `heart_rate` metrics — for the session of 2026-09-21,
92 samples inside the workout interval, of which 77 sit in a 5-second burst over
the first six minutes.

Until now the heart rate of an Oura workout reached the dashboard only over
Apple Health, because the Oura app writes the workout into HealthKit and the
Health Auto Export path carries `heartRateData` with it. A user with Oura alone,
or a user whose export stops, had a workout row without a heart rate although
every sample sat in the database. That was the state on 2026-09-21: 194 Oura
workouts, none with a summary.

**Minutes rather than samples.** Oura records densely while a workout runs and
sparsely afterwards, so an average over samples is an average of the sampling
rate as much as of the heart rate — the same defect the dashboard aggregation
was corrected for on 2026-09-20. For that session the two readings are 98.32
over the samples and 96.16 over the 16 minutes. Per-minute rows also match the
shape the Health Auto Export path writes, so the detail chart reads one kind of
row.

**This does not replace the Apple Health copy.** Measured for the same session:
HealthKit holds three minutes (07:20, 07:50, 07:51) that the API does not
return, including the minimum of 69 bpm and the maximum of 116, while the API
holds the 77-sample burst that HealthKit does not have. Neither series contains
the other, which is why the priority for the `activity` category keeps deciding
which row the list shows rather than one of them being dropped.

**The API returns Oura's own workouts only.** The Oura app shows workouts it
imported from Apple Health, and those do not come back out of the API, so an
`Oura` row is never a re-import of an Apple workout. Measured over 2026-07-23 to
2026-09-22 for this account: 53 workouts from the API against 173 Apple Health
workouts in the same window, with `source` reading `confirmed` 45 times,
`workout_heart_rate` 7 times and `manual` once. The 121 Apple workouts without a
counterpart are 73 × Walking, 15 × Traditional Strength Training, 13 × Cycling,
10 × Flexibility, 6 × Functional Strength Training, 3 × Outdoor Cycling and one
dive. The direction of the 52 pairs is settled by their energy: Apple's
`activeEnergyBurned` in kJ equals the Oura figure in kcal times 4.184 to the last
floating-point digit, in all 52 — the Apple row is the Oura workout written into
HealthKit. The duplication in the workout list is therefore always one Oura
session on two paths, which the 5-minute rule in `QueryWorkouts` resolves.

**Trigger to re-open.** An Oura endpoint that carries heart rate on the workout
itself, or a second source whose workouts arrive without one — the second would
turn the sync's fixed `Oura` argument into a per-source decision. Equally, Oura
beginning to return the workouts it imported: those would arrive as `Oura` rows
with the start times of Apple rows.

---

## 2026-09-20 — Data has its own ramp, and the nav bar has its own surface

**Decided:** 2026-09-20

**Decision.** Sleep stages and heart rate zones read `--color-data-1` through
`--color-data-5` instead of the neutral ramp and the brand token. One hue,
OKLCH 196 at chroma .044–.074, on the same lightness scale as `neutral-500`
through `-900`; the dark variant mirrors it as the neutral ramp does. Zones take
the steps by index, stages by absolute lightness — Deep at the dark end, Awake
at the light one — through four `--color-stage-*` aliases that hold the
theme-dependent reversal in CSS, so `stageColors.ts` stays a static map.

The nav bar takes `--color-surface` and a hairline rule in place of the 2px one;
the mobile tab bar follows. The page header stays on `--color-bg`.

**Reasoning.** With data on the brand token, an awakening in the hypnogram was
drawn in the same blue as the nav marker and the selected range button, and the
neutral ramp coloured table rules and sparklines at the same time, so data and
frame were not separable. This is the piece the role split of the same day named
as unfinished.

The order lives in lightness rather than hue, at about .09 L per step: petrol
collapses to grey under protanopia and deuteranopia, so the hue identifies the
series for viewers without a deficiency while the order holds for everyone. The
hue sits 58° from the brand blue and 128° from the negative amber at about half
their chroma, which keeps a data area from reading as selected or as judged, and
the brand blue keeps its blueness in the same simulations. Chroma is pulled back
at both ends of the ramp, away from the sRGB gamut wall, where a 4px hypnogram
lane shifts in tone with the display profile.

The scale carries no `color-mix()` and no alpha: `tokenColor()` resolves these
for the uPlot canvas, and a transparent step shifts its lightness against the
ground, which breaks the order the ramp exists to carry.

**Alternatives rejected.** A separate tone for the top step — today's accent
role for Awake and zone 5 — would break the ramp as an ordered series and add a
third special colour next to brand and negative. The step out comes from the
lightness jump instead: zone 5 is the ramp's strongest contrast, and between REM
and Awake sits a skipped step, so `data-2` on light ground stays unused. Hue 176
(jade) was measured as safer against confusion at 78° from the brand, and read
as a foreign green family next to `#1d5fa8`.

Two scales, one per series, were rejected because stages and zones carry the
same semantics and never share a screen; the shared lightness ladder is what
holds a REM swatch and a zone 3 swatch at the same visual weight.

**In-bar labels.** `--color-neutral-100` on `--color-data-2` reaches 3.4:1 and
`--color-text` on it 3.8:1, both under the 4.5:1 that 12.5px text needs, so only
`data-4` and `data-5` can carry a label inside the fill. The composition and
zone bars already put their labels outside it, so no component changed.

**Measured series carry the ramp; frame and derived references stay neutral.**
A chart with one series takes `--color-data-3`: the metric line
(`MetricChart.tsx`), the daily series in the Trends small multiples, the heart
rate line (`HRTimelineChart.tsx`) and the active bar of the correlation-by-lag
list. Axes, grids, ticks and labels keep `--color-neutral-300`/`-600` and the
2px baseline keeps `--color-text`. The 7-day rolling mean stays a dashed
`--color-neutral-500`, because it is context to the daily value rather than a
measurement of its own, and the dashed neutral separates it from the solid data
line.

Two derived references take a ramp step anyway, because each is the statement
its screen exists to make: the regression line in `Scatter.tsx` and the Pearson
figure on Correlations both take `--color-data-5`, so the number and the line
read as one thing. Drawn neutral, the line would disappear among 150
half-transparent points. The scatter's points take `--color-data-2` at 55 %.

The p25–p75 band gets `--color-data-band` (`#dfeaea` light, `#192a2a` dark), one
step of its own rather than a mix, so the uPlot path resolves it too. It
previously used `--color-accent-100`, which is also the table hover.

`VERDICT_COLOR` in `trend.ts` moves to the `--color-positive-700` /
`--color-negative-700` pair that `deltaColor()` already uses, so the Trends page
and the delta column of the metric table state the same thing in the same
colour. The Trends legend becomes four entries, because one swatch labelled
"Fitted trend" named a colour the chart never draws.

What keeps the brand: the metric rail selection in `MetricsPage`, the range
switch, and the workout type filter — those are selection, not data.

The minimum block height in `NightsChart.tsx` rises from `PLOT_HEIGHT * 0.0045`
to `* 0.01`, 1.35px to 3px. Awake sits at the light end of the ramp at 2.4:1
against the ground, which a hairline does not carry; the fix belongs to the
layout, because raising that step's contrast would break the stage order.

**Trigger to re-open.** A thin `data-1` lane — the step reaches 2.4:1 on light
ground and 2.0:1 on dark, which carries a wide block and not a 4px lane. The fix
would lower that step by about .04 L rather than raise its chroma. A sixth
series with more classes interpolates in the lightness space at hue 196 rather
than adding a second hue.

**Design handoff:** `design_handoff_datenfarben`, which supersedes
`design_handoff_farbsystem` on this point.

**Revisions.** 2026-09-20: extended from the two classified series to every
measured series, with the chart rule, `--color-data-band` and the `NightsChart`
minimum height, after the handoff added them. The ramp, the mapping and the
rejected alternatives are unchanged.

---

## 2026-09-20 — The front page keeps the per-metric latest-value lookup

**Decided:** 2026-09-20

**Decision.** The front page loads in about 0.65s, and `GetLatestMetricsFor` in
`server/internal/storage/health_metrics.go` keeps its per-metric
`LATERAL … LIMIT 1`. No further work is done on the lookup.

**Reasoning.** The load went from about 5s to about 0.65s across `a3c634c`,
`5a75915` and `8c1ed3d`, the four cores given to `freereps-lxc` and the memory
fix recorded in the homelab repo. `pg_stat_statements` is enabled on the
instance and attributes 531ms of the remaining 652ms to one statement, the
per-metric lookup; the daily series accounts for 76ms and every other statement
for under 1ms. The remaining 0.65s was not raised again after `8c1ed3d`.

**Alternatives rejected, both measured.** Widening or staging the time window
changes nothing, because the cost scales with the number of per-metric lookups
rather than with the window: `bc4196a` made the page about a third slower and
was reverted in `03cbb3b`. A continuous aggregate addresses the 76ms
daily-series half of the profile, not the 531ms half.

**What a fix would have to change.** Either the chunk layout — the 514 7-day
chunks merged into fewer — or the query shape, deriving the latest value from
the daily-series query that already runs, which needs `max(time)` and the
winning source added to it. Both are recorded here so the two dead ends above
are not measured a second time.

**Trigger to re-open.** The load is raised as a complaint again, or the number
of allowlisted metrics grows, since the 531ms scales with the count of
per-metric lookups.

---

## 2026-09-20 — No FreeReps endpoint is reachable outside the tailnet

**Decided:** 2026-09-20

**Decision.** The public instance at `https://freereps-test.meltforce.net/`,
deployed without Tailscale so that an App Store reviewer could reach it, is shut
down. Every FreeReps endpoint is reachable inside the tailnet only, which
restores the state `DECISIONS.md`, 2026-03-15 describes.

**Reasoning.** The review instance was a bounded exception: it served demo data
rather than real health data, and it existed because a reviewer cannot join the
tailnet. App Store approval removed the reason for it. Neither
`freereps-test.meltforce.net` nor `freereps-test.coydog-fence.ts.net` resolves
as of 2026-09-20, while `https://freereps.coydog-fence.ts.net/api/v1/version`
answers, which is the check the Uptime Kuma row in the homelab repo performs.

**Residual.** `configuration/nihilist/roles/caddy/files/Caddyfile:140` in the
homelab repo notes that per-site configs on that host are managed outside the
repo and names `freereps-testserver` as the example. Whether a site file for it
remains on `nihilist` is not visible from this repo. DNS does not resolve, so a
remaining file serves nothing.

**Trigger to re-open.** A further App Store submission whose review requires a
server the reviewer can reach.

---

## 2026-09-20 — Sleep belongs to one channel, decided at the entrance

**Decided:** 2026-09-20

**Decision.** The ingest endpoint (`POST /api/v1/ingest`) does not write sleep —
neither sessions nor stages — when the user's source priority for the `sleep`
category resolves to a provider that synchronises through its own channel.
`sleepSyncSources` names those providers: Oura and Withings. Everything else in
the same payload is stored as before, the sleep category samples included; only
their extraction into `sleep_stages` is refused.

**Reasoning.** A provider is not a path. Oura's nights arrive twice: once from
its API, once from HealthKit, where the Oura app writes them and the FreeReps
iOS app forwards every category type it finds. Both deliveries carry
`source = 'Oura'`, so nothing downstream can tell them apart — not the unique
index, whose interval bounds differ by seconds between the two, and not a source
priority, which would compare a name against itself. Every night was therefore
stored twice (`INCIDENTS.md`, 2026-09-20).

Resolving it at read time, the way cumulative metrics are resolved since the
same day, is not available here for that reason: the two candidates are
indistinguishable once stored. The decision moves to the entrance, where the
channel is still known.

The rule reads the priority the user already configures rather than a flag of
its own, so the setting that says "Oura owns my sleep" is the setting that stops
the second copy. A category rule outranks `_default`, exactly as
`ResolveSourcePriority` reads it: naming Apple Health for sleep while Oura leads
everywhere else keeps sleep flowing through the endpoint.

**Alternatives rejected.** Dropping sleep from the iOS app's sync would decide
for every user from one user's setup, and the app cannot know what the server
already has. Refusing sleep from the endpoint unconditionally would lose it for
anyone without a syncing provider, for whom this is the only route.

**Trigger to re-open.** A provider that syncs sleep but leaves gaps the
HealthKit path would fill. The rule takes the whole category from one channel;
it does not merge them.

---

## 2026-09-20 — Colour carries four roles, not one

**Decided:** 2026-09-20

**Decision.** `--color-accent` no longer means brand, selection, data and
judgement at once. Four roles, each with its own token:

- **brand** — nav marker, links, buttons, selected states
- **positive** (`--color-positive*`) — a metric moving the right way
- **negative** (`--color-negative*`) — a metric moving the wrong way
- **data** — sleep stages, heart rate zones; still on the brand token, see the
  open item in `ROADMAP.md`

`deltaColor()` in `server/web/src/utils/metricDirection.ts` gives regression
`--color-negative-700` instead of the neutral tone it shared with movement that
carries no judgement. Metrics whose `DIRECTION` is `"neutral"` — weight, BMI,
heart rate, respiratory rate — stay neutral whichever way they move.

Ground and ink move with it: paper is cool-tinted rather than near-white
(`#edf0f1`), ink is blue-grey rather than near-black (`#232c33`), and the
neutral ramp is pulled cool to match. Type, spacing, radii and layout are
untouched.

**Reasoning.** One colour cannot mean "this is us" and "this is good" at the
same time. With a single accent, every hovered table row read as signalled, and
a worsening metric had no colour of its own: `deltaColor()` returned the same
neutral tone for regression and for a number that had not really moved.

Blue against amber rather than green against red. Red-green deficiency affects
roughly 8% of men, while blue and orange stay distinguishable under all the
common deficiencies. Both tones sit at the same lightness and saturation and
differ only in hue, so neither reads as louder than the other.

`--color-positive` deliberately equals `--color-accent` on light ground. It is
a token of its own so brand and improvement can part later without touching a
call site.

**Trigger to re-open.** Data semantics still borrow the brand token
(`stageColors.ts`), which is the piece this change does not finish. Should
brand and positive need to differ, `--color-positive` is already the seam.

---

## 2026-09-20 — Source selection and sample reduction are separate steps

**Decided:** 2026-09-20

**Decision.** Reading a metric resolves the source first and aggregates second,
and the two no longer share one window function.

- The dedup CTE marks every row of the winning source with `rn = 1` through
  `FIRST_VALUE(source)`. Callers keep their `WHERE rn = 1`, which now removes
  competing sources and no longer removes samples of the source that won.
- A cumulative metric resolves its source **per day**, every other metric **per
  5-minute window**.
- A cumulative metric is aggregated as the sum over all rows of the winning
  source. Every other metric is averaged within each 5-minute window and then
  across those windows, with `MIN` and `MAX` taken over the rows.
- A query spanning several metrics resolves each metric with the priority of
  its own category, not with `_default`.

**Reasoning.** The interval a source states a quantity over differs per source:
Oura writes a day's steps as one row, Apple Health writes hourly blocks, and
Health Auto Export can be configured to write per-second samples. Any rule
fixed to one window is therefore wrong for some pairing, which produced 32,361
steps for a day of 16,652 (`INCIDENTS.md`, 2026-09-20).

Resolving per day is right for a counter because the two candidate figures cover
the same day and adding them doubles it. Resolving per day is wrong for a
sampled value because the leading source is not obliged to cover the whole day:
on 2026-09-18 Apple Health held 54 heart rate windows in which Oura had no row,
and a daily choice would have deleted the morning training session from the
day's figure.

Averaging the rows of a sampled metric directly was rejected for the same reason
it looks correct. On 2026-09-18 six workouts covered 16% of the day and held 77%
of the heart rate rows, because a workout samples per second and a quiet hour
does not. An average over rows is an average of the sampling rate as much as of
the heart rate, so the windows are averaged first and weigh alike.

The alternatives considered and rejected:

- **Keep one row per window** (the previous behaviour). Correct for neither
  class: a counter loses every sample but one, and a sampled value is
  represented by whichever row sorted first.
- **Skip the dedup entirely for cumulative metrics**, as proposed in
  `meltforce/FreeReps#1`. It repairs the sample loss and reintroduces the
  double count, because two sources reporting the same day then both contribute.

**Trigger to re-open.** A source that reports a cumulative metric more than once
per day with overlapping coverage, which the daily choice cannot resolve. Also:
Apple Health devices are stored under one empty source name because the HAE
ingest discards the device name from `sources[]`, so iPhone and Watch cannot be
told apart by priority. A user whose export carries both devices separately
would have their steps counted twice; storing the device name is the fix and is
tracked in `ROADMAP.md`.

---

## 2026-09-20 — The OAuth redirect URIs are derived from the request, not configured

**Decided:** 2026-09-20

**Decision.** `/oura/callback` and `/withings/callback` are built per request from
the origin the request arrived on (`internal/server.callbackURL`), honouring
`X-Forwarded-Proto` and `X-Forwarded-Host`. `server.base_url` overrides the origin
and is empty by default. Both integrations report the resulting value in their
status endpoint, and each Settings tab shows it as the string to paste into the
provider's form.

**Reasoning.** The two URIs were compile-time constants naming one deployment's
MagicDNS name, which made the OAuth flow unusable for any other installation and
silently wrong in local development. Three properties decided the shape:

- **The request already carries the right answer.** The user reached the UI on the
  host the provider will redirect back to, so that host is the one to register.
  Deriving it means a fresh installation configures nothing, and renaming a host
  changes the URI without an edit.
- **Start and exchange have to agree.** The provider compares the redirect URI
  sent when the flow opens with the one sent when the code is exchanged. Both
  calls build it the same way, and the callback arrives on the same origin as the
  request that opened the flow.
- **A displayed value beats a documented one.** The URI is the one setup value an
  operator cannot look up, and a guess that differs in scheme, port or hostname
  fails at the last step of the flow rather than when it is entered.

**Why an override exists anyway.** A reverse proxy under a name the forwarding
headers do not carry, and a UI reachable under several names while the provider
accepts one registered URI. A `base_url` carrying a path is refused at startup,
because it would produce `…/app/oura/callback` while the router serves
`/oura/callback` — a mismatch the provider reports at the end of a flow.

**Trigger to re-open.** A provider that requires a URI registered per user rather
than per application, or a deployment that terminates TLS without setting either
forwarding header.

---

## 2026-09-20 — A failing data source reports itself to an ntfy topic

**Decided:** 2026-09-20

**Decision.** FreeReps posts its own alerts as a JSON object to a configured ntfy
topic, in the shape Uptime Kuma's webhook notification produces: `schema` (the
number 1), `monitor_id`, `service` and `status` (0 = problem, 1 = resolved), plus
`hostname`, `monitor_type`, `since` and `msg`. Following Kuma's field names rather
than inventing better ones means a consumer that already parses Kuma's webhooks
needs no second parser.

The ids sit in the block 9200–9299, clear of the monitor ids a Kuma instance on
the same topic hands out from 1 upwards
(`server/internal/alerts/watcher.go`):

| id | Condition |
|---|---|
| 9200 | the manual channel test from Settings → Alerts |
| 9201 / 9202 / 9203 | the Withings, Oura and Hevy sync failing repeatedly |
| 9204 | no Health Auto Export delivery for longer than the silence threshold |

Four sub-decisions that are not obvious from the code:

- **The rules read `import_logs`, they do not hook into the sync loops.** A
  syncer that stopped running writes no row at all, and a rule that fires on a
  failed run would stay silent for exactly that case. The same query then also
  covers the Apple Health path, which this server does not poll — there the
  condition is silence.
- **The configuration is a database row, edited in the Settings UI**
  (`alert_settings`, one row, seeded once from `config.yaml`). Every other
  integration in this project is configured in that screen, and a value that
  lives only in the deployed config file needs an Ansible run to change. The seed
  is one-directional so a redeploy cannot overwrite what was entered.
- **Three consecutive failed runs for one user, not one.** The Withings sync
  produced isolated DNS failures (`server misbehaving`) among the 2,197 real
  ones; at a 30-minute interval the threshold reports a defect within two hours
  while a single timeout stays out of the session.
- **The state is stored and the message sent on the transition only.** A consumer
  groups messages by `monitor_id`, so repeating `status: 0` every cycle adds
  nothing to what it already shows. The state is written after the send
  succeeded, so an unreachable topic delays an alert instead of swallowing it.
  For the same reason an id stands for one condition permanently: two conditions
  sharing one id become indistinguishable on the receiving side.

**Reasoning.** The Withings integration failed every 30 minutes for 46 days and
nothing said so ([`INCIDENTS.md`](INCIDENTS.md), 2026-09-20); the only signal was
a row in `import_logs` that nothing reads on a schedule. Uptime Kuma cannot close
that gap from outside: the container is healthy and the HTTP endpoint answers
while a source silently delivers nothing, which is what its monitors check.

**Trigger to re-open.** A second consumer of the channel that needs a different
payload shape; a condition whose firing rate makes the channel noisy enough to be
muted; or a move off ntfy, which would change the transport but not the
per-condition id.

**Where the receiving side is documented.** The deployment this was built for
runs the consumer in its own infrastructure repository, which registers the id
blocks of every sender writing to that topic — including this project's
9200–9299. That document is the place to look when an id has to be added; nothing
in FreeReps depends on it.

---

## 2026-08-10 — The Alpha session timezone is configuration, and the natural key stays at the instant

**Decided:** 2026-08-10

**Decision.** The zone the Alpha Progression export's session times are read in
comes from `ingest.session_timezone` (`server/internal/config/config.go`,
default `Europe/Berlin`), never from `time.Local`. Startup fails on an unknown
zone name, on the empty string and on `"Local"` — the two spellings
`time.LoadLocation` accepts as "whatever this host happens to be".

Two sub-decisions that would otherwise be re-derived:

- **The unique constraint keeps identifying a session by its instant.**
  `workout_sets_source_natural_key` stays
  `(user_id, source, session_date, exercise_number, set_number, is_warmup)`.
  Widening it to the calendar day would have caught the duplication, and is
  rejected: two genuine sessions on one day overlap in `exercise_number` and
  `set_number`, so a day-level key silently drops the second session's sets —
  the same class of loss as [`INCIDENTS.md`](INCIDENTS.md), 2026-04-08. The
  instant is the right key; it just has to be computed deterministically.
- **The zone database is compiled into the binary** via a blank
  `time/tzdata` import in `internal/config`. The runtime image installs
  `tzdata` today, so this changes nothing about the current deployment; it
  removes the case where a base-image change turns a valid zone name into a
  startup failure.

**Reasoning.** The export carries a bare wall clock and nothing that identifies
its zone, so some zone has to be supplied. Taking it from the process
environment makes the stored instant a property of the host that ran the
import: the same file produced 08:22Z on a machine in Europe/Berlin and 09:22Z
in the deployed container, which carries no `/etc/localtime` and therefore runs
in UTC. Because that instant is part of the row's natural key,
`ON CONFLICT DO NOTHING` saw two different sessions and stored the full history
twice.

`Europe/Berlin` is the default rather than UTC because it is the zone the stored
history was written in; a different default would move every future session
relative to the sessions already stored, which is the failure this setting
exists to prevent. An installation elsewhere sets the key.

**Trigger to re-open.** A second user in another zone, which turns a server-wide
setting into a per-user one; or Alpha Progression adding a zone or a UTC offset
to its export, which would make the setting unnecessary for new files while the
stored history still depends on it.

---

## 2026-08-05 — Withings is read directly, and the Apple Health path stays

**Decided:** 2026-08-05

**Decision.** FreeReps reads weight, body composition and blood pressure from the
Withings Public API (`internal/withings/`, wire format in
[`server/specs/withings-api.md`](server/specs/withings-api.md)), on the same
shape as the Oura integration: per-user app credentials in the database, OAuth2
consent through the settings tab, a 30-minute poll with a 90-day backfill.

Three sub-decisions that are not obvious from the code:

- **The Health Auto Export path is not disabled.** It stays the only route for an
  installation without a Withings account. Overlap is resolved by source
  priority, whose default becomes `["Withings", "Oura", ""]`
  (`internal/config/config.go`).
- **Blood pressure is written as two qty metrics**, `blood_pressure_systolic`
  and `blood_pressure_diastolic`, not through the `Systolic`/`Diastolic` columns
  of `HealthMetricRow`. Those columns exist for the HAE shape, but
  `blood_pressure` has no `metric_allowlist` entry, so rows written that way are
  rejected at ingest. The dashboard, the correlation picker and the iOS app all
  work with the split names.
- **The pulse the blood pressure cuff records gets its own metric**,
  `blood_pressure_heart_rate`, rather than joining `heart_rate`. A single seated
  measurement in the same series as the continuous heart rate from Oura and the
  Apple Watch shifts every daily average, and the two stop being comparable.

**Reasoning.** The measurements existed in FreeReps only via Health Mate → Apple
Health → Health Auto Export. That chain advances when the Health app on the
phone syncs, which is not on a schedule anyone controls. The Withings Public API
tier requires no contract and no approval, so the direct read costs one
registered application.

Two API properties drove the implementation and are the reason the code deviates
from the Oura equivalent in two places:

- **Errors arrive with HTTP 200** and a non-zero `status` in the body. Branching
  on the HTTP status alone turns every failure into a successful empty result.
- **The refresh token rotates** and the previous one stops working within hours.
  `UpsertWithingsToken` is therefore an upsert rather than the bare `UPDATE`
  used for Oura, where a row that does not exist makes the write a silent no-op.

**Trigger to re-open.** Withings moving the measure endpoints behind a paid plan;
a Withings webhook subscription replacing the poll; or the Health Auto Export
path being retired for other reasons, which would remove the need for a priority
rule at all.

---

## 2026-08-05 — The web UI runs on the Modernist design system, with one phone breakpoint

**Decided:** 2026-08-05

**Decision.** The web UI is rebuilt against an external design package
(`design_handoff_freereps_redesign`) rather than continuing the dark Tailwind
default. What that fixes in structure, beyond the visual system:

- **The front page is one request.** `GET /api/v1/metrics/latest` returns, per
  visible metric, the latest value, a 7-day delta, a percentile range and a
  daily series. `GET /api/v1/dashboard/init` is removed; the dashboard no longer
  calls `available-metrics` or `timeseries` at all.
- **Sparklines are inline SVG `<polyline>`**, so no chart library loads on the
  front page. uPlot remains only on the workout detail route.
- **One breakpoint at 768px**, not a second app. The same page component picks
  between a table body and a row body via `useMediaQuery`.
- **Metrics and Correlations are desktop-only.** Both need width the phone does
  not have — a 248px rail beside a 420px chart, and a 720×520 scatter beside a
  440px column. Below 768px they stay reachable by URL and render a notice.

**Reasoning.** The old dashboard rendered ten metric cards plus a full
time-series chart on load: three API calls and a chart library before the first
number appeared. Carrying ~30 floats per metric in the latest payload costs less
than a second round trip, and it lets the front page answer "how am I doing
today" without loading a plotting library at all. Routing every colour through
tokens is what makes the three-way theme switch a variable swap rather than a
second stylesheet.

**Deviations from the package, and why.** The design's Settings rail names five
tabs; the shipped rail has seven. Hevy and Import drive working integrations
that the package does not mention, and dropping them with the old layout would
have removed function, not styling. The Metrics chart is inline SVG rather than
the lazy-loaded uPlot the package suggests, because the design it specifies —
band, gridlines, baseline, two polylines — needs no plotting library.

**The stated range is p05–p95, not min–max.** One dropped sensor reading would
widen a min/max range enough to make the number meaningless.

**The home screen icon carries a light mark, against the package.** The package
draws ink `#201e1d` on the accent field. iOS 18 renders home screen icons in a
light, a dark and a tinted mode, and a web app cannot supply a separate dark
variant — iOS derives it from the one icon. With the mark darker than the field
(luminance 0.013 against 0.200) both collapse toward black in the dark mode and
the icon reads as an empty rounded square. The mark is now `#f3f2f2` at
luminance 0.890, so it stays the brighter element through the conversion. Form,
kerning and the rule are untouched; only the fill changed.

**Distance is normalised to kilometres in one place.** Apple Health reports
walking and cycling distance in metres, and the summary strip summed the raw
field under a fixed "km" label — 428 km rendered as 427 955. Every distance on
screen now passes through `distanceKm`, which converts by the row's own unit.
The same class of failure as the metric units in
[`INCIDENTS.md`](INCIDENTS.md), 2026-03-26: a unit that varies per row and a
label that does not.

**Zone bands derive from the 99.9th percentile heart rate, not the maximum.**
Verified against the deployed instance, `MAX()` returned 210 bpm from a single
strap dropout. At that peak the second zone starts at 126, which put whole
strength sessions in zone 1 and made the bars carry no information. A genuine
maximum effort contributes many samples near the top; one artefact contributes
one.

**An empty `source` reads as "Apple Health", not "—".** HealthKit writes through
Health Auto Export without setting the field, and the priority rules already
match it as the empty string. Six of seventeen visible metrics were showing an
em dash for an origin that is in fact known.

**`GetLatestMetrics` now resolves source priority.** It previously picked by
timestamp alone, so a lower-priority device writing a minute later decided both
the shown value and the source name — beside a sparkline computed from the
higher-priority device, which does dedupe. Priority now decides within a
5-minute bucket and recency between buckets, matching every other query. Same
failure shape as [`INCIDENTS.md`](INCIDENTS.md), 2026-03-25.

**Trigger to re-open.** A screen whose data cannot be served from one request
without a second round trip, or a phone layout that needs different information
rather than a different arrangement of the same information.

---

## 2026-08-05 — How training volume and estimated strength are computed

**Decided:** 2026-08-05

**Decision.** Three conventions underlie every strength aggregate:

- **Volume per muscle group is reported twice** — as sets whose exercise targets
  the muscle directly, and as a weighted count that adds assisting muscles at
  0.5. Neither figure is presented as *the* volume.
- **Estimated one-rep max uses Epley over repetitions plus reps in reserve**:
  `kg × (1 + (reps + rir)/30)`. Sets without an effort rating are excluded.
- **Effort is read from `effort_rir`**, the generated column that resolves RIR
  and RPE, so both logging scales feed the same bands.

**Reasoning.** Each of these is a convention, not a measurement. No data in this
system says how much of a bench press the triceps carry, and Epley is a linear
approximation that drifts above roughly ten effective repetitions. Reporting two
volume figures keeps the 0.5 weighting from disappearing into a single number,
which is what the 2026-02-19 decision against opaque scores asks for. The reps in
reserve enter the strength estimate because a set stopped two short of failure
demonstrates the strength of a longer set — leaving them out understates about
half the sets in this history.

**Also decided.** Every volume figure carries `approximate_pct` per muscle group
and `unmapped_sets` per period. The first says how much of it rests on exercise
names mapped onto a near equivalent, the second how many sets reach no catalog
entry at all. A volume number without them would be a statement about an unknown
fraction of the training.

**Trigger to re-open.** A source that reports muscle involvement per set rather
than per exercise, or an effort scale that does not map onto reps in reserve.

---

## 2026-08-04 — Hevy replaces Alpha Progression, ingested by polling the event feed

**Decided:** 2026-08-04

**Decision.** Hevy Pro replaces Alpha Progression as the training logger. The
server ingests it by polling `GET /v1/workouts/events?since=` on a ticker, in the
same shape as the Oura sync. Hevy's webhook is not used.

**Reasoning.** The Alpha path was a manual CSV upload and it did not hold: on the
day of this decision the newest row in `workout_sets` dated 2026-05-16 while
`workouts` carried strength sessions from Apple Health up to 2026-06-05. Hevy's
event feed delivers updates *and* deletions since a timestamp, which makes an
outbound poll idempotent and lets a correction made in the app reach the server.

**Alternative considered.** Liftosaur, whose Liftoscript programs carry the
progression rule inside the plan, and whose API can validate a generated program
before it goes live. Rejected because its history format needs a parser where
Hevy returns JSON, because it has no delta endpoint, and because RPE per set —
which the existing intensity analysis depends on — is documented for Hevy and was
not confirmed for Liftosaur.

**Alternative considered.** Hevy's webhook, which POSTs to a registered URL when
a workout is saved. Rejected because it requires a publicly reachable endpoint
with its own authentication, which would reopen the 2026-03-15 decision below,
and because its payload carries only a workout id — the data still has to be
fetched outbound. Its only gain is latency.

**Cost accepted.** Hevy routines carry no progression logic, so load progression
between two planning passes is carried by rep ranges and the previous-session
values the app displays, not by the plan itself.

**Trigger to re-open.** Hevy drops the event feed, RPE stops arriving per set, or
the progression gap turns out to need automation after all.

---

## 2026-08-04 — FreeReps stays the system of record; planning lives outside it

**Decided:** 2026-08-04

**Decision.** Training planning and analysis run in a separate Claude Code
project that talks to the FreeReps MCP server and to a Hevy MCP server. FreeReps
gains no planning logic, no prescription engine and no writes back into the
training app.

**Reasoning.** This keeps the 2026-02-19 decision intact — data and
visualization, no computed scores, no coaching. It also puts the split where the
data is: analysis belongs on FreeReps, which is the only place that sees training
alongside sleep, HRV and readiness. The current plan exists only in Hevy, so
reading and writing it belongs there.

**Consequence for the MCP setup.** Both servers offer overlapping read tools —
`hevy-mcp` ships `get-training-summary` next to the FreeReps `get_strength_summary`
(named `get_training_summary` when this was decided).
The planning project denies the Hevy analysis tools so evaluation cannot
accidentally run on training data alone, without the recovery context.

**Trigger to re-open.** A prescription that needs recovery data as an input — a
deload triggered by a measured HRV decline rather than by a calendar week — since
no external service can compute that.

---

## 2026-08-04 — Forgejo is the source of truth, GitHub is a mirror

**Decided:** 2026-08-04 (commit `3ba3b75`)

**Decision.** `git.coydog-fence.ts.net/meltforce.net/freereps` is `origin` and
the only push target. `github.com/meltforce/FreeReps` receives a
`git push --mirror`. Two workflows stay on GitHub — `ios.yml` and `release.yml`
— because they need a macOS runner and Docker Hub respectively.

**Reasoning.** CI, registry and deploy target are all inside the tailnet; a run
that starts on GitHub has to reach in from outside. The exception is the Xcode
build, for which no macOS runner exists on the Forgejo side.

**Consequence that bites.** `--mirror` force-pushes *and* prunes refs absent on
Forgejo. Anything that must survive on GitHub has to exist on Forgejo first — a
branch created only on GitHub is deleted at the next sync.

**Trigger to re-open.** A macOS runner becomes available inside the tailnet, or
the mirror's pruning costs something that outweighs having one source of truth.

---

## 2026-03-25 — Oura and Apple Health are merged at query time, not at ingest

**Decided:** 2026-03-25 (commit `a12046a`, extended by `71785c0`)

**Decision.** Both sources write their own rows. Deduplication happens in the
query path through a per-user, per-category source priority, configurable in
Settings. No source is normalized away on ingest.

**Reasoning.** The two sources disagree about the same night in ways that are not
resolvable at write time: Oura reports one long sleep session, Apple Health
reports several fragments, and which one is right depends on the metric. Keeping
both rows preserves the raw data the project is built around, and priority is
then a display decision that can be changed without re-importing.

**Alternative considered.** Merging on ingest into one canonical row. Rejected
because it destroys data at the point of no return, and because the correct
priority differs per metric category.

**Cost accepted.** Every query that reads a metric carries the dedup CTE.
`67e35d5` added a covering index for it after the dashboard first load became
measurably slow.

**Trigger to re-open.** A third source arrives whose overlap cannot be expressed
as a priority order.

---

## 2026-03-15 — Tailscale is the authentication layer; the app adds none

**Decided:** 2026-03-15 (recorded in `app/readiness_assessment.md` § 2, commit `2918cef`)

**Decision.** No application-level authentication — no API keys, no bearer
tokens. The server runs `tsnet`; iPhone and server must be on the same tailnet,
which supplies TLS and identity.

**Reasoning.** Adding an application auth layer would duplicate what Tailscale
already provides and introduce credential management for no gain in the
deployment model this project targets.

**Alternative considered.** API keys per device. Rejected on the above; it also
moves a secret onto the phone, which the current design avoids entirely.

**Where it does not hold.** The app accepts an arbitrary host/port/HTTPS
configuration for local development and App Store review. There, securing the
endpoint is the operator's responsibility — see the open row about the review
test server in [`ROADMAP.md`](ROADMAP.md).

**Trigger to re-open.** A deployment that cannot use a tailnet, or multi-user
support, which is a v1 non-goal below.

---

## 2026-02-19 — Data and visualization, no computed scores

**Decided:** 2026-02-19 (project start; stated in `README.md` § Design Principles)

**Decision.** FreeReps stores raw data and visualizes it. It computes no
composite scores — no Recovery, no Exertion, no readiness figure. Analysis is
delegated to Claude through the MCP server.

**Reasoning.** A proprietary score is an opaque function of inputs the user
cannot inspect, and every such algorithm encodes assumptions that do not
generalize across bodies. Raw data plus a free correlation explorer plus an LLM
gives the same answers with the derivation visible.

**Not doing, for the same reason.** Workout planning and automated coaching.

**Trigger to re-open.** A score that can state its inputs and its formula in the
UI, and that answers a question the correlation explorer cannot.

---

## 2026-02-19 — Non-goals for v1

**Decided:** 2026-02-19 (project start)

**Decision.** Out of scope: a native iOS/watchOS app beyond the sync companion,
direct Apple HealthKit integration on the server, multi-user support, third-party
integrations such as Strava, and push notifications.

**Reasoning.** Each of them widens the surface without serving the core loop —
collect, store, visualize, expose over MCP. Multi-user in particular would reach
into every query and into the auth decision above, which currently rests on
"one tailnet, one person".

**Note.** Per-user rows already exist in the schema (metric visibility, source
priority, Oura tokens). That is per-identity storage behind Tailscale identity,
not multi-user support: there is no tenancy boundary and no sharing model.

**Trigger to re-open.** A second person actually uses an instance.

---

## 2026-02-19 — Stack: Go, React, PostgreSQL + TimescaleDB

**Decided:** 2026-02-19 (project start; the table this replaces lived in `CLAUDE.md`)

**Decision.**

| Component | Choice | Reasoning |
|---|---|---|
| Backend | Go | Single binary with the web UI embedded via `go:embed`, which is what makes the self-hosted deployment one artifact. |
| Frontend | React 19 + Vite + Tailwind CSS 4 | Chart ecosystem and TypeScript. |
| Charts | uPlot for time series, Recharts for bar and scatter | uPlot renders the large series without dropping frames; Recharts composes declaratively where the data is small. |
| Database | PostgreSQL + TimescaleDB | Hypertables and rolling aggregates for time-series queries. |
| MCP transport | stdio and SSE | stdio for a local Claude Code session, SSE for remote access over the tailnet. |
| Deployment | Docker Compose | Database and app in one stack, multi-stage build. |

**Consequence that bites.** The `go:embed web/dist` directive means the backend
does not compile without that directory. Every build path needs the frontend
built first, or a stub — `.forgejo/workflows/ci.yml` creates the stub explicitly,
and `server/CLAUDE.md` documents the local equivalent.

**Trigger to re-open.** TimescaleDB licensing or packaging changes, or a chart
requirement neither library covers.
