/** Runs a bounded set of tasks and keeps their input order in the result. */
export async function mapConcurrent<T, U>(
  items: readonly T[],
  concurrency: number,
  run: (item: T, index: number) => Promise<U>,
): Promise<U[]> {
  assertConcurrency(concurrency);
  const results = new Array<U>(items.length);
  let next = 0;
  let failure: { error: unknown } | undefined;
  async function worker(): Promise<void> {
    while (!failure && next < items.length) {
      const index = next++;
      try {
        results[index] = await run(items[index]!, index);
      } catch (error) {
        failure ??= { error };
      }
    }
  }
  // Every started task settles before the caller closes its browser or removes
  // a staging directory. Promise.all's early rejection would race cleanup.
  await Promise.all(
    Array.from({ length: Math.min(concurrency, items.length) }, worker),
  );
  if (failure) throw failure.error;
  return results;
}

export function assertConcurrency(value: number): void {
  if (!Number.isSafeInteger(value) || value < 1) {
    throw new Error(
      `Screenshot concurrency must be a positive integer: ${value}`,
    );
  }
}
