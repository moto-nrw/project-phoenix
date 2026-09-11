"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createLogger } from "~/lib/logger";

export function useClipboardCopy(component: string, resetMs = 1500) {
  const [copied, setCopied] = useState(false);
  const logger = useMemo(() => createLogger({ component }), [component]);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
    };
  }, []);

  const copy = useCallback(
    async (text: string) => {
      try {
        await navigator.clipboard.writeText(text);
        setCopied(true);
        if (timeoutRef.current) clearTimeout(timeoutRef.current);
        timeoutRef.current = setTimeout(() => setCopied(false), resetMs);
        return true;
      } catch (err) {
        logger.warn("clipboard_copy_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        return false;
      }
    },
    [logger, resetMs],
  );

  return { copied, copy };
}
