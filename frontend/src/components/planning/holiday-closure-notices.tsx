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
} from "~/components/planning/use-holiday-closure-notices";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { hasPermission } from "~/lib/auth-utils";
import {
  type HolidayClosureNotice,
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

  const rows = (response.data ?? []).map((notice) => ({
    notice,
    title: `„${notice.reason}“`,
  }));
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
  const title = "Eltern der Ferienbetreuung informieren";
  const description =
    "Diese Schließtage liegen in gebuchten Ferienwochen. Die Eltern bekommen erst eine E-Mail, wenn Sie oder das Träger-Büro „Eltern informieren“ wählen.";
  const renderRow = (
    row: HolidayClosureNoticeRowContent<HolidayClosureNotice>,
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
  if (!canNotify) return null;
  if (response.error && !response.data)
    return (
      <SectionCard title={title} concept={holidayWeeksConcept}>
        <LoadErrorAlert error={loadError} actionLayout="responsive" />
      </SectionCard>
    );
  if (rows.length === 0) return null;
  return (
    <div className="flex flex-col gap-3">
      {open.length > 0 ? (
        <SectionCard
          title={title}
          concept={holidayWeeksConcept}
          titleBadge={
            waiting > 0 ? (
              <StatusBadge tone="warning" label={`${waiting} offen`} />
            ) : null
          }
          description={description}
        >
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
