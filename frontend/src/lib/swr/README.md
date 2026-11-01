# Resource keys

Use the existing authenticated hooks. Each resource family owns its key,
parameters and fetcher; pages do not give the same response a new name.

```tsx
import { useSWRAuth, useSWRPreload, useInvalidateFamily } from "~/lib/swr";
import { rooms } from "~/lib/swr/rooms";

const params = { page: 1, pageSize: 500 };
const response = useSWRAuth(rooms.key(params), () => rooms.fetcher(params));
const preload = useSWRPreload();
const invalidateFamily = useInvalidateFamily();

// On intent, with the same parameters as the destination:
// The destination owns load errors; intent must not show an error toast.
void preload(rooms.key(params), () => rooms.fetcher(params)).catch(
  () => undefined,
);
// After a write, refresh every parameter set, including preload-only keys:
void invalidateFamily(rooms.family);
```

1. Factory keys are **unscoped**. The hooks add the authenticated portal,
   school and account through `scope.ts`. Never give an unscoped key to the
   global cache or a package-level preload. Without session access there is
   no cache key, fetch, preload or mutation. Discarded preloads resolve to
   `undefined`; failures of the current generation still reject.
2. Use `useSWRCacheKey()` to resolve the same identity for a list-overlay ID.
   Seed through the helpers of `seed.ts`: `useSWRSeedFrom()` derives a detail
   seed from current cached data and keeps its source's generation stamp;
   `stampSWRSeeds` and `writeSWRSeeds` carry a server snapshot. Do not restamp
   old data as a fresh fallback. A partial seed must still revalidate;
   `revalidateIfStale: false` would suppress the request in SWR 2.5.1.
3. The callback returned by `useTenantMutate()` supports optimistic
   data, rollback and `populateCache` under the generation lock. Prefer the
   reader's bound `mutate` when it already owns that key. For an action the
   user should see at once (send, check in, toggle), use
   `useOptimisticAction(key, confirmed)` from `~/lib/swr/optimistic`: the
   expected result is an overlay on the confirmed data per scoped key and
   data generation, never a cache write. A failure drops exactly its own
   overlay, concurrent actions keep theirs, and after a scope or data bump
   neither overlay, answer nor rollback appears. A `run` kept from before a
   bump of its key (a closure, a retry handler) starts nothing. `commit`
   writes the answer through the reader's bound `mutate` (or local state);
   `refusal` drops the resource on a 401/403/404 with one of the codes the
   caller lists for its backend route (no default; message routes:
   `~/lib/message-send-refusals`). A business 403, a session code or a
   failure without a response only rolls back. SWR's own `optimisticData` restores the
   value before the first optimistic write and lets a newer mutation swallow
   an older one's rollback, so concurrent actions would overwrite each other.
4. Family prefixes end in `-`. `useInvalidateFamily` matches that prefix
   within the current scope, including `$inf$` keys and preloads. The existing
   `useTenantMutateMatching([factory.family])` remains available for combined
   invalidations, `useInvalidateMatching()` for a predicate over the unscoped
   key (`() => true` refreshes the whole session). Session boundaries also
   clear all list overlays.
5. Keys describe the **response**, not its screen. Different parameters or
   projections stay separate: system rooms, paginated room lists, the staff
   directory and its teacher projection must not overwrite one another.
   Filter/sort display transformations belong outside the shared cache.
6. Outside this directory there is no unlocked cache access. Only the
   public modules are reachable (`index`, `hooks`, `mutate`,
   `config-boundaries`, `scope-boundary`, the key and factory modules), and
   none of their exports hands out the cache, a provider or a raw mutate:
   readers and writers pass the generation lock, `mutate` only revalidates
   or clears and keeps only `revalidate` of its options, and
   `SWRSeedBoundary` / `IsolatedSWRCache` take data or nothing.
   `swr/no-cache-access` judges every module reference by its resolved
   target and every imported name by the module's actual exports;
   `cache-boundary.test.tsx` holds the reviewed export list. Tests reach
   SWR's provider through `testing.ts`; `swr/no-test-support-import` keeps
   `src/test/**` out of production code.
