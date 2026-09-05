"use client";

import { Loading } from "~/components/ui/loading";
import {
  useNavigationFallbackSuppressed,
  useNavigationProgressPending,
} from "~/components/ui/navigation-progress";

/**
 * Zeigt beim ersten Aufruf einen Platzhalter im persistenten Portalrahmen.
 * Nach einem clientseitigen Wechsel bleibt die zuvor gerenderte Seite sichtbar,
 * auch wenn der Wechsel nicht von einer Shell-Navigation gemeldet wurde.
 */
export default function ProtectedLoading() {
  const pending = useNavigationProgressPending();
  const fallbackSuppressed = useNavigationFallbackSuppressed();
  return pending || fallbackSuppressed ? null : <Loading fullPage={false} />;
}
