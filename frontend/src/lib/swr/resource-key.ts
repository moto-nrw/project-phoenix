/** Stable parameter identity; omitted values and object insertion order do not fork a cache. */
export function resourceKey(family: string, params: object): string {
  return `${family}${JSON.stringify(params, (_key, value: unknown) => {
    if (value && typeof value === "object" && !Array.isArray(value)) {
      return Object.fromEntries(
        Object.entries(value)
          .filter(([, v]) => v !== undefined)
          .sort(([a], [b]) => a.localeCompare(b)),
      );
    }
    return value;
  })}`;
}
