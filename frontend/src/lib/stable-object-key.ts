const keys = new WeakMap<object, string>();
let nextKey = 0;

/**
 * Gives client-side editable rows a React identity without adding UI-only
 * fields to API payloads. Call `copyStableObjectKey` whenever replacing a row
 * object so controlled inputs retain focus through immutable updates.
 */
export function getStableObjectKey(value: object, prefix = "row"): string {
  const existing = keys.get(value);
  if (existing) return existing;
  nextKey += 1;
  const key = `${prefix}-${nextKey}`;
  keys.set(value, key);
  return key;
}

export function copyStableObjectKey<T extends object>(
  source: object,
  replacement: T,
): T {
  keys.set(replacement, getStableObjectKey(source));
  return replacement;
}
