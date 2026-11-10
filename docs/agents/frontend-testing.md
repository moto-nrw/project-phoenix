# Frontend tests: clock, timers, absence assertions, kit pixel comparison

Read before writing or diagnosing a date- or time-sensitive Vitest test, a
test that asserts something is absent, or before proving that a kit change
is invisible.
Paths start at `frontend/`; commands run from `frontend/`.

## Projects

`vitest.config.ts` runs four projects. `api-node` covers `src/app/api/**`.
`logic-node` runs the files listed in `src/test/node-test-files.ts` in the
node environment with only `setup-common.ts`. Everything else runs in
`app-dom` (happy-dom plus `setup.ts`). List a new `*.test.ts` in
`node-test-files.ts` when it renders nothing and uses no DOM, window, storage,
next-intl, SWR or tenant-context mock, and confirm it passes with
`pnpm vitest run --project logic-node`. Keep a test in `app-dom` when a module
it imports branches on `window`, `document` or storage: under node it would
pass on the server branch only.

`scripts/test-changed.sh` runs only the filesystem guards when the frontend
itself did not change (backend or Markdown inputs). Locally it skips that run
when every non-ignored file, the installed dependencies, the frontend env files
and Node are unchanged since the last pass; `PHX_GUARDS_CACHE=off` forces it.
CI never reuses a result.

`school-date-dom` runs the school-date component suites listed in
`schoolDateTestFiles` in `frontend/vitest.config.ts`, using happy-dom and forked
processes. Its regression cases change `TZ` to UTC, Pacific/Honolulu and
Pacific/Kiritimati at runtime (the original sick-report suite also uses Tokyo);
Node worker threads do not apply those timezone changes. The cases assert the
local date first so an ineffective timezone change cannot hide the defect.
It runs first (`groupOrder` 0), then `api-node` and `logic-node`, then
`app-dom`.

## Reused workers

