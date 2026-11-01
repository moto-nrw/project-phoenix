/**
 * Optimistic actions under the generation lock (#5011).
 *
 * An action shows its expected result at once and confirms it with the
 * server answer. The expected result never enters the SWR cache: it is an
 * overlay on the confirmed data, kept here per scoped key and per data
 * generation. That gives these invariants by construction:
 *
 * - O1 An overlay is visible only under the scoped key (portal, school,
 *   account) and in the data generation it started in. A scope or data
 *   bump of the key hides it at once (generation.ts).
 * - O2 After the generation moved while the request ran, neither the
 *   overlay, nor the server answer, nor a rollback reaches the new scope:
 *   the answer is not committed, the overlay is dropped, and a rollback
 *   writes nothing because the cache was never touched.
 * - O3 A failure drops exactly its own overlay. The view is the confirmed
 *   data with the remaining overlays applied, so it returns to the state
 *   before this action, and concurrent actions keep their own overlays and
 *   commits instead of rolling each other back (SWR's `optimisticData`
 *   restores the value before the first optimistic write, and a newer
 *   mutation swallows an older one's rollback).
 * - O4 The commit is the caller's write through the existing locked paths
 *   (a reader's bound `mutate`, useTenantMutate) or into local state. This
 *   module reads and writes no cache entry; a refusal goes through
 *   useDiscardRefusedEntity. It adds no option to the allowlists.
 * - O8 (G21) `run` belongs to the render it came from. A callback retained
 *   across a scope or data bump of its key (a closure, a retry handler)
 *   starts nothing: no overlay, no request, no commit. Its captured data
 *   never becomes an overlay of the next generation.
 *
 * A 401, 403 or 404 counts as a refusal only with one of the codes the
 * caller lists for its backend route (status and code, ADR 0006); there is
 * no default. A refusal drops every representation of the resource the
 * caller names. Other failures, a business 403, a session-level code or a
 * failure without a response, only roll back.
 */

"use client";

import {
  useCallback,
  useLayoutEffect,
  useMemo,
  useRef,
  useSyncExternalStore,
} from "react";
import type { ErrorCode } from "~/lib/error-codes.generated";
import {
  currentSWRDataGeneration,
  dataGenerationOf,
  subscribeSWRDataGeneration,
} from "./generation";
import { useDiscardRefusedEntity, useSWRCacheKey } from "./hooks";
import { isSWRSessionPending } from "./scope";

/** `pending`: the request runs. `failed`: it failed and the caller kept it. */
export type OptimisticState = "pending" | "failed";

/** What an overlay knows about itself while it is applied. */
export interface OptimisticEntry {
  /** Unique id of this action, e.g. for the temporary list row it adds. */
  readonly id: string;
  readonly state: OptimisticState;
}

export interface OptimisticRun<T, R> {
  /** The expected result on top of the confirmed data. Pure. */
  readonly optimistic: (data: T, entry: OptimisticEntry) => T;
  /** The write to the server. */
  readonly request: () => Promise<R>;
  /**
   * Takes the server answer into the confirmed data, through a locked write
   * (the reader's bound `mutate`, useTenantMutate) or local state. Runs
   * only while the action's generation and scope are still current, in the
   * same task in which the overlay goes, so no frame shows both or neither.
   */
  readonly commit: (result: R) => void;
  /**
   * Keep a failed overlay, marked `failed`, until `dismiss` or the next
   * generation. Off: a failure only rolls back.
   */
  readonly keepFailed?: boolean;
  /** A read can confirm this write before, or instead of, its response. */
  readonly confirmed?: (data: T) => boolean;
  /**
   * The resource behind the action. A 401/403/404 with one of `codes` (the
   * refusals of the action's backend route) drops every cached key `matches`
   * names (unscoped, as the factories build it); the refusal stays visible.
   */
  readonly refusal?: {
    readonly matches: (key: string) => boolean;
    readonly codes: readonly ErrorCode[];
  };
}

export type OptimisticOutcome<R> =
  | { readonly status: "done"; readonly result: R }
  | {
      readonly status: "failed";
      readonly error: unknown;
      /** The server refused the resource; its representations are gone. */
      readonly refused: boolean;
    }
  /** The generation or scope moved while the request ran: nothing happened. */
  | { readonly status: "discarded" };

