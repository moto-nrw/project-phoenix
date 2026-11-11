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

import {
  CalendarCheck,
  CalendarX,
  ChevronDown,
  ChevronUp,
  Mail,
} from "lucide-react";
import { formatClosingRange } from "~/components/traeger/closing-day-format";
import { Button } from "~/components/ui/button";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { holidayWeeksConcept } from "~/lib/concepts/holidayWeeks";
import {
  familiesBookedSentence,
  noticeStatusView,
  shownNotice,
  useNoticeList,
  useNoticeNotify,
  type HolidayClosureNoticeRowContent,
} from "~/lib/hooks/use-holiday-closure-notices";
import {
  childrenLabel,
  familiesLabel,
  HolidayClosureConsequences,
  HolidayClosureWeekList,
} from "~/components/traeger/closing-day-holiday-impact";
import type { FormErrorInput } from "~/components/ui/form-error";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  notifyPendingHolidayClosures,
  notifyTraegerHolidayClosureNotice,
  type PendingHolidayClosures,
  type TraegerHolidayClosureNotice,
} from "~/lib/traeger-closing-days";

const logger = createLogger({ component: "TraegerHolidayClosureNotices" });

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

  const retryableMails = pending?.retryableMails ?? 0;
  const unknownMails = pending?.unknownMails ?? 0;
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
        retryableMails > 0 && suggested.length === 0
          ? "Erneuter Versand ist beauftragt."
          : `${familiesLabel(families)} ${families === 1 ? "bekommt" : "bekommen"} eine E-Mail.`,
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
  }, [pending, reload, retryableMails, showError, success, suggested.length]);

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
        details:
          waiting && notice.weeks.length > 0 ? (
            <div className="mt-2">
              <HolidayClosureWeekList
                weeks={notice.weeks}
                schoolName={schoolName}
              />
            </div>
          ) : null,
      };
    });

  const {
    open,
    done,
    visible,
    hiddenCount,
    waiting,
    expanded,
    openListId,
    toggleExpanded,
  } = useNoticeList(rows, isSending);
  const title = "Eltern noch nicht informiert";
  const description =
    "Schulhinweise brauchen einen Klick von Ihnen oder der Schule. Fehlgeschlagene E-Mails können Sie erneut senden.";
  const intro =
    suggested.length > 0 || retryableMails > 0 || unknownMails > 0 ? (
      <div className="space-y-2 text-sm">
        {retryableMails > 0 ? (
          <p>
            {retryableMails === 1
              ? "1 E-Mail ist nicht angekommen."
              : `${retryableMails} E-Mails sind nicht angekommen.`}{" "}
            Ein Klick versucht den Versand erneut.
          </p>
        ) : null}
        {unknownMails > 0 ? (
          <p>
            Bei {unknownMails} E-Mail{unknownMails === 1 ? "" : "s"} ist das
            Ergebnis unbekannt. Bitte lassen Sie den Versand vom moto-Team
            prüfen.
          </p>
        ) : null}
        {suggested.length > 0 ? (
          <HolidayClosureConsequences
            keepsContribution={pending?.keepsContribution ?? true}
          />
        ) : null}
      </div>
    ) : null;
  const action =
    suggested.length > 1 || retryableMails > 0 ? (
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={() => void notifyAll()}
        isLoading={notifyingAll}
        loadingText="Wird gesendet..."
      >
        <Mail className="h-4 w-4" aria-hidden="true" />
        {suggested.length > 0 ? "Alle Eltern informieren" : "Erneut senden"}
      </Button>
    ) : null;
  const renderRow = (
    row: HolidayClosureNoticeRowContent<TraegerHolidayClosureNotice>,
  ) => {
    const notice = shownNotice(row.notice, isSending(row.notice.id));
    const status = noticeStatusView(notice);
    const { current, previous } = notice;
    const showFamilies = notice.status === "suggested" && notice.families > 0;
    return (
      <li
        key={row.notice.id}
        className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-start"
      >
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <p className="text-sm font-semibold wrap-anywhere text-gray-900">
              {row.title}
            </p>
            {status.badge ? (
              <StatusBadge
                label={status.badge.label}
                tone={status.badge.tone}
                size="xs"
              />
            ) : null}
            {row.badges}
          </div>
          <ul className="mt-1 space-y-0.5 text-sm text-gray-700">
            {current ? (
              <li className="flex items-start gap-1.5">
                <CalendarX
                  className="mt-0.5 h-4 w-4 shrink-0 text-gray-500"
                  aria-hidden="true"
                />
                <span className="min-w-0 tabular-nums">
                  {previous ? "Neu" : "Geschlossen"}:{" "}
                  {formatClosingRange(current)}
                </span>
              </li>
            ) : null}
            {previous ? (
              <li className="flex items-start gap-1.5">
                <CalendarCheck
                  className="mt-0.5 h-4 w-4 shrink-0 text-gray-500"
                  aria-hidden="true"
                />
                <span className="min-w-0 tabular-nums">
                  {current
                    ? `Vorher: ${formatClosingRange(previous)}`
                    : `Schließtag gelöscht: ${formatClosingRange(previous)}.`}
                </span>
              </li>
            ) : null}
          </ul>
          {showFamilies ? (
            <p className="mt-1 flex items-start gap-1.5 text-sm text-gray-900">
              <Mail
                className="mt-0.5 h-4 w-4 shrink-0 text-gray-500"
                aria-hidden="true"
              />
              <span>{familiesBookedSentence(notice.families)}</span>
            </p>
          ) : null}
          {row.details}
          {status.hint ? (
            <p className="mt-1 text-sm text-gray-600">{status.hint}</p>
          ) : null}
        </div>
        {notice.canNotify ? (
          <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
            <Button
              type="button"
              variant="surface"
              size="compact"
              aria-label={`Eltern informieren: ${row.title}`}
              onClick={() => void notify(row.notice)}
            >
              <Mail className="h-4 w-4" aria-hidden="true" />
              Eltern informieren
            </Button>
          </div>
        ) : null}
      </li>
    );
  };
  if (loadError)
    return (
      <SectionCard title={title} concept={holidayWeeksConcept}>
        <LoadErrorAlert error={loadError} actionLayout="responsive" />
      </SectionCard>
    );
  if (rows.length === 0 && !intro && !action) return null;
  return (
    <div className="flex flex-col gap-3">
      {open.length > 0 || intro || action ? (
        <SectionCard
          title={title}
          concept={holidayWeeksConcept}
          titleBadge={
            waiting > 0 ? (
              <StatusBadge tone="warning" label={`${waiting} offen`} />
            ) : null
          }
          description={description}
          action={action}
        >
          {intro ? <div className="mb-3">{intro}</div> : null}
          {visible.length > 0 ? (
            <ResponsiveActionList id={openListId}>
              {visible.map(renderRow)}
            </ResponsiveActionList>
          ) : null}
          {hiddenCount > 0 ? (
            <Button
              type="button"
              variant="ghost"
              size="md"
              className="mt-2 w-full"
              aria-expanded={expanded}
              aria-controls={openListId}
              onClick={toggleExpanded}
            >
              {expanded ? (
                <>
                  Weniger anzeigen
                  <ChevronUp className="size-4" aria-hidden="true" />
                </>
              ) : (
                <>
                  {hiddenCount} weitere anzeigen
                  <ChevronDown className="size-4" aria-hidden="true" />
                </>
              )}
            </Button>
          ) : null}
        </SectionCard>
      ) : null}
      {done.length > 0 ? (
        <SectionCard
          title={`Erledigte Hinweise (${done.length})`}
          description="Für diese Schließtage ist nichts mehr zu tun."
          collapsible
          defaultCollapsed
          headingLevel={3}
        >
          <ResponsiveActionList>{done.map(renderRow)}</ResponsiveActionList>
        </SectionCard>
      ) : null}
    </div>
  );
}
