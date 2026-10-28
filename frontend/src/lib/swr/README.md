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
void preload(rooms.key(params), () => rooms.fetcher(params));
// After a write, refresh every parameter set, including preload-only keys:
void invalidateFamily(rooms.family);
```

1. Factory keys are **unscoped**. The hooks add the authenticated portal,
   school and account through `scope.ts`. Never give an unscoped key to the
   global cache or a package-level preload. Without session access there is
   no cache key, fetch, preload or mutation.
2. Use `useSWRCacheKey()` to resolve the same identity for a cache read or
   list-overlay ID. A cache-derived detail seed must retain its source's
   generation stamp. Do not restamp old data as a fresh fallback. A partial
   seed must still revalidate; `revalidateIfStale: false` would suppress the
   request in SWR 2.5.1.
3. The callback returned by `useTenantMutate()` supports optimistic
   data, rollback and `populateCache` under the generation lock. Prefer the
   reader's bound `mutate` when it already owns that key.
4. Family prefixes end in `-`. `useInvalidateFamily` matches that prefix
   within the current scope, including `$inf$` keys and preloads. The existing
   `useTenantMutateMatching([factory.family])` remains available for combined
   invalidations. Session boundaries also clear all list overlays.
5. Keys describe the **response**, not its screen. Different parameters or
   projections stay separate: system rooms, paginated room lists, the staff
   directory and its teacher projection must not overwrite one another.
   Filter/sort display transformations belong outside the shared cache.

The pure `*-keys.ts` modules keep API clients out of always-mounted invalidators.
`portal-keys.ts` contains the smaller portal resource families. There is no
runtime registry. Direct package imports outside this boundary and new free
hook-key literals fail oxlint; both legacy baselines are shrink-only.