interface Overlay {
  readonly id: string;
  readonly owner: object;
  /** Data generation of the key when the action started. */
  readonly generation: number;
  readonly state: OptimisticState;
  readonly apply: (data: unknown, entry: OptimisticEntry) => unknown;
  readonly confirmed?: (data: unknown) => boolean;
}

const NONE: readonly Overlay[] = [];
// Overlays per full, scoped key. Arrays are replaced, never changed, so
// useSyncExternalStore sees each change.
const overlays = new Map<string, readonly Overlay[]>();
const listeners = new Set<() => void>();
let sequence = 0;

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function update(
  key: string,
  change: (list: readonly Overlay[]) => readonly Overlay[],
): void {
  const next = change(overlays.get(key) ?? NONE);
  if (next.length === 0) overlays.delete(key);
  else overlays.set(key, next);
  for (const listener of listeners) listener();
}

function isCurrent(key: string, overlay: Overlay): boolean {
  return overlay.generation === dataGenerationOf(key);
}

// An overlay of an ended generation is never shown again (O1); free it.
subscribeSWRDataGeneration(() => {
  for (const [key, list] of overlays) {
    if (list.some((overlay) => !isCurrent(key, overlay))) {
      update(key, (current) =>
        current.filter((overlay) => isCurrent(key, overlay)),
      );
    }
  }
});

/**
 * Optimistic actions on the data of the factory key `key`.
 *
 * `confirmed` is that key's confirmed data: the `data` of its useSWRAuth
 * reader, or local state the view loaded for the same resource. `data` is
 * `confirmed` with the current overlays applied; `stateOf(id)` says whether
 * an entry an overlay added is still on its way or failed.
 *
 * @example
 * ```tsx
 * const { data: confirmed, mutate } = useSWRAuth(key, fetcher);
 * const action = useOptimisticAction(key, confirmed);
 * const outcome = await action.run({
 *   optimistic: (thread, { id }) => withMessage(thread, id, body),
 *   request: () => postMessage(threadId, body),
 *   commit: (sent) => void mutate((t) => withSent(t, sent), { revalidate: true }),
 *   keepFailed: true,
 *   refusal: { codes: TEAM_CHAT_SEND_REFUSALS, matches: (k) => k === key },
 * });
 * ```
 */
