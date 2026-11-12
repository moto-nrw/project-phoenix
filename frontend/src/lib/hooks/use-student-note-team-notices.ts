"use client";

import { useSession } from "next-auth/react";
import { useCallback } from "react";
import { hasPermission } from "~/lib/auth-utils";
import { useShellAuthSafe } from "~/lib/shell-auth-context";
import {
  studentNotesService,
  type StudentNoteTeamNotice,
} from "~/lib/student-notes-api";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { studentKeys } from "~/lib/swr/portal-keys";

/**
 * Notizen, die jemand aus dem Team mit „Speichern und Team informieren“
 * geteilt hat und die diese Person noch nicht gelesen hat (#6854).
 *
 * Header und Notizen-Reiter lesen denselben SWR-Eintrag: öffnet die Person
 * die Notizen eines Kindes, schreibt `markRead` die Antwort des Servers
 * (die noch ungelesenen) zurück, und der rote Zähler im Header zieht mit.
 * Neu geladen wird beim Zurückkehren ins Fenster und jede Minute, wie die
 * Erinnerungen; eine geteilte Notiz kommt zusätzlich als Mitteilung aufs
 * Gerät, wenn die Person das zulässt.
 */
export function useStudentNoteTeamNotices() {
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
  const key = enabled ? studentKeys.teamNotices(accountId) : null;
  const { data, error, isLoading } = useSWRAuth<StudentNoteTeamNotice[]>(
    key,
    () => studentNotesService.listTeamNotices(),
    {
      revalidateOnFocus: true,
      refreshInterval: 60_000,
      keepPreviousData: false,
    },
  );
  const tenantMutate = useTenantMutate();
  const markRead = useCallback(
    async (studentId: string) => {
      if (!key) return;
      const remaining =
        await studentNotesService.markTeamNoticesRead(studentId);
      await tenantMutate(key, remaining, { revalidate: false });
    },
    [key, tenantMutate],
  );
  const notices = enabled ? (data ?? []) : [];
  return {
    enabled,
    notices,
    error: enabled ? error : undefined,
    isLoading: enabled && isLoading,
    markRead,
  };
}