All projects run with `isolate: false`: one worker thread (a process for
`school-date-dom`) runs many test files. `src/test/setup-worker-reuse.ts`, the
first setup file, resets after every file what a fresh worker would not have:
evaluated modules and `vi.mock` registrations, the file's `vi.spyOn` spies and
its mocks in the registry that `vi.clearAllMocks` walks, globals and
prototypes, Node's builtin modules with their exported classes and prototypes
(a patched
`ServerResponse.prototype` kept the previous file's module instances), the DOM
with cookies, URL and viewport, listeners on window/document/html/head/body,
pending Node and window timers, and `process.env`. A property it cannot restore
fails the file. `src/test/reused-threads-pool.ts` bounds each thread's heap and
replaces a thread after a file that leaves more than 200 MB of heap.

- Never rely on state from another file; the file order in a worker is
  arbitrary. A package whose module state must start fresh in every file
  (Testing Library, SWR, next-auth) belongs in `perFileDependencies` in
  `vitest.config.ts`. A non-configurable global that a natively loaded package
  keeps for the worker's lifetime goes into `WORKER_LIFETIME_KEYS` with its
  reason.
- Every test ends on a fresh macrotask (`setup-common.ts`), so work the test
  left pending runs before the next test. Timers are the exception: happy-dom
  sends `hashchange` after `history.replaceState` from a `setTimeout`, and it
  can reach the next test (`portal-shell-scroll.test.tsx` waits for it in
  `beforeEach`). Act only on what the test awaited:
  after `await screen.findBy…(A)` or `await waitFor(() => expect(mock)…)`,
  query a different element B with `findBy…`, and wait for a button to be
  enabled before clicking it.
- Check order independence with
  `pnpm vitest run --sequence.shuffle.files --sequence.seed=<n>`, and with
  `--maxWorkers=1` for the most sharing.
- Transformed modules persist across runs in
  `node_modules/.experimental-vitest-cache` (`experimental.fsModuleCache`),
  keyed by file content, Vite config and lockfile. A Vite plugin whose output
  depends on other files must add them to the cache key; after a suspected
  stale transform, `pnpm vitest --clearCache` empties the cache.

## Scope

Every Vitest test in all projects (`app-dom`, `school-date-dom`, `api-node`, `logic-node`) runs
on a
deterministic clock (#3101). `src/test/setup-common.ts` freezes `Date` at
`TEST_CLOCK_INSTANT` before each test and normally restores the real clock
afterwards. If a file enables fake timers in module scope or `beforeAll`, the
setup preserves that mode while resetting its system time before each test. If
a test releases fake timers, the setup restores them after that test so the
scope's `afterAll` can release them. The file restores real timers in
`afterAll`.
`vitest.config.ts` pins the zone to `Europe/Berlin`, so the frozen instant is
always Wednesday, 9 September 2026, 12:00 Berlin time (`TEST_CLOCK_TODAY` =
`2026-09-09`).

The freeze covers `new Date()` and `Date.now()` everywhere in the worker: the
test body, the rendered component tree, and libraries such as react-day-picker
and date-fns. Mocking a domain hook (`useBerlinToday`, `todayISO`) no longer
leaves a calendar or a relative-time label on the CI host's date.

Only `Date` is frozen. `setTimeout`, `setInterval`, promises and Testing
Library's `waitFor` keep running on real timers; async tests behave as before.

## Use

Helpers live in `src/test/clock.ts` (`~/test/clock`).

| Need | Use | Never |
|---|---|---|
| The default instant or day | `TEST_CLOCK_INSTANT`, `TEST_CLOCK_TODAY` | a literal copied from the setup |
| Another point in time (Berlin midnight, week change, DST switch) | `setTestClock("2026-03-29T01:30:00+01:00")` in the test or `beforeEach` | `vi.useFakeTimers({ toFake: ["Date"] })` boilerplate; the setup already freezes `Date` |
| Control timers (`setInterval`, debounce) | `vi.useFakeTimers()`; it starts at the frozen instant, `vi.advanceTimersByTime` moves clock and timers | assuming fake timers start at the host time |
| Real event loop again mid-test | `releaseFakeTimers()`: timers become real, the clock stays where the fake timers left it | `vi.useRealTimers()` in a test body or `before*` hook |
| Cleanup | nothing; the setup restores the clock and uninstalls fake timers after every test, except file-scoped fake timers, which the file restores in `afterAll`. `afterEach(() => vi.useRealTimers())` stays allowed | `vi.useRealTimers()` inside `try … finally` |
| Wall-clock time on purpose | `useRealClock("reason")` with the reviewed reason as a string literal | `vi.getRealSystemTime()`, `vi.stubGlobal("Date", …)`, `globalThis.Date = …` |

A test that mocks a domain hook to one day while the component tree reads
another day is deterministic but asserts a mismatch. Prefer `setTestClock` and
let the hook read the frozen clock unless the mismatch is the behaviour under
test.

Fixtures built relative to "now" (`Date.now() + 60_000`, `new Date()`) stay
valid: fixture and product code see the same frozen instant. Fixtures that use
`Date.now()` for uniqueness collide within one test; use a counter instead.

## Enforcement

- `src/test/clock.test.tsx` is the regression test of the mechanism: a pinned
  hook supplies the explicit today marker to the real kit `DatePicker`, while
  the independent `Date`/`Date.now()` assertions remain on the frozen instant.
  `setTestClock` covers Berlin midnight, a week change and the DST switch,
  and time and timer state reset between tests.
- oxlint rule `test-clock/no-real-clock`
  (`scripts/oxlint-plugin-test-clock.mjs`, tested in
  `scripts/oxlint-plugin-test-clock.test.ts`) runs in `pnpm run check` and CI.
  It rejects `vi.useRealTimers()` outside `afterEach`/`afterAll`,
  `useRealClock()` without a non-empty literal reason, `vi.getRealSystemTime()`
  and statically identifiable replacement of the global `Date`, including
  direct assignments and `Object.defineProperty` / `Reflect` writes. There is
  no baseline and no allowlist; the exception is the reason inside the
  `useRealClock` call.
- `.claude/rules/no-test-modifications.md` applies: a date-dependent failure is
  fixed by pinning the clock, not by loosening the assertion.

## Absence assertions

`queryBy…` with `not.toBeInTheDocument()`, `toBeNull()`, `toHaveLength(0)` or
`not.toHaveBeenCalled()` also passes against an empty render, a permanent
skeleton or renamed text. Permission and feature gates mostly rest on such
assertions, so each one needs a positive check (#4435):

1. **Anchor in the same render.** Before the absence check, assert an element
   only the loaded content renders: a neighbour entry of the same group, a
   list row, a mocked child of the loaded view. A page title, shell or tab bar
   counts only if the loading state does not render it; check the skeleton
   (for example `LOADING_TABS` in the student record skeleton) before you pick
   one. Each anchor block, one or more consecutive assertions, starts with a
   `// Anker: <what it proves>` line, so `grep "// Anker:"` finds every anchor.
   Take the anchor from the branch (view, role, mode) the gate hangs on. An
   element several views render proves only "loaded": the student record's
   `guardian-manager` renders in both the full and the limited view, so a gate
   of the full view anchors on `care-schedule-manager`, which only the full
   view renders.
2. **Counterpart.** Next to the test for the forbidden role or the switched-off
   feature, a test for the allowed role or switched-on feature finds the same
   element with the same query. Both take the same path (URL, filter, clicks)
   and differ only in the permission or setting. A label no counterpart can
   find is a dead check, usually left behind by a rename.
3. **Components that render nothing when hidden.** Keep the visible
   environments of the positive cases in the table. Each hidden row spreads
   the visible row it is checked against and overrides only what differs. It
   asserts absent, unmounts, resets state the first render changed (visit
   count, storage), renders the visible row and asserts present
   (`src/components/tenant/pwa-install-hint.test.tsx`).
4. **Countercheck once per file.** Temporarily let the component return `null`
   or stay in its loading state and run the absence tests. Each must fail, and
   the failure must come from the anchor assertion itself (its query in the
   error message), not from an unrelated line. Revert and record the result
   in the PR.
5. **`waitFor` waits for a positive state.** A `waitFor` whose callback only
   asserts absence passes on its first run, before asynchronous data arrives.
   Await the loaded content first (`await screen.findBy…`), then assert the
   absence synchronously. Keep `waitFor` for a transition: the element was
   asserted present, an action removes it, `waitFor` waits for it to go.

```tsx
render(<Sidebar />);

// Anker: die Staff-Navigation ist geladen.
expect(screen.getByText("Meine Gruppen")).toBeInTheDocument();
expect(screen.queryByText("Datenverwaltung")).not.toBeInTheDocument();
```

Enforcement: oxlint plugin `scripts/oxlint-plugin-absence-assertion.mjs`
(tested in `scripts/oxlint-plugin-absence-assertion.test.ts`) runs in
`pnpm run check` and CI. `absence-assertion/needs-anchor` rejects a test whose
only DOM assertions are absences; `getBy…`/`findBy…` calls, positive jest-dom
matchers and positive checks of a DOM node or its text count as evidence,
mock-call assertions do not. `absence-assertion/no-early-wait-for` rejects a
`waitFor` with an absence assertion before any positive DOM evidence. Evidence
counts from the test, the `beforeEach`/`beforeAll` hooks of its `describe`
blocks and same-file helpers it calls; imported helpers are opaque. The only
exception is the exact empty-render contract,
`expect(container.firstChild).toBeNull()`,
`expect(container).toBeEmptyDOMElement()` or
`expect(container.innerHTML).toBe("")`, for a component whose hidden state
renders nothing and that has no visible environment worth tabling. Empty text
(`container.textContent` is `""`) stays an absence, and
`not.toBeEmptyDOMElement()` is no anchor: a skeleton passes both. There is no
baseline or allowlist.

## Kit pixel comparison

`pnpm run kit-pixel` proves that a kit refactor changes nothing a user sees
(#6562, epic #6561). It renders every story under `src/components/ui/` at
desktop and phone width (`playwright.kit-pixel.config.ts`) and compares them
pixel for pixel, threshold zero, against a baseline of a chosen commit. Local
only; no CI job runs it.

1. `pnpm run kit-pixel baseline --ref <commit>` (default `HEAD`). Exports the
   commit with `git archive`, so uncommitted edits stay out, builds Storybook
   from it and writes the images to `.kit-pixel/baseline/`. A failed run
   keeps the previous baseline.
2. Change the code.
3. `pnpm run kit-pixel compare` builds Storybook from the working tree. Done
   when it ends with „Keine Abweichung“. It also fails when a story of the
   baseline is gone (deleted or renamed). A deviation leaves expected, actual
   and `*-diff.png` under `.kit-pixel/results/`;
   `pnpm exec playwright show-report .kit-pixel/report` shows them side by side.

Options for both commands: `--filter <regex>` on the story id
(`components-ui-modal--default`; with a leading dash write `--filter=--opened$`),
`--all-stories` to include stories outside `ui/`. `compare` reuses the
baseline's selection unless given its own.

- A refactor ticket adds missing stories for the pieces it touches in its
  first commit, without code changes, and takes the baseline from that commit.
- Open states (menu, popover, picker) are stories with a `play` function that
  opens them; `OverflowMenu` and `ListboxDropdown` `Opened` are the templates.
  A modal story passes `isOpen`. The run waits for `play` to finish and fails
  the story when `play` throws: Storybook itself still reports success then.
- Fixed for stable images: the app fonts (`.storybook/fonts.css`, loaded
  before every render; a baseline from a commit before #6562 renders Arial),
  the Vitest instant `TEST_CLOCK_INSTANT` with running timers, Berlin time,
  reduced motion, no CSS transitions, CSS animations in zero time, CPU
  rendering. A running transition alone changed edge smoothing between runs.
  The build is served from disk via `page.route`: no port, no network.
- Chrome updates itself, and another build renders differently. The baseline
  records the version; `compare` refuses a different one. Take a new baseline
  then.