export function useOptimisticAction<T>(
  key: string | null,
  confirmed: T | undefined,
) {
  const scoped = useSWRCacheKey();
  const fullKey = key === null ? null : scoped(key);
  const scope = scoped("");
  const discard = useDiscardRefusedEntity();
  const owner = useRef<object>({}).current;
  const lifetime = useRef<{ active: boolean } | null>(null);
  // The scope of the latest render: an answer of an earlier scope is not
  // committed, also when no generation moved for its key.
  const latestScope = useRef(scope);
  useLayoutEffect(() => {
    latestScope.current = scope;
  }, [scope]);

  // Renders again on every scope or data bump, which hides overlays (O1).
  useSyncExternalStore(
    subscribeSWRDataGeneration,
    currentSWRDataGeneration,
    currentSWRDataGeneration,
  );
  const generation = fullKey === null ? null : dataGenerationOf(fullKey);
  // The generation `run` of this render may act in (O8).
  const origin = generation;
  const stored = useSyncExternalStore(
    subscribe,
    () => (fullKey === null ? NONE : (overlays.get(fullKey) ?? NONE)),
    () => NONE,
  );
  const visible = useMemo(
    () =>
      generation === null
        ? NONE
        : stored.filter(
            (overlay) =>
              overlay.generation === generation &&
              !(confirmed !== undefined && overlay.confirmed?.(confirmed)),
          ),
    [generation, stored, confirmed],
  );

  const latestConfirmed = useRef(confirmed);
  useLayoutEffect(() => {
    latestConfirmed.current = confirmed;
    if (fullKey === null || confirmed === undefined) return;
    const list = overlays.get(fullKey) ?? NONE;
    if (list.some((entry) => entry.confirmed?.(confirmed))) {
      update(fullKey, (current) =>
        current.filter((entry) => !entry.confirmed?.(confirmed)),
      );
    }
  }, [fullKey, confirmed]);

  const data = useMemo(
    () =>
      confirmed === undefined
        ? undefined
        : (visible.reduce<unknown>(
            (current, overlay) =>
              overlay.apply(current, { id: overlay.id, state: overlay.state }),
            confirmed,
          ) as T),
    [confirmed, visible],
  );

  // Each visit has its own lifetime, even when the same hook changes keys
  // and comes back. Pending successes may still commit after leaving; late
  // failures must not become undismissable overlays in a later visit.
  useLayoutEffect(() => {
    const view = { active: true };
    lifetime.current = view;
    return () => {
      view.active = false;
      if (fullKey === null) return;
      update(fullKey, (list) =>
        list.filter(
          (overlay) => overlay.owner !== owner || overlay.state !== "failed",
        ),
      );
    };
  }, [fullKey, owner]);

  const stateOf = useCallback(
    (id: string): OptimisticState | undefined =>
      visible.find((overlay) => overlay.id === id)?.state,
    [visible],
  );

  /** Drops the failed overlays this view kept. */
  const dismiss = useCallback(() => {
    if (fullKey === null) return;
    update(fullKey, (list) =>
      list.filter(
        (overlay) => overlay.owner !== owner || overlay.state !== "failed",
      ),
    );
  }, [fullKey, owner]);

  const run = useCallback(
    async <R>(action: OptimisticRun<T, R>): Promise<OptimisticOutcome<R>> => {
      // Without a session there is no scope to show or confirm anything in.
      // A callback kept from before a bump of its key starts nothing (O8):
      // its caller's data belongs to the generation that ended.
      if (
        fullKey === null ||
        scope === null ||
        origin === null ||
        isSWRSessionPending() ||
        latestScope.current !== scope ||
        dataGenerationOf(fullKey) !== origin
      ) {
        return { status: "discarded" };
      }
      const view = lifetime.current;
      const target = fullKey;
      const generation = origin;
      sequence += 1;
      const id = `optimistic-${sequence}`;
      const overlay: Overlay = {
        id,
        owner,
        generation,
        state: "pending",
        apply: action.optimistic as Overlay["apply"],
        confirmed: action.confirmed as Overlay["confirmed"],
      };
      update(target, (list) => [...list, overlay]);
      const drop = () =>
        update(target, (list) => list.filter((entry) => entry.id !== id));
      const stillCurrent = () =>
        !isSWRSessionPending() &&
        dataGenerationOf(target) === generation &&
        latestScope.current === scope;

      let result: R;
      try {
        result = await action.request();
      } catch (error) {
        if (
          !stillCurrent() ||
          (latestConfirmed.current !== undefined &&
            action.confirmed?.(latestConfirmed.current))
        ) {
          drop();
          return { status: "discarded" };
        }
        const refused =
          action.refusal !== undefined &&
          discard(error, {
            codes: action.refusal.codes,
            matches: action.refusal.matches,
          });
        if (!view?.active) {
          drop();
          return { status: "discarded" };
        }
        if (action.keepFailed === true && !refused) {
          update(target, (list) =>
            list.map((entry) =>
              entry.id === id ? { ...entry, state: "failed" } : entry,
            ),
          );
        } else {
          drop();
        }
        return { status: "failed", error, refused };
      }
      if (!stillCurrent()) {
        drop();
        return { status: "discarded" };
      }
      // Commit first, then drop the overlay in the same task: React renders
      // the confirmed answer without the overlay in one pass.
      action.commit(result);
      drop();
      return { status: "done", result };
    },
    [fullKey, scope, origin, owner, discard],
  );

  const pending = visible.some((overlay) => overlay.state === "pending");

  return { data, run, stateOf, dismiss, pending };
}

/** One logical send keeps its id after an unknown outcome, within its SWR scope.
 * A changed draft is a new intent; an acknowledged send ends the attempt.
 */
export function useMessageSendAttempt(key: string | null) {
  const scoped = useSWRCacheKey();
  const fullKey = key === null ? null : scoped(key);
  const current = useRef<{
    scope: string | null;
    body: string;
    id: string;
  } | null>(null);
  const begin = useCallback(
    (body: string) => {
      if (current.current?.scope !== fullKey || current.current.body !== body) {
        current.current = { scope: fullKey, body, id: crypto.randomUUID() };
      }
      return current.current.id;
    },
    [fullKey],
  );
  const complete = useCallback((id: string) => {
    if (current.current?.id === id) current.current = null;
  }, []);
  return { begin, complete };
}
