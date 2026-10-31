"use client";

import { useCallback, useState } from "react";

/**
 * Remembers that the school switched a messaging feature off while a
 * conversation stood open: the history has loaded, only sending fails with
 * the "switched off" code. The cached settings flag may still say "on", so the
 * page cannot rely on it alone.
 *
 * The mark belongs to one conversation (`scope`). It ends once the settings
 * flag turns on again after having been off: the school allows sending again,
 * and the composer returns without a reload.
 */
export function useSwitchedOffWhileOpen(
  enabled: boolean,
  scope: string,
): {
  readonly switchedOff: boolean;
  readonly markSwitchedOff: () => void;
} {
  const [markedScope, setMarkedScope] = useState<string>();
  const [seenEnabled, setSeenEnabled] = useState(enabled);
  // Adjust state while rendering instead of in an effect, so the composer
  // returns in the same render that sees the flag back on.
  if (seenEnabled !== enabled) {
    setSeenEnabled(enabled);
    if (enabled) setMarkedScope(undefined);
  }
  const markSwitchedOff = useCallback(() => setMarkedScope(scope), [scope]);
  return { switchedOff: markedScope === scope, markSwitchedOff };
}
