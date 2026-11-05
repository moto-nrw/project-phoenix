"use client";

import { useSyncExternalStore } from "react";
import { useShellAuth } from "~/lib/shell-auth-context";
import { traegerPortalUrl } from "~/lib/traeger-handoff";

const neverChanges = () => () => undefined;

/**
 * Die Adresse des Rückwegs aus der Schule ins Träger-Portal (#5687), nur für
 * eine Sitzung, die über das Träger-Büro geöffnet wurde. Das Layout der Schule
 * kennt die Sitzung schon auf dem Server; die Adresse entsteht aber erst im
 * Browser, deshalb erscheint der Link dort nach dem Hydrieren.
 */
export function useTraegerReturnHref(): string | null {
  const { isViaTraeger } = useShellAuth();
  const href = useSyncExternalStore(neverChanges, traegerPortalUrl, () => null);
  return isViaTraeger ? href : null;
}
