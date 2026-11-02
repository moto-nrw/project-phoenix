/** Run every cleanup in order without replacing the measurement's original error. */
export async function withPerfCleanup<T>(
  measure: () => Promise<T>,
  cleanups: ReadonlyArray<() => Promise<void>>,
): Promise<T> {
  const errors: unknown[] = [];
  let result: T | undefined;
  try {
    result = await measure();
  } catch (error) {
    errors.push(error);
  }
  const measurementFailed = errors.length > 0;
  for (const cleanup of cleanups) {
    try {
      await cleanup();
    } catch (error) {
      errors.push(error);
    }
  }
  if (errors.length === 1) throw errors[0];
  if (errors.length > 1) {
    throw new AggregateError(
      errors,
      measurementFailed
        ? "Performance measurement failed; cleanup also failed"
        : "Performance cleanup failed",
      { cause: errors[0] },
    );
  }
  return result as T;
}
