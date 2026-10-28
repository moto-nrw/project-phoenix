/**
 * Cache scope of ~/lib/swr (#5006): which session a cache entry belongs to,
 * and how the cache is dropped at the boundary of that session.
 *
 * Two values describe a session:
 * - the scope prefix (portal, school, account) starts every key the wrappers
 *   build, so no entry of one account is ever read under another account's
 *   key, even by a late response or a stale closure;
 * - the auth generation (scope, school or Träger, account, preview target,
 *   roles and permissions) marks every change after which no entry may stay:
 *   login, logout, account, school, preview or rights change. SWRScopeBoundary
 *   resets the cache whenever it changes.
 *
 * The browser session has no session ID of its own (#5005); the generation
 * stands in for it. A new sign-in of the same account with the same rights
 * keeps the cache: its entries show exactly what the new session may see.
 */

import type { Session } from "next-auth";
import { useSyncExternalStore } from "react";
// swr/no-direct-import permits package imports inside this shared wrapper only.
import { unload, type Cache } from "swr";
import { SWRGlobalState, createCacheHelper, getTimestamp } from "swr/_internal";
import { ApiError } from "~/lib/api-error";
import type { ErrorCode } from "~/lib/error-codes.generated";
import { clearListOverlays } from "~/lib/list-overlays";
import { hasSessionAccess } from "~/lib/session-access";

type MaybeSession = Session | null | undefined;

const generations = new WeakMap<Session, string>();

/**
 * The auth generation of a session; null while it grants no access. Built
 * once per session object, since every SWR hook asks on every render.
 */
export function authGeneration(session: MaybeSession): string | null {
  if (!hasSessionAccess(session)) return null;
  const known = generations.get(session);
  if (known !== undefined) return known;
  const user = session.user;
  const generation = JSON.stringify([
    user.scope ?? "",
    user.tenantId ?? null,
    user.orgId ?? null,
    user.id,
    user.previewTargetAccountId ?? null,
    user.roles ?? [],
    user.permissions ?? [],
  ]);
  generations.set(session, generation);
  return generation;
}

/** Scope prefix of the tenant portal: the school's slug and the account. */
export function tenantSWRScope(tenantSlug: string, accountId: string): string {
  return `${tenantSlug}:${accountId}`;
}

/**
 * The prefix of every key the wrappers build for this session; null while
 * the session grants no access, so nothing is fetched. Inside a
 * TenantProvider the school is the URL's slug. The school portal has no
 * TenantProvider and is bound to the school of its session.
 */
export function swrScopePrefix(
  session: MaybeSession,
  tenantSlug: string | null,
): string | null {
  if (!hasSessionAccess(session) || !session.user.id) return null;
  const { id, scope, tenantId, orgId } = session.user;
  if (tenantSlug) return tenantSWRScope(tenantSlug, id);
  if (scope === "school") {
    return tenantId == null ? null : `school:${tenantId}:${id}`;
  }
  return `${scope || "tenant"}:${tenantId ?? orgId ?? "-"}:${id}`;
}

/** A key of the scope `prefix`. */
export function scopedSWRKey(prefix: string, key: string): string {
  return `${prefix}:${key}`;
}

// Counts the resets and discards of this tab. A hook keeps the data of an
// earlier key on screen (keepPreviousData) only while the count is the one
// it had when that data arrived (see useScopedSWR).
let epoch = 0;

/** How many times this tab dropped cache entries. */
export function swrScopeEpoch(): number {
  return epoch;
}

// Cross-tab invalidation must take effect before React commits new keys or
// SessionProvider answers. The revision also permanently withdraws shell seeds.
const initialRefresh = { pending: false, revision: 0 };
let sessionRefresh = initialRefresh;
const refreshListeners = new Set<() => void>();

function subscribeRefresh(listener: () => void): () => void {
  refreshListeners.add(listener);
  return () => {
    refreshListeners.delete(listener);
  };
}

export function useSWRSessionRefresh() {
  return useSyncExternalStore(
    subscribeRefresh,
    () => sessionRefresh,
    () => initialRefresh,
  );
}

export function isSWRSessionPending(): boolean {
  return sessionRefresh.pending;
}

export function suspendSWRSession(): void {
  sessionRefresh = { pending: true, revision: sessionRefresh.revision + 1 };
  for (const listener of refreshListeners) listener();
}

export function resumeSWRSession(): void {
  if (!sessionRefresh.pending) return;
  sessionRefresh = { ...sessionRefresh, pending: false };
  for (const listener of refreshListeners) listener();
}

/**
 * Drops the whole cache of this tab: every entry including the
 * `useSWRInfinite` and `useSWRSubscription` keys, preloads, requests and
 * mutations in flight (their answers are discarded), the data
 * `keepPreviousData` holds on screen, and the list overlays (#5124).
 *
 * `revalidate` lets mounted hooks of an unchanged key load again; pass it
 * only when the session already in place is the one they should load for.
 */
export function resetSWRScope({ revalidate }: { revalidate: boolean }): void {
  epoch += 1;
  clearListOverlays();
  unload({ revalidate });
}

const REFUSAL_STATUSES = new Set([401, 403, 404]);
const SPECIAL_KEY = /^\$(inf|sub)\$/;

/**
 * Whether `error` is the server's refusal of an entity with one of `codes`.
 * Programs branch on the code (ADR 0006), never on a message.
 */
export function isEntityRefusal(
  error: unknown,
  codes: readonly ErrorCode[],
): boolean {
  return (
    error instanceof ApiError &&
    error.status !== undefined &&
    REFUSAL_STATUSES.has(error.status) &&
    error.code !== undefined &&
    codes.includes(error.code)
  );
}

/**
 * Drops the entries of scope `prefix` whose unscoped key `matches`: their
 * data, the data kept on screen, preloads and requests and mutations in
 * flight. An error an entry carries stays, so the view shows the refusal
 * instead of the data. Nothing is fetched again. Returns how many entries
 * were dropped.
 */
export function discardScopedEntries(
  cache: Cache,
  prefix: string,
  matches: (key: string) => boolean,
): number {
  const state = SWRGlobalState.get(cache);
  if (!state) return 0;
  const [, MUTATION, FETCH, PRELOAD] = state;
  const scope = `${prefix}:`;
  const dropped: string[] = [];
  for (const key of cache.keys()) {
    const plain = key.replace(SPECIAL_KEY, "");
    if (plain.startsWith(scope) && matches(plain.slice(scope.length))) {
      dropped.push(key);
    }
  }
  if (dropped.length === 0) return 0;
  epoch += 1;
  for (const key of dropped) {
    delete FETCH[key];
    delete PRELOAD[key];
    // Like unload: an older mutation fails its timestamp check on resolution
    // and writes nothing back, not even its rollback.
    const ts = getTimestamp();
    MUTATION[key] = [ts, ts];
    const [, set] = createCacheHelper(cache, key);
    // `_c` holds the data an optimistic update replaced; SWR's State type
    // does not name it.
    set({ data: undefined, _c: undefined } as Parameters<typeof set>[0]);
  }
  return dropped.length;
}
