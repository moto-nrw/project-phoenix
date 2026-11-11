"use client";

import { useLayoutEffect, useState } from "react";
import { ApiError } from "~/lib/api-error";
import { useDiscardRefusedEntity } from "~/lib/swr/hooks";

/** A refusal drops the scoped cache, including previous data and pending reads.
 * Later transient failures cannot restore it; only a successful fetch can. */
export function useAccessControlledData<T>(
  key: string | null,
  data: T | undefined,
  error: unknown,
) {
  const discard = useDiscardRefusedEntity();
  const [refusedKey, setRefusedKey] = useState<string | null>(null);
  const refusal =
    error instanceof ApiError && (error.status === 401 || error.status === 403);
  const accessRefused =
    refusal ||
    (key !== null &&
      refusedKey === key &&
      (Boolean(error) || data === undefined));

  useLayoutEffect(() => {
    if (refusal && key !== null) {
      setRefusedKey(key);
      discard(error, {
        codes: error.code ? [error.code] : [],
        matches: (candidate) => candidate === key,
        refusedKey: key,
      });
    }
  }, [key, error, refusal, discard]);
  useLayoutEffect(() => {
    if (!error && data !== undefined) setRefusedKey(null);
  }, [data, error]);

  return { data: accessRefused ? undefined : data, accessRefused };
}
