// swr/no-direct-import permits package imports inside this shared wrapper only.
import { mutate as swrMutate } from "swr";

type Matcher = string | ((key: unknown) => boolean);

/**
 * SWR's global mutate, limited to revalidating or clearing (#5007): a key or
 * key filter without data. A write of data has no originating generation
 * here, so a retained call after a scope bump would land in the next
 * session; data goes through useTenantMutate, the bound mutate of
 * useSWRAuth or the seed helpers instead.
 */
export function mutate(
  ...args: [
    key: Matcher,
    data?: undefined,
    options?: boolean | { revalidate?: boolean },
  ]
): Promise<unknown> {
  const [key, data, options] = args;
  // SWR counts the arguments: `mutate(key)` revalidates, while an explicit
  // `undefined` clears the entry. Pass the call through as given.
  if (args.length < 2) return swrMutate(key);
  if (data !== undefined) {
    return Promise.reject(
      new TypeError(
        "~/lib/swr mutate only revalidates or clears; write data through useTenantMutate",
      ),
    );
  }
  return swrMutate(key, undefined, options as never);
}
