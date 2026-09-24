"use client";

import { usePathname, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { useMemo } from "react";

import { leadsSchool } from "~/lib/auth-utils";
import { buildHelpHref, type HelpRole } from "~/lib/help-topics";
import { useShellAuth } from "~/lib/shell-auth-context";
import {
  useNFCEnabled,
  useOpenCareGroupMode,
  usePresenceMode,
} from "~/lib/tenant-context";

/**
 * Die Adresse hinter dem Navigationseintrag `Hilfe`.
 *
 * Nackt auf `/help` fragte die Hilfe zuerst, fuer wen die Anleitung ist
 * und wie die OGS arbeitet -- vier Fragen, deren Antworten die angemeldete
 * Sitzung bereits kennt. Wer aus der App kommt, soll direkt bei seinen
 * Themen landen und ueber `return_to` wieder zurueckfinden.
 *
 * Seitenleiste und mobile Navigation holen die Adresse beide hier, damit
 * Desktop und Handy nicht wieder auseinanderlaufen (#3575). Die Adresse
 * bleibt ohne Schulnamen-Praefix: `/help` ist nicht mandantengebunden.
 */
export function useHelpHref(): string {
  const rawPathname = usePathname();
  const searchParams = useSearchParams();
  const { data: session } = useSession();
  const { mode } = useShellAuth();
  const nfcEnabled = useNFCEnabled();
  const presenceMode = usePresenceMode();
  const openCareGroupMode = useOpenCareGroupMode();
  const userLeadsSchool = leadsSchool(session);

  return useMemo(() => {
    const role: HelpRole =
      mode === "parent" ? "parent" : userLeadsSchool ? "lead" : "caregiver";
    const currentQuery = searchParams.toString();
    return buildHelpHref({
      role,
      nfcEnabled,
      presenceMode: presenceMode === "binary" ? "binary" : "detailed",
      groupMode: openCareGroupMode ? "open_care" : "fixed_groups",
      returnTo: currentQuery ? `${rawPathname}?${currentQuery}` : rawPathname,
    });
  }, [
    mode,
    nfcEnabled,
    openCareGroupMode,
    presenceMode,
    rawPathname,
    searchParams,
    userLeadsSchool,
  ]);
}
