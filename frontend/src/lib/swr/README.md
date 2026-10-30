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
   reader's bound `mutate` when it already owns that key.
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
   SWR's provider through `testing.ts`.

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
