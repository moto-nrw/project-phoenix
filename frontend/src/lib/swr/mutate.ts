// swr/no-direct-import permits package imports inside this shared wrapper only.
import { mutate as swrMutate } from "swr";
import { lockMutation } from "./generation";

/**
 * SWR's global mutate under the generation lock (#5006): a write to a key
 * is stamped with the generation it started in, and an answer that settles
 * after that generation ended is not kept. Revalidations and key filters
 * pass through unchanged.
 */
export const mutate = ((...args: Parameters<typeof swrMutate>) => {
  const [key, data, options] = args;
  if (typeof key !== "string" || args.length < 2 || data === undefined) {
    return swrMutate(...args);
  }
  const [locked, lockedOptions] = lockMutation(key, data, options);
  return swrMutate(key, locked, lockedOptions as never);
}) as typeof swrMutate;
