"use client";

import { useEffect, useMemo, useRef, type ReactNode } from "react";
import { useSWRPreload } from "~/lib/swr";
import {
  DataPrefetchContext,
  type PrefetchResource,
} from "~/components/ui/data-prefetch-intent";

function mayPrefetch(): boolean {
  const connection = (
    navigator as Navigator & {
      connection?: { saveData?: boolean; effectiveType?: string };
    }
  ).connection;
  return (
    !connection?.saveData &&
    connection?.effectiveType !== "slow-2g" &&
    connection?.effectiveType !== "2g" &&
    connection?.effectiveType !== "3g"
  );
}

/** Only the locked SWR writer sees data; this queue owns scheduling, not a cache. */
export function ResourcePrefetchProvider({
  children,
  resourcesFor,
}: Readonly<{
  children: ReactNode;
  resourcesFor: (href: string) => Promise<readonly PrefetchResource[]>;
}>) {
  const preload = useSWRPreload();
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  // Shared across scope changes too: old network work still occupies a slot.
  const scheduler = useRef({ active: 0, queue: [] as (() => Promise<void>)[] });
  const intent = useMemo(() => {
    const pending = new Set<string>();
    // Resolve code/descriptors alongside the route at 80 ms, not after the
    // 300-ms data timer. Contains no responses and starts no data requests.
    const prepared = new Map<string, Promise<readonly PrefetchResource[]>>();
    const resolve = (href: string) => {
      let result = prepared.get(href);
      if (!result) {
        result = resourcesFor(href);
        if (prepared.size >= 12) prepared.delete(prepared.keys().next().value!);
        prepared.set(href, result);
      }
      return result;
    };
    const drain = () => {
      const state = scheduler.current;
      while (state.active < 3 && state.queue.length > 0) {
        const task = state.queue.shift()!;
        state.active++;
        void task().finally(() => {
          state.active--;
          drain();
        });
      }
    };
    const run = (href: string, explicit?: readonly PrefetchResource[]) => {
      if (!mayPrefetch()) return;
      // Capture the locked callback BEFORE importing resources. A scope change
      // while the chunk loads or the queue waits cannot rebind old intent.
      void (explicit ? Promise.resolve(explicit) : resolve(href))
        .then((resources) => {
          if (!mounted.current) return;
          for (const resource of resources) {
            if (
              pending.has(resource.key) ||
              scheduler.current.queue.length >= 12
            )
              continue;
            pending.add(resource.key);
            scheduler.current.queue.push(async () => {
              try {
                if (mounted.current && mayPrefetch())
                  await preload(resource.key, resource.fetcher, {
                    intent: true,
                  });
              } catch {
                // Speculation has no UI error path. The page performs its normal load.
              } finally {
                pending.delete(resource.key);
              }
            });
          }
          drain();
        })
        .catch(() => undefined);
    };
    return Object.assign(run, {
      prepare: (href: string) => {
        if (mayPrefetch()) void resolve(href).catch(() => undefined);
      },
    });
  }, [preload, resourcesFor]);
  return <DataPrefetchContext value={intent}>{children}</DataPrefetchContext>;
}
