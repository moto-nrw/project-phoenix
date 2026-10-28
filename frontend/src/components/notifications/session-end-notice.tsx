"use client";

import { useEffect } from "react";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import {
  installSessionEndBarrier,
  SESSION_END_EVENT,
  type SessionEndDetail,
} from "~/lib/session-end-barrier";

export const DISCARDED_WRITE_MESSAGE =
  "Ihre letzte Änderung wurde nicht gespeichert, weil die Abmeldung lief. Bitte speichern Sie sie noch einmal.";

/**
 * A logout waits for this tab's requests before it deletes the cookie (#5126).
 * When the tab stays after all, the writes held meanwhile were not sent; this
 * says so once, instead of each caller showing its own error.
 */
export function SessionEndNotice() {
  const { showMessage } = useApiErrorDisplay();

  useEffect(installSessionEndBarrier, []);

  useEffect(() => {
    const onSessionEnd = (event: Event) => {
      const detail = (event as CustomEvent<SessionEndDetail>).detail;
      if (!detail.ending && detail.discardedWrites > 0)
        showMessage(DISCARDED_WRITE_MESSAGE);
    };
    window.addEventListener(SESSION_END_EVENT, onSessionEnd);
    return () => window.removeEventListener(SESSION_END_EVENT, onSessionEnd);
  }, [showMessage]);

  return null;
}
