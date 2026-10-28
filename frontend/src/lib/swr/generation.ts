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

// The data generation each value was produced in, per key. Objects are held
// weakly; primitives keep their last few values, enough for a rollback to
// the value before an optimistic update.
const PRIMITIVES_KEPT = 4;
const objectStamps = new Map<string, WeakMap<object, number>>();
const primitiveStamps = new Map<string, Map<unknown, number>>();

function isObject(value: unknown): value is object {
  return (
    (typeof value === "object" && value !== null) || typeof value === "function"
  );
}

function stampOf(key: string, value: unknown): number | undefined {
  const plain = plainSWRKey(key);
  return isObject(value)
    ? objectStamps.get(plain)?.get(value)
    : primitiveStamps.get(plain)?.get(value);
}

/** Records that `value` of `key` was produced in data generation `generation`. */
export function stampSWRValue(
  key: string,
  value: unknown,
  generation: number,
): void {
  if (value === undefined) return;
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
 * Whether `value` may be shown under `key`: true for a value of the current
 * data generation, false for an older one, undefined for a value no path of
 * ~/lib/swr produced (a raw write, a caller's `fallbackData`).
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

/**
 * Puts the data argument of a mutation of `key` under the generation lock.
 * Optimistic and computed values are stamped with the generation the
 * mutation started in; an answer that arrives after the generation moved is
 * not written, the entry keeps its committed value instead.
 */
export function lockMutation(
  key: string,
  data: unknown,
  options: unknown,
): [unknown, unknown] {
  const start = startOf(key);
  const stamp = <V>(value: V): V => {
    stampSWRValue(key, value, start.data);
    return value;
  };
  const settle = (value: unknown): unknown =>
    isObject(value) && typeof (value as Promise<unknown>).then === "function"
      ? (value as Promise<unknown>).then(stamp)
      : stamp(value);
  const locked =
    typeof data === "function"
      ? (current: unknown) => settle((data as (c: unknown) => unknown)(current))
      : settle(data);
  if (typeof options === "boolean") options = { revalidate: options };
  const given = (options ?? {}) as {
    populateCache?: boolean | ((result: unknown, current: unknown) => unknown);
    optimisticData?: unknown;
  };
  if (given.populateCache === false) return [locked, options];
  const populate = given.populateCache;
  const optimistic = given.optimisticData;
  return [
    locked,
    {
      ...given,
      ...(optimistic === undefined
        ? {}
        : {
            optimisticData:
              typeof optimistic === "function"
                ? (...args: unknown[]) =>
                    stamp((optimistic as (...a: unknown[]) => unknown)(...args))
                : stamp(optimistic),
          }),
      populateCache: (result: unknown, current: unknown) => {
        if (!isStillCurrent(key, start)) return current;
        return typeof populate === "function"
          ? stamp(populate(result, current))
          : result;
      },
    },
  ];
}
