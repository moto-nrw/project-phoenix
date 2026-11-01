/**
 * Generation lock of ~/lib/swr (#5006): the one source of truth for whether
 * a piece of SWR data or an answer may still be shown or written.
 *
 * Every drop raises a generation:
 * - a scope bump (session boundary, other-tab signal) for every key;
 * - a data bump (refused entity, clearing matching keys) for the keys its
 *   predicate matches;
 * - a request bump (revalidating matching keys) for the keys it names.
 *
 * Write side: every fetcher, preload, retry and mutation in ~/lib/swr notes
 * the request generation of its key when it starts and discards its answer
 * when the generation moved before it finished (DiscardedAnswer). Read side:
 * every value a hook shows carries the data generation it was produced in
 * (stamp); a value of an older data generation is not shown.
 *
 * The cleanup in scope.ts (`unload()`, invalidateSWRKeys) still frees the
 * cache, but nothing here depends on it being complete.
 *
 * Stamps live only as long as their scope generation (#5223): a scope bump
 * releases every stamp and fallback record, and a value without a stamp
 * counts only while no release could have taken its stamp. The server
 * stamps nothing; it never moves a generation.
 */

// swr/no-direct-import permits package imports inside this shared wrapper only.
import { preload as swrPreload } from "swr";

const SPECIAL_KEY = /^\$(inf|sub)\$/;

/** The key without SWR's `$inf$`/`$sub$` marker. */
export function plainSWRKey(key: string): string {
  return key.replace(SPECIAL_KEY, "");
}

let counter = 0;
let scopeGeneration = 0;
// Data bumps since the last scope bump. They stay few (refusals, explicit
// clears) and keep matching keys that appear only later, such as a seed.
let dataBumps: { generation: number; matches: (key: string) => boolean }[] = [];
// Request bumps name concrete keys: only requests in flight at the bump
// matter, and SWR keys each of them.
let requestBumps = new Map<string, number>();

/** Captures the generation in which a caller can originate a mutation. */
export function currentSWRGeneration(): number {
  return counter;
}

/** The generation data of `key` must carry to be shown. */
export function dataGenerationOf(key: string): number {
  const plain = plainSWRKey(key);
  let generation = scopeGeneration;
  for (const bump of dataBumps) {
    if (bump.generation > generation && bump.matches(plain)) {
      generation = bump.generation;
    }
  }
  return generation;
}

/** The generation an answer for `key` must have started in to be written. */
export function requestGenerationOf(key: string): number {
  return Math.max(
    dataGenerationOf(key),
    requestBumps.get(plainSWRKey(key)) ?? 0,
  );
}

// Moves with every scope or data bump; hooks render again on it, so no
// value of an older generation stays on screen. Request bumps leave values
// visible and render nothing.
let dataMoves = 0;
const listeners = new Set<() => void>();

function moveData(): void {
  dataMoves += 1;
  for (const listener of listeners) listener();
}

/** How often values stopped counting; see useSWRDataGeneration. */
export function currentSWRDataGeneration(): number {
  return dataMoves;
}

