"use client";

/**
 * Elternhinweise der Schule (ADR 0196): Liegt ein eigener Schließtag in einer
 * gebuchten Ferienwoche, schlägt moto nach dem Speichern oder Löschen vor,
 * die Eltern zu informieren. Ein Klick hier oder im Träger-Büro beauftragt
 * die E-Mail. Die Schule sieht nur, wie viele Familien gebucht haben, nie
 * Namen oder Kontakte. `version` ändert sich nach jedem Speichern und
 * Löschen eines Schließtags; dann lädt die Liste neu.
 */

import { useEffect, useRef } from "react";
import { useSession } from "next-auth/react";

import {
  HolidayClosureNoticeList,
  useNoticeNotify,
} from "./holiday-closure-notice-list";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { hasPermission } from "~/lib/auth-utils";
import {
  fetchHolidayClosureNotices,
  notifyHolidayClosureNotice,
  sendingRefreshInterval,
} from "~/lib/holiday-closure-notices";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import { timetableKeys } from "~/lib/swr/portal-keys";

export function HolidayClosureNotices({
  version,
}: Readonly<{ version: number }>) {
  const { data: session } = useSession();
  const canNotify = hasPermission(session, "schedules:update");
  const response = useSWRAuth(
    canNotify ? timetableKeys.holidayClosureNotices() : null,
    fetchHolidayClosureNotices,
    { refreshInterval: sendingRefreshInterval },
  );
  const { mutate } = response;
  const loadError = useSwrLoadError(
    response.error,
    "die Hinweise an Eltern",
    mutate,
  );
  const { isSending, notify } = useNoticeNotify(
    notifyHolidayClosureNotice,
    mutate,
  );

  const seen = useRef(version);
  useEffect(() => {
    if (seen.current === version) return;
    seen.current = version;
    void mutate();
  }, [version, mutate]);

  if (!canNotify) return null;
  return (
    <HolidayClosureNoticeList
      title="Eltern der Ferienbetreuung informieren"
      description="Diese Schließtage liegen in gebuchten Ferienwochen. Die Eltern bekommen erst eine E-Mail, wenn Sie oder das Träger-Büro „Eltern informieren“ wählen."
      rows={(response.data ?? []).map((notice) => ({
        notice,
        title: `„${notice.reason}“`,
      }))}
      loadError={
        response.error && !response.data ? (
          <LoadErrorAlert error={loadError} actionLayout="responsive" />
        ) : null
      }
      isSending={isSending}
      onNotify={(notice) => void notify(notice)}
    />
  );
}
