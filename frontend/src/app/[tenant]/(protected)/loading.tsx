"use client";

import { Loading } from "~/components/ui/loading";
import { useNavigationProgressPending } from "~/components/ui/navigation-progress";

/**
 * Zeigt beim ersten Aufruf einen Platzhalter im persistenten Portalrahmen.
 * Bei einem bereits gemeldeten Seitenwechsel bleibt die alte Seite sichtbar;
 * der Fortschrittsbalken der Hülle übernimmt dann die Rückmeldung.
 */
export default function ProtectedLoading() {
  return useNavigationProgressPending() ? null : <Loading fullPage={false} />;
}
