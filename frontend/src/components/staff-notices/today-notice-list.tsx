"use client";

import { useLayoutEffect, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  acknowledgeStaffNotice,
  describeRecurrence,
  STAFF_NOTICES_REFRESH_EVENT,
} from "~/lib/staff-notices-api";
import type { StaffNotice } from "~/lib/staff-notices-api";

const logger = createLogger({ component: "TodayNoticeList" });

/**
 * Die Hinweise der Leitung, die HEUTE gelten — als Texte, nicht als Zeilen:
 * der Hinweis IST der Inhalt. Wichtige stehen zuerst (Sortierung kommt aus dem
 * Backend) und tragen ein Kennzeichen. Bestätigt wird der Hinweis, nicht der
 * Tag: ein wiederkehrender Hinweis fragt nicht jeden Dienstag erneut.
 *
 * Dieselbe Liste steht in moto schule (#2208); dort reicht die Seite die
 * Kenntnisnahme über ihre eigene Sitzung herein (`acknowledge`).
 */
export function TodayNoticeList({
  notices,
  onChanged,
  acknowledge = acknowledgeStaffNotice,
}: {
  readonly notices: readonly StaffNotice[];
  readonly onChanged: () => Promise<unknown>;
  readonly acknowledge?: (id: string) => Promise<void>;
}) {
  const [pending, setPending] = useState<string | null>(null);
  // Eine Kenntnisnahme ist eine Aktion ohne Formular: der Fehler kommt als
  // Meldung mit Wiederholen (#2517).
  const { show: showError } = useApiErrorDisplay();
  const latestConfirmRef = useRef<(notice: StaffNotice) => void>(
    () => undefined,
  );

  const confirm = async (notice: StaffNotice) => {
    setPending(notice.id);
    try {
      await acknowledge(notice.id);
      await onChanged();
      window.dispatchEvent(new Event(STAFF_NOTICES_REFRESH_EVENT));
    } catch (err) {
      logger.error("staff_notice_acknowledge_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showError(err, {
        object: "die Kenntnisnahme",
        retry: () => latestConfirmRef.current(notice),
      });
    } finally {
      setPending(null);
    }
  };
  useLayoutEffect(() => {
    latestConfirmRef.current = (notice) => void confirm(notice);
  });

  return (
    <>
      <ul className="space-y-4">
        {notices.map((notice) => (
          <li key={notice.id}>
            <div className="flex flex-wrap items-center gap-2">
              <h3 className="text-[15px] font-semibold text-gray-900">
                {notice.title}
              </h3>
              {notice.priority === "important" && (
                <StatusBadge label="Wichtig" tone="orange" />
              )}
              <span className="text-xs text-gray-500">
                {describeRecurrence(notice)}
              </span>
            </div>

            {notice.body && (
              <p className="mt-1 max-w-3xl text-sm leading-6 whitespace-pre-line text-gray-600">
                {notice.body}
              </p>
            )}

            {notice.requires_acknowledgement && (
              <div className="mt-2">
                {notice.acknowledged_at ? (
                  <StatusBadge label="Zur Kenntnis genommen" tone="green" />
                ) : (
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    disabled={pending === notice.id}
                    onClick={() => void confirm(notice)}
                  >
                    {pending === notice.id
                      ? "Wird gespeichert …"
                      : "Zur Kenntnis nehmen"}
                  </Button>
                )}
              </div>
            )}
          </li>
        ))}
      </ul>
    </>
  );
}
