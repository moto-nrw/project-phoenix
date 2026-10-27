"use client";

import { useEffect, useRef } from "react";

/**
 * The account a value belongs to: the one signed in when this value first
 * appeared (#5000). Lists, URL selections and loaded data survive an account
 * switch unchanged until the new account's arrive; such a value keeps its
 * previous owner, so nothing is remembered from it for the new account.
 *
 * Call it before the effects that read the result: effects run in
 * declaration order, so the owner is current when they run.
 */
export function useArrivalOwner<T>(value: T, owner: string | null) {
  const arrivalRef = useRef<{ value: T; owner: string | null } | null>(null);
  const ownerRef = useRef<string | null>(null);
  useEffect(() => {
    if (!arrivalRef.current || arrivalRef.current.value !== value) {
      arrivalRef.current = { value, owner };
    }
    ownerRef.current = arrivalRef.current.owner;
  }, [value, owner]);
  return ownerRef;
}
