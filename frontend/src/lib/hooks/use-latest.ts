"use client";

import { useLayoutEffect, useRef } from "react";

/**
 * Keeps the most recently committed value available to asynchronous callbacks
 * without mutating refs during render.
 *
 * Deliberately without a dependency array: callers hand this hook a closure
 * built fresh on every render — that is the whole point of a latest-ref — so a
 * `[value]` array would list a dependency that always differs and only pretends
 * to skip work. Writing after every commit is the same behaviour without the
 * pretence.
 */
export function useLatest<T>(value: T) {
  const ref = useRef(value);

  useLayoutEffect(() => {
    ref.current = value;
  });

  return ref;
}
