"use client";

/**
 * „Eltern noch nicht informiert“ im Träger-Büro (ADR 0196): je Schul-
 * Schließtag in einer gebuchten Ferienwoche eine Zeile mit Schule, Grund,
 * Tagen, Stand, Kindern und Familien und den betroffenen Wochen mit den
 * geschlossenen und wieder geöffneten Tagen. „Eltern informieren“ beauftragt
 * den Hinweis einer Zeile, „Alle Eltern informieren“ alle Vorschläge der
 * Liste, beides mit einem Klick. Denselben Auftrag kann die Schule auslösen;
 * ein zweiter Klick sendet nichts doppelt.
 */

import { useCallback, useState } from "react";
import { Mail } from "lucide-react";

import {
  HolidayClosureNoticeList,
  type HolidayClosureNoticeRowContent,
  useNoticeNotify,
} from "~/components/planning/holiday-closure-notice-list";
import { formatDate } from "~/lib/date-helpers";
import {
  childrenLabel,
  familiesLabel,
  HolidayClosureConsequences,
  HolidayClosureDayLines,
} from "~/components/traeger/closing-day-holiday-impact";
import { Button } from "~/components/ui/button";
import type { FormErrorInput } from "~/components/ui/form-error";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  notifyPendingHolidayClosures,
  notifyTraegerHolidayClosureNotice,
  type PendingHolidayClosures,
  type TraegerHolidayClosureNotice,
} from "~/lib/traeger-closing-days";

const logger = createLogger({ component: "TraegerHolidayClosureNotices" });

function NoticeWeeks({
  notice,
}: Readonly<{ notice: TraegerHolidayClosureNotice }>) {
  if (notice.weeks.length === 0) return null;
  return (
    <ul className="mt-2 space-y-1.5">
      {notice.weeks.map((week) => (
        <li key={week.weekId} className="space-y-0.5">
          <p className="text-sm text-gray-600 tabular-nums">
            <span className="font-medium text-gray-900">{week.periodName}</span>
            {" · "}
            {formatDate(week.startsOn)} bis {formatDate(week.endsOn)}
          </p>
          <HolidayClosureDayLines week={week} />
        </li>
      ))}
    </ul>
  );
}

export function TraegerHolidayClosureNotices({
  pending,
  loadError,
  schoolName,
  reload,
}: Readonly<{
  pending: PendingHolidayClosures | undefined;
  loadError: FormErrorInput;
  schoolName: (id: string) => string;
  reload: () => Promise<unknown>;
}>) {
  const { isSending, notify } = useNoticeNotify(
    notifyTraegerHolidayClosureNotice,
    reload,
  );
  const [notifyingAll, setNotifyingAll] = useState(false);
  const { show: showError } = useApiErrorDisplay();
  const { success } = useToast();

  const notices = pending?.notices ?? [];
  const suggested = notices.filter(
    (notice) => notice.status === "suggested" && !isSending(notice.id),
  );

  const notifyAll = useCallback(async () => {
    if (!pending) return;
    setNotifyingAll(true);
    try {
      const families = await notifyPendingHolidayClosures(pending.token);
      success(
        `${familiesLabel(families)} ${families === 1 ? "bekommt" : "bekommen"} eine E-Mail.`,
      );
    } catch (err) {
      logger.error("traeger_holiday_closure_notify_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showError(err, { object: "die E-Mail an die Eltern" });
    } finally {
      setNotifyingAll(false);
      // Bewusst still: Scheitert nur das Nachladen, bleibt der alte Stand
      // stehen, bis die Seite wieder lädt.
      await reload().catch((err: unknown) => {
        logger.warn("traeger_holiday_closures_reload_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    }
  }, [pending, reload, showError, success]);

  const rows: HolidayClosureNoticeRowContent<TraegerHolidayClosureNotice>[] =
    notices.map((notice) => {
      const waiting = notice.status === "suggested" && !isSending(notice.id);
      return {
        notice,
        title: `${schoolName(notice.schoolId)}: „${notice.reason}“`,
        badges:
          waiting && notice.children > 0 ? (
            <StatusBadge
              label={childrenLabel(notice.children)}
              tone="blue"
              size="xs"
            />
          ) : null,
        details: waiting ? <NoticeWeeks notice={notice} /> : null,
      };
    });

  return (
    <HolidayClosureNoticeList
      title="Eltern noch nicht informiert"
      description="Diese Schließtage haben Schulen selbst eingetragen. Sie liegen in gebuchten Ferienwochen. Die Eltern bekommen erst eine E-Mail, wenn Sie oder die Schule „Eltern informieren“ wählen."
      rows={rows}
      intro={
        suggested.length > 0 ? (
          <HolidayClosureConsequences
            keepsContribution={pending?.keepsContribution ?? true}
          />
        ) : null
      }
      action={
        suggested.length > 1 ? (
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void notifyAll()}
            isLoading={notifyingAll}
            loadingText="Wird gesendet..."
          >
            <Mail className="h-4 w-4" aria-hidden="true" />
            Alle Eltern informieren
          </Button>
        ) : null
      }
      loadError={
        loadError ? (
          <LoadErrorAlert error={loadError} actionLayout="responsive" />
        ) : null
      }
      isSending={isSending}
      onNotify={(notice) => void notify(notice)}
    />
  );
}
