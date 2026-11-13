"use client";

import { useSession } from "next-auth/react";
import { useCallback } from "react";
import { hasPermission } from "~/lib/auth-utils";
import {
  pickupChangeNoticesService,
  type PickupChangeNotice,
  type PickupChangeNoticeRevision,
} from "~/lib/pickup-change-notices-api";
import { useShellAuthSafe } from "~/lib/shell-auth-context";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { studentKeys } from "~/lib/swr/portal-keys";

/**
 * Geänderte Abholzeiten von Kindern, für die diese Person zuständig ist, die
 * sie noch nicht gesehen hat (#6855). Die OGS hat eine Anfrage der Eltern für
 * einen Tag freigegeben oder die Freigabe zurückgenommen.
 *
 * Header und Kinderakte lesen denselben SWR-Eintrag wie die geteilten
 * Notizen (#6854): öffnet die Person das Kind, schreibt `markRead` die
 * Antwort des Servers (die noch ungesehenen) zurück, und der rote Zähler
 * zieht mit. Neu geladen wird beim Zurückkehren ins Fenster und jede Minute.
 */
export function usePickupChangeNotices() {
  const { data: session, status } = useSession();
  // Outside the staff shell (previews, isolated tests) there is no reader.
  const mode = useShellAuthSafe()?.mode;
  const accountId = session?.user?.id;
  const enabled =
    status === "authenticated" &&
    !!accountId &&
    mode === "teacher" &&
    hasPermission(session, "users:read");
  // The account keeps two staff apart on one device.
  const key = enabled ? studentKeys.pickupChangeNotices(accountId) : null;
  const { data } = useSWRAuth<PickupChangeNotice[]>(
    key,
    () => pickupChangeNoticesService.list(),
    {
      revalidateOnFocus: true,
      refreshInterval: 60_000,
      keepPreviousData: false,
    },
  );
  const tenantMutate = useTenantMutate();
  const markRead = useCallback(
    async (
      studentId: string,
      observed: readonly PickupChangeNoticeRevision[],
    ) => {
      if (!key || observed.length === 0) return;
      const remaining = await pickupChangeNoticesService.markRead(
        studentId,
        observed,
      );
      await tenantMutate(key, remaining, { revalidate: false });
    },
    [key, tenantMutate],
  );
  return { notices: enabled ? (data ?? []) : [], markRead };
}
