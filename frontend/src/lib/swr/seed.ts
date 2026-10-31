/**
 * Seeds (#5006, #5007): values the server or a page built for a key before
 * its reader loaded it. A seed belongs to the data generation it was built
 * in: after a drop of its key no reader shows it, also not as fallback. App
 * code seeds only through these helpers, never through the generation
 * internals.
 */

"use client";

// swr/no-direct-import permits package imports inside this shared wrapper only.
import { mutate as swrMutate, useSWRConfig } from "swr";
import { useCallback, useSyncExternalStore } from "react";
import {
  currentSWRDataGeneration,
  subscribeSWRDataGeneration,
  dataGenerationOf,
  isCurrentSWRValue,
  lockMutation,
  stampSWRValue,
} from "./generation";
import { useSWRCacheKey } from "./hooks";

/**
 * Stamps server seed entries, keyed by their full scoped key, with the
 * current data generation of that key.
 */
export function stampSWRSeeds(entries: Readonly<Record<string, unknown>>) {
  for (const [key, value] of Object.entries(entries)) {
    stampSWRValue(key, value, dataGenerationOf(key));
  }
}

/**
 * Writes server seed entries, keyed by their full scoped key, into the cache
 * under the generation lock without revalidating. An entry of the current
 * generation a reader already holds stays.
 */
export function writeSWRSeeds(entries: Readonly<Record<string, unknown>>) {
  for (const [key, data] of Object.entries(entries)) {
    const [locked, options] = lockMutation(
      key,
      (current: unknown) => current ?? data,
      { revalidate: false },
    );
    void swrMutate(key, locked, options as never);
  }
}

/**
 * Seeds the factory key `target` from the cached data of the factory key
 * `source` (or the first matching list), both in the session's scope.
 * A matcher receives only unscoped resource keys. `derive` sees only data of the
 * source's current generation; its result carries that generation, so a
 * later drop of the source hides the seed too. Returns the seed, or
 * undefined without a session, current source data or derived value.
 */
export function useSWRSeedFrom() {
  const scoped = useSWRCacheKey();
  const { cache } = useSWRConfig();
  // Memoized callers must withdraw their seed on a source/target/scope drop,
  // even if their entity ID and source matcher have not changed (G16/G21).
  const revision = useSyncExternalStore(
    subscribeSWRDataGeneration,
    currentSWRDataGeneration,
    currentSWRDataGeneration,
  );
  return useCallback(
    <S, T>(
      source: string | ((key: string) => boolean),
      target: string,
      derive: (data: S) => T | undefined,
    ): T | undefined => {
      if (revision !== currentSWRDataGeneration()) return undefined;
      const prefix = scoped("");
      const targetKey = scoped(target);
      if (prefix === null || targetKey === null) return undefined;
      const candidates =
        typeof source === "string"
          ? [scoped(source)!]
          : [...cache.keys()].filter(
              (key) =>
                key.startsWith(prefix) && source(key.slice(prefix.length)),
            );
      for (const sourceKey of candidates) {
        const data = (cache.get(sourceKey) as { data?: S } | undefined)?.data;
        if (data === undefined || isCurrentSWRValue(sourceKey, data) !== true)
          continue;
        // A discarded target must not be resurrected from an older list.
        if (dataGenerationOf(sourceKey) < dataGenerationOf(targetKey)) continue;
        const seeded = derive(data);
        if (seeded !== undefined) {
          stampSWRValue(targetKey, seeded, dataGenerationOf(sourceKey));
          return seeded;
        }
      }
      return undefined;
    },
    [scoped, cache, revision],
  );
}
