/**
 * Deep equality for SWR cache data.
 *
 * SWR keeps the cached value when `compare(cached, fetched)` is true, so a
 * compare that reports false positives silently drops fresh data. SWR 2.5
 * defaults to `dequal/lite`, which treats ANY two `Map`s (and `Set`s) as equal:
 * they have no own enumerable keys. Caches holding a Map, such as the daily
 * time-tracking projection, then kept their first value forever, and the
 * table showed the old Saldo after every correction until a reload (#3258).
 *
 * This compare keeps the deep-equal short cut for plain data and compares
 * Map and Set entries by content.
 */
export function swrDataEqual(a: unknown, b: unknown): boolean {
  if (Object.is(a, b)) return true;
  if (
    typeof a !== "object" ||
    typeof b !== "object" ||
    a === null ||
    b === null
  ) {
    return false;
  }
  if (Object.getPrototypeOf(a) !== Object.getPrototypeOf(b)) return false;

  if (a instanceof Map && b instanceof Map) {
    if (a.size !== b.size) return false;
    for (const [key, value] of a) {
      if (!b.has(key) || !swrDataEqual(value, b.get(key))) return false;
    }
    return true;
  }
  if (a instanceof Set && b instanceof Set) {
    if (a.size !== b.size) return false;
    for (const value of a) {
      if (!b.has(value)) return false;
    }
    return true;
  }
  if (a instanceof Date && b instanceof Date) {
    return a.getTime() === b.getTime();
  }
  if (Array.isArray(a) && Array.isArray(b)) {
    return (
      a.length === b.length &&
      a.every((value, index) => swrDataEqual(value, b[index]))
    );
  }

  const aKeys = Object.keys(a);
  const bKeys = Object.keys(b);
  if (aKeys.length !== bKeys.length) return false;
  const bRecord = b as Record<string, unknown>;
  const aRecord = a as Record<string, unknown>;
  return aKeys.every(
    (key) =>
      Object.prototype.hasOwnProperty.call(bRecord, key) &&
      swrDataEqual(aRecord[key], bRecord[key]),
  );
}
