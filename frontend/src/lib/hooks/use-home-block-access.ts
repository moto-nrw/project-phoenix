"use client";

import { useMemo } from "react";
import { useSession } from "next-auth/react";

import {
  hasEffectiveAdminScope,
  hasPermission,
  isCaregiver,
} from "~/lib/auth-utils";
import { canOpenRequestsPage } from "~/lib/change-request-access";
import type { HomeBlockAccess } from "~/lib/home-blocks";
import { useOptionalSupervision } from "~/lib/supervision-context";

/**
 * Die Rechte der angemeldeten Person in der Form, die der Bausteinkatalog der
 * Startseite erwartet (#2180).
 *
 * Bewusst keine Rollennamen: Rollen sind pro Schule frei anlegbar, eine
 * selbst angelegte Rolle muss ohne Codeänderung eine brauchbare Startseite
 * bekommen. Deshalb entscheidet ausschliesslich das Recht, an dem auch der
 * Endpunkt hinter dem Baustein hängt. Was hier durchkommt, prüft der Server
 * anschliessend noch einmal selbst.
 *
 * Zwei Angaben sind keine Rechte, sondern Zuschnitt: ob die Person selbst
 * betreut (Basisrolle `user`/`teacher`) und ob sie heute eine eigene Gruppe
 * hat. Beides entscheidet nur, welche Standardansicht sie bekommt und ob
 * „Meine Gruppe" etwas zu zeigen hätte.
 */
export function useHomeBlockAccess(): HomeBlockAccess {
  const { data: session } = useSession();
  const requestsPage = canOpenRequestsPage(session);
  const adminScope = hasEffectiveAdminScope(session);
  const caresForGroups = isCaregiver(session);
  const { hasGroups } = useOptionalSupervision();

  return useMemo(
    () => ({
      isAdminScope: adminScope,
      // Adminzuschnitt kommt überall durch, wie in der Seitenleiste: das
      // Backend lässt ein Wildcard-Recht denselben Weg gehen.
      has: (permission: string) =>
        adminScope || hasPermission(session, permission),
      canOpenRequestsPage: requestsPage,
      caresForGroups,
      hasOwnGroups: hasGroups,
    }),
    [session, adminScope, requestsPage, caresForGroups, hasGroups],
  );
}