7. Options are an allowlist in type and at runtime: readers take
   `ReaderOptions` (`READER_OPTION_KEYS`), mutations `MUTATION_OPTION_KEYS`,
   `SWRSeedBoundary` only `fallback` and `cacheData`. The hooks copy the
   allowed keys and throw on any other outside production; `use`,
   `provider`, `fallback`, `fetcher`, `compare` and `onErrorRetry` never
   reach SWR. `onSuccess` and `onError` get the value or failure alone.

Migrated direct readers explicitly retain SWR’s former `keepPreviousData: false`
default unless they already opted in. Do not carry a previous child, thread or
audience into a newly selected key.

The pure `*-keys.ts` modules keep API clients out of always-mounted invalidators.
`portal-keys.ts` contains the smaller portal resource families. There is no
runtime registry. Direct package imports outside this boundary and new free
hook keys fail oxlint; both legacy baselines are shrink-only. A free key is
anything at a `useSWRAuth`, `useImmutableSWR` or `useSWRWithId` call that is
not built by an export of `~/lib/swr` (directly, through a local constant or
in a branch with `null`).

## List-to-detail previews

`studentKeys`, `roomKeys` and `staffKeys` own `previewSources` and `preview`.
The projection returns only `{ id, name, context }`, never a full record or
access flags. A detail page calls `useSWRSeedFrom()(factory.previewSources,
factory.detailKey(id), data => factory.preview(data, id))` (children use
`detail`, staff use the active `recordKey` or limited `profileKey`). The matcher runs only over the current
school/account scope and skips old generations, including lists older than a
discarded detail key. The helper stamps its derived value but does not write
it into the SWR cache. Pages use it only for the loading header, not as the
complete detail response.

The pages memoize this projection for a loading attempt. The helper callback
changes on scope or data-generation revisions, so a memoized preview is
withdrawn when its source or target is discarded. The old list is checked
before stamping; a target newer than the source cannot be reseeded (G16/G21).
Unrelated renders do not scan the cache or call `stampSWRValue` again.

Show the preview only in the existing loading header. The body, actions,
permission flags and detail-only fields wait for the detail request; that
request always runs. Detail readers use `keepPreviousData: false`, so changing
the entity cannot reuse the previous entity's complete record. Without a
matching list the existing loading state remains unchanged.

Message previews use `messageKeys.teamChatPreview` / `groupPreview`; staff
announcements use `parentAnnouncementKeys.preview`. Only the list-visible
name/title enters the loading header. Last-message text, unread parts, group
members, recipients, private replies and permission flags never enter it.
These sources do not match any parents-portal resource: relationship-filtered
parent conversations continue to wait for their authorized detail response.
For a list-known published/expired poll, type and status enable only the
scoped evaluation request in parallel with the detail. The confirmed detail
alone renders the evaluation and its actions; the same evaluation key is
shared with the panel, so mounting it does not repeat the request.

## Parents portal

`useParentChildren`, `useParentFeatures` and `useParentToday` share the parent
resources between the shell, start page and child page. Features and today's
status use the sorted `(tenant_id, student_id)` relationship set in their
factory key, together with the Berlin calendar day for date-dependent flags
and status. Each relationship is fetched, including siblings at the same
school; a school's flags never stand in for another child's permissions.
A non-refusal background failure preserves that child's latest confirmed
features and status; successful siblings use their fresh answers. Alternating
partial failures retain successful per-child updates in the current batch error,
not only the last wholly successful batch. At request start, `readCurrent()` on
the authenticated reader supplies generation-checked data/error snapshots.
This capability exposes no cache and immediately returns empty values after a
scope/data drop or session suspension, including before a React rerender.
It is optional in the structural response type for compatibility with standard
SWR-shaped consumers; real authenticated readers always implement it. Without a confirmed value, failed feature lookups stay off and failed
status lookups use `UNKNOWN_CHILD_TODAY`. Errors remain available for logging
and cold-load error presentation. Partial results are read only through the
wrapper's current-error gate (G19). No separate fallback cache is retained:
access refusal, logout, account or relationship changes discard confirmed and
partial values together. The private batch failure is not an `ApiError`, so
refusals cannot restamp its partial results into a newer generation.
The authenticated wrapper adds `parent:-:<account>`: the parents JWT has no
school, and account-wide lists can span schools. They must not use a tenant,
school or operator session. News, message lists and the start-page calendar
also have one factory key per response.

