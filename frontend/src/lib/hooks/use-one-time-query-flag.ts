"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";

/**
 * Reads a `?name=1` query flag once on mount and then strips it from the URL
 * via `window.history.replaceState` (no history entry, no server roundtrip;
 * Next.js syncs the router). The returned value stays `true` for the lifetime
 * of the component, but a refresh, bookmark, or shared link no longer carries
 * the flag — it is one-time by construction. Other query params and the hash
 * survive the rewrite.
 */
export function useOneTimeQueryFlag(name: string): boolean {
  const searchParams = useSearchParams();
  const [value] = useState(() => searchParams.get(name) === "1");

  useEffect(() => {
    if (!value) return;
    const params = new URLSearchParams(window.location.search);
    if (params.get(name) === null) return;
    params.delete(name);
    const qs = params.toString();
    window.history.replaceState(
      null,
      "",
      `${window.location.pathname}${qs ? `?${qs}` : ""}${window.location.hash}`,
    );
  }, [value, name]);

  return value;
}