export function subscribeSWRDataGeneration(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** Session boundary: no earlier answer or value of any key counts. */
export function bumpScopeGeneration(): void {
  counter += 1;
  releaseStamps();
  scopeGeneration = counter;
  dataBumps = [];
  requestBumps = new Map();
  moveData();
}

/** Earlier values and answers of the plain keys `matches` stop counting. */
export function bumpDataGeneration(matches: (key: string) => boolean): void {
  counter += 1;
  dataBumps.push({ generation: counter, matches });
  moveData();
}

/** Earlier answers of `keys` stop counting; their values stay visible. */
export function bumpRequestGeneration(keys: Iterable<string>): void {
  counter += 1;
  for (const key of keys) requestBumps.set(plainSWRKey(key), counter);
}

/** Thrown in place of an answer that started in an older generation. */
export class DiscardedAnswer extends Error {
  constructor() {
    super("SWR answer discarded: its generation ended");
    this.name = "DiscardedAnswer";
  }
}

export function isDiscardedAnswer(error: unknown): error is DiscardedAnswer {
  return error instanceof DiscardedAnswer;
}

// The data generation each value was produced in, per key, for the current
// scope generation only. Objects are held weakly; primitives keep their last
// few values, enough for a rollback to the value before an optimistic update.
const PRIMITIVES_KEPT = 4;
const objectStamps = new Map<string, WeakMap<object, number>>();
const primitiveStamps = new Map<string, Map<unknown, number>>();
// The committed primitive an optimistic run in flight rolls back to keeps
// its stamp outside that history until the run settles (#5485): newer
// optimistic values must not evict it. One count per run.
const pinnedStamps = new Map<
  string,
  Map<unknown, { generation: number; runs: number }>
>();

// The data generation from which values of a key that no path of ~/lib/swr
// stamped (a caller's `fallbackData`, a provider fallback) count, per key of
// the current scope generation (#5006). Set when such a value is first shown
// and raised by every fresh answer: a dropped key shows no fallback until data
// of the current generation arrived, across remounts and hooks.
const fallbackGenerations = new Map<string, number>();

// The scope generation from which a value without a stamp counts (#5223).
// It follows the scope only while no stamp was ever released: a released
// stamp leaves its value reachable without one (cache with cleanup off,
// previous data, a late answer), and such a value must not count again.
let unstampedFrom = 0;

// The server renders requests of many sessions in one long process and never
// moves a generation: every value it sees is current. It keeps no metadata.
const isServer = typeof window === "undefined";

function releaseStamps(): void {
  const released =
    objectStamps.size > 0 ||
    primitiveStamps.size > 0 ||
    pinnedStamps.size > 0 ||
    fallbackGenerations.size > 0;
  if (!released && unstampedFrom === scopeGeneration) unstampedFrom = counter;
  objectStamps.clear();
  primitiveStamps.clear();
  pinnedStamps.clear();
  fallbackGenerations.clear();
}

/** Only for tests: forget every stamp and fallback record. */
export function _resetSWRFallbacksForTests(): void {
  objectStamps.clear();
  primitiveStamps.clear();
  pinnedStamps.clear();
  fallbackGenerations.clear();
  unstampedFrom = scopeGeneration;
}

/** Only for tests: how many keys carry stamp or fallback metadata. */
export function _swrStampedKeyCountForTests(): number {
  return new Set([
    ...objectStamps.keys(),
    ...primitiveStamps.keys(),
    ...pinnedStamps.keys(),
    ...fallbackGenerations.keys(),
  ]).size;
}

function isObject(value: unknown): value is object {
  return (
    (typeof value === "object" && value !== null) || typeof value === "function"
  );
}

function stampOf(key: string, value: unknown): number | undefined {
  const plain = plainSWRKey(key);
  if (isObject(value)) return objectStamps.get(plain)?.get(value);
  const stamp = primitiveStamps.get(plain)?.get(value);
  const pinned = pinnedStamps.get(plain)?.get(value)?.generation;
  return stamp === undefined || (pinned !== undefined && pinned > stamp)
    ? pinned
    : stamp;
}

/** Keeps the stamp of a primitive `value` of `key` while a run needs it. */
function pinStamp(key: string, value: unknown): (() => void) | undefined {
  const generation = stampOf(key, value);
  if (isObject(value) || value === undefined || generation === undefined)
    return undefined;
  const plain = plainSWRKey(key);
  let pins = pinnedStamps.get(plain);
  if (!pins) {
    pins = new Map();
    pinnedStamps.set(plain, pins);
  }
  const pin = pins.get(value) ?? { generation, runs: 0 };
  pin.generation = Math.max(pin.generation, generation);
  pin.runs += 1;
  pins.set(value, pin);
  const held = pins;
  return () => {
    // A scope boundary may have released the pins since.
    if (pinnedStamps.get(plain) !== held || held.get(value) !== pin) return;
    pin.runs -= 1;
    if (pin.runs > 0) return;
    held.delete(value);
    if (held.size === 0) pinnedStamps.delete(plain);
  };
}

/** Records that `value` of `key` was produced in data generation `generation`. */
export function stampSWRValue(
  key: string,
  value: unknown,
  generation: number,
): void {
  if (value === undefined || isServer) return;
  noteFreshData(key, generation);
  const plain = plainSWRKey(key);
  if (isObject(value)) {
    let stamps = objectStamps.get(plain);
    if (!stamps) {
      stamps = new WeakMap();
      objectStamps.set(plain, stamps);
    }
    stamps.set(value, generation);
    return;
  }
  let stamps = primitiveStamps.get(plain);
  if (!stamps) {
    stamps = new Map();
    primitiveStamps.set(plain, stamps);
  }
  stamps.delete(value);
  stamps.set(value, generation);
  if (stamps.size > PRIMITIVES_KEPT) {
    stamps.delete(stamps.keys().next().value);
  }
}

/**
 * Whether `value` was stamped for `key` in its current data generation: true
 * for a value of the current data generation, false for an older one,
 * undefined for a value without a stamp (released at a scope boundary, a
 * caller's `fallbackData`, a server value).
 */
export function isCurrentSWRValue(
  key: string,
  value: unknown,
): boolean | undefined {
  if (value === undefined) return true;
  const stamp = stampOf(key, value);
  if (stamp === undefined) return undefined;
  return stamp >= dataGenerationOf(key);
}

/**
 * Whether a cached `value` of `key` may be shown, returned or built on. A
 * value without a stamp counts only while no released stamp or drop of the
 * key could be its own; nothing restamps it (#5223).
 */
export function isVisibleSWRValue(key: string, value: unknown): boolean {
  return (
    isCurrentSWRValue(key, value) ?? unstampedFrom >= dataGenerationOf(key)
  );
}

/** Whether a fallback without a stamp of `key` may be shown now. */
export function isFallbackVisible(key: string): boolean {
  const from = fallbackGenerations.get(plainSWRKey(key));
  // A predicate drop also invalidates keys first read after it. An unseen
  // fallback starts in this scope, unless a scope boundary released stamps.
  return (from ?? unstampedFrom) >= dataGenerationOf(key);
}

/**
 * Records that unstamped fallbacks of `key` were shown in data generation
 * `generation`; the first record of a key stays (call after render).
 */
export function noteFallbackShown(key: string, generation: number): void {
  if (isServer) return;
  const plain = plainSWRKey(key);
  if (!fallbackGenerations.has(plain)) {
    fallbackGenerations.set(plain, generation);
  }
}

function noteFreshData(key: string, generation: number): void {
  const plain = plainSWRKey(key);
  const from = fallbackGenerations.get(plain);
  if (from === undefined || from < generation) {
    fallbackGenerations.set(plain, generation);
  }
}

// The data generation each failure belongs to. A failure of an older
// generation is not shown; the refusal an entity drop keeps is restamped.
const errorStamps = new WeakMap<object, number>();

export function stampSWRError(error: unknown, generation: number): void {
  if (isObject(error)) errorStamps.set(error, generation);
}

/** Whether `error` of `key` may be shown; unstamped failures count. */
export function isCurrentSWRError(key: string, error: unknown): boolean {
  if (!isObject(error)) return true;
  const stamp = errorStamps.get(error);
  return stamp === undefined || stamp >= dataGenerationOf(key);
}

/** Generations a fetch, preload or mutation of `key` started in. */
export interface SWRStart {
  readonly request: number;
  readonly data: number;
}

export function startOf(key: string): SWRStart {
  return { request: requestGenerationOf(key), data: dataGenerationOf(key) };
}

/** Whether an answer started at `start` may still be written under `key`. */
export function isStillCurrent(key: string, start: SWRStart): boolean {
  return requestGenerationOf(key) === start.request;
}

/**
 * Whether a mutation started at `start` may still write under `key`. A
 * request bump only ends answers of requests (#5223): the caller's own write
 * stays valid until a scope or data bump of its key.
 */
function isMutationCurrent(key: string, start: SWRStart): boolean {
  return dataGenerationOf(key) === start.data;
}

// Preloads keep the generation they started in, so a later reader that
// takes the preloaded answer judges it by that start, not by its own.
const preloadStarts = new WeakMap<object, SWRStart>();

/**
 * Wraps a fetcher of `key` in the generation lock: an answer or failure
 * that started in an older generation is replaced by DiscardedAnswer, a
 * current answer is stamped. `fetcher` may return a preload of `key` (SWR hands it over in
 * place of a request); that preload is judged by its own start.
 */
export function lockFetcher<A extends unknown[], T>(
  key: string,
  fetcher: (...args: A) => T | Promise<T>,
): (...args: A) => Promise<T> {
  return async (...args) => {
    // SWR runs a hook's latest fetcher for any key it still revalidates,
    // also an earlier one; never answer an earlier key with this fetcher.
    if (typeof args[0] === "string" && args[0] !== key) {
      throw new DiscardedAnswer();
    }
    let start = startOf(key);
    const pending = fetcher(...args);
    if (isObject(pending)) start = preloadStarts.get(pending) ?? start;
    let value: T;
    try {
      value = await pending;
    } catch (error) {
      // A failure of an ended generation is no result either: not shown,
      // not logged, not retried.
      if (!isStillCurrent(key, start)) throw new DiscardedAnswer();
      stampSWRError(error, start.data);
      throw error;
    }
    if (!isStillCurrent(key, start)) throw new DiscardedAnswer();
    stampSWRValue(key, value, start.data);
    return value;
  };
}

/** SWR's preload under the generation lock; for the full, scoped key. */
export function preloadSWR<T>(
  key: string,
  fetcher: () => Promise<T>,
): Promise<T> {
  const start = startOf(key);
  const request = swrPreload(key, lockFetcher(key, fetcher)) as Promise<T>;
  preloadStarts.set(request, start);
  return request;
}

/** One mutation run under the generation lock (G25). */
export interface SWRMutationRun {
  /** The run's start token: whether its key's data generation still holds. */
  readonly isCurrent: () => boolean;
  /** Call once SWR's mutate for this run settled: releases its pin. */
  readonly done: () => void;
}

/**
 * Puts the data argument of a mutation of `key` under the generation lock.
 *
 * G25: whether the run writes, stamps, rolls back or hands its result to the
 * caller depends only on the token it took at its start (the data generation
 * of its key), never on its result being equal to a value stamped since: two
 * primitives of different runs cannot be told apart by value. A value is
 * stamped only where SWR writes it: the optimistic value, the populated
 * result (only the latest run of a key reaches populateCache), the rollback.
 * A result SWR never writes (overtaken run, `populateCache: false`) stays out
 * of the four-value history of its key. The committed primitive an
 * optimistic run rolls back to keeps its stamp until `done` (#5485).
 */
export function lockMutation(
  key: string,
  data: unknown,
  options: unknown,
): [unknown, unknown, SWRMutationRun] {
  const start = startOf(key);
  const isCurrent = () => isMutationCurrent(key, start);
  const stamp = <V>(value: V): V => {
    if (isCurrent()) stampSWRValue(key, value, start.data);
    return value;
  };
  let unpin: (() => void) | undefined;
  const run: SWRMutationRun = {
    isCurrent,
    done: () => {
      unpin?.();
      unpin = undefined;
    },
  };
  if (typeof options === "boolean") options = { revalidate: options };
  const given = (options ?? {}) as {
    populateCache?: boolean | ((result: unknown, current: unknown) => unknown);
    optimisticData?: unknown;
    rollbackOnError?: boolean | ((error: unknown) => boolean);
  };
  // A value of an ended generation is no base for a new one: functions of
  // the mutation see nothing in its place, as for an empty entry.
  const base = (value: unknown): unknown =>
    isVisibleSWRValue(key, value) ? value : undefined;
  const locked =
    typeof data === "function"
      ? (current: unknown) => (data as (c: unknown) => unknown)(base(current))
      : data;
  const optimistic = given.optimisticData;
  const rollback = given.rollbackOnError;
  let committed: unknown;
  // An optimistic value is shown whatever populateCache says: stamp it.
  const withOptimistic = {
    ...given,
    ...(optimistic === undefined
      ? {}
      : {
          optimisticData: (...args: unknown[]) => {
            // SWR hands the committed value first: the one it rolls back to.
            committed = base(args[0]);
            if (unpin === undefined && isCurrent())
              unpin = pinStamp(key, committed);
            return stamp(
              typeof optimistic === "function"
                ? (optimistic as (...a: unknown[]) => unknown)(
                    ...args.map(base),
                  )
                : optimistic,
            );
          },
          rollbackOnError: (error: unknown) => {
            // An ended run writes no rollback either.
            if (!isCurrent()) return false;
            const shouldRollback =
              typeof rollback === "function"
                ? rollback(error)
                : rollback !== false;
            // The value SWR restores carries the stamp it had before.
            if (shouldRollback) stamp(committed);
            return shouldRollback;
          },
        }),
  };
  const populate = given.populateCache;
  if (populate === false) return [locked, withOptimistic, run];
  return [
    locked,
    {
      ...withOptimistic,
      populateCache: (result: unknown, current: unknown) => {
        if (!isCurrent()) return current;
        return stamp(
          typeof populate === "function"
            ? populate(result, base(current))
            : result,
        );
      },
    },
    run,
  ];
}