Batch readers expose `hasConfirmedData` for a complete, successful response;
partial error values are not a confirmed initial response. Pages keep loaded
content and dialogs mounted after background failures and log those failures.
Use `useSwrResourceLoadError` from `~/lib/hooks/use-swr-load-error` for this
shared presentation rule: it accepts SWR resources, blocks only on errors without
confirmed data, and logs background failures. Empty lists are confirmed data.
Pass batch readers with their `hasConfirmedData` flag so partial failure payloads
do not hide an initial load error. Access/scope discards remain the resource
boundary’s responsibility; the presenter never retains discarded data.
The load-error path remains for resources that have never loaded successfully.

Warm mounts use confirmed data immediately. Revalidating readers refresh an
unmounted resource after an SSE invalidation; only the session-wide children
list and profile skip mount revalidation. Requests deduplicate for 60 seconds. Focus recovery is throttled to
60 seconds; SSE events revalidate the shared resources. Profile, children and
the start-page feeds start independently, then features and day status start
in parallel as soon as children are known. The child header still reads the
complete authorized children list, not a new detail preview or permission seed.

The parent API reports `care.child_not_linked` (401/403/404) to the
authenticated bridge. `care.guardian_permission_denied` triggers eviction only
for the features and today readers, which check `parent_portal.access` alone.
Action-specific refusals, including document inbox reads and writes, do not
prove loss of child access. Reads keep their load errors; write actions roll
back and recheck their still-readable resource (as in #5425).
It discards the parent resource family through `useDiscardRefusedEntity`,
revalidates the children list, and broadcasts only account and refusal code
to the other tabs. The API captures the originating account before the request;
a late refusal cannot evict another account. Business refusals do not evict.
A changed relationship set also drops derived resources, so removing and later
re-adding a relationship cannot resurrect its former flags. The shell remounts
remaining local-state sections at account/scope and relationship boundaries.
No cache internals, persistent storage or new lint exceptions are involved.

## Operator portal

The backend session scope `platform` maps to `operator:{accountId}`. Operators
read across schools, so there is no school in that session prefix. A school or
organization ID belongs to the resource factory key, not to the session scope.
All-school summaries stay under `schoolSummaries()`; consumers filter after
reading and never put a cross-school response under a school-specific key.

Use URL `schoolId` for list requests, independently of filter-option loading.
The organization detail uses `organizationSchoolSummaries(id)` and its audited
organization-specific endpoint, never the all-school response. The school detail
loads organization and all-school summaries in parallel; only its ID-dependent
requests wait for that resolution. Summary invalidation covers both all-school
and organization-filtered responses. Account/device writes use their factory
families; person deletion/merging uses `personChanges` to refresh person lists
and every school/organization counter.
Announcement statistics and confirmation details run only while expanded.
The operator auth guard also keys local page/modal state to its confirmed scope
and hides it while another tab's session change is being confirmed.

## Navigation intent

Navigation data preloads must use `useSWRPreload` with the destination's exact
factory key and fetcher. Code prefetch and data prefetch are independent:
data waits for about 300 ms of mouse intent, keyboard focus or touch start.
Do not preload audited reads, inaccessible resources, or speculative detail
fields. A failed speculative request must not become the destination's load
error. Scope changes discard pending answers, including work queued before
the change. Respect Save-Data and slow connections and bound concurrency.

`preload(key, fetcher, { intent: true })` is the speculative variant of the
same locked writer. It skips an already current cached response and removes
only its own failed preload promise, so a later page runs its ordinary loader.
It never mutates response data directly, bypasses a generation check, or
removes a replacement promise. General preloads keep their existing contract.
The protected tenant shell shares thin fetchers, not page hooks, so data
intent does not wait for another code chunk. It starts data only after intent. Its queue runs at most three resources concurrently
and holds at most twelve waiting resources; account/school/generation changes
invalidate the captured writer even while code is loading or work is queued.
