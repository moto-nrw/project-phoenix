"use client";

/**
 * Elternhinweise zu Schul-Schließtagen in gebuchten Ferienwochen (ADR 0196),
 * an beiden Stellen gleich: im Kalender der Schule und auf der Seite
 * „Schließtage“ des Träger-Büros. Jede Zeile nennt Grund, Tage, Stand und,
 * solange der Hinweis auf einen Klick wartet, die Zahl der Familien. Ein
 * Klick auf „Eltern informieren“ beauftragt den Versand ohne zweite Frage;
 * die Zeile zeigt sofort „Wird gesendet“. Erledigte Hinweise liegen in einer
 * zugeklappten Karte darunter. Gescheiterte und unklare Sendungen bleiben
 * oben sichtbar.
 */

import { type ReactNode, useCallback, useId, useState } from "react";
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
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { holidayWeeksConcept } from "~/lib/concepts/holidayWeeks";
import {
  type HolidayClosureNotice,
  isNoticeDone,
} from "~/lib/holiday-closure-notices";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "HolidayClosureNoticeList" });

/** Offene Zeilen vor „N weitere anzeigen“. */
const INITIAL_OPEN_COUNT = 5;

/** „1 Familie hat …“ oder „12 Familien haben …“. */
function familiesBookedSentence(families: number): string {
  return families === 1
    ? "1 Familie hat an diesen Tagen Ferienbetreuung gebucht."
    : `${families} Familien haben an diesen Tagen Ferienbetreuung gebucht.`;
}

interface NoticeStatusView {
  /** Kein Etikett beim Vorschlag: Der Satz sagt es schon. */
  readonly badge: {
    readonly label: string;
    readonly tone: StatusBadgeTone;
  } | null;
  readonly hint: string | null;
}

/** Die Texte des Stands aus ADR 0196, Abschnitt 5. */
function noticeStatusView(
  notice: Pick<HolidayClosureNotice, "status" | "changed">,
): NoticeStatusView {
  switch (notice.status) {
    case "suggested":
      return notice.changed
        ? {
            badge: { label: "Geändert", tone: "warning" },
            hint: "Der Schließtag wurde geändert. Bitte prüfen Sie den Hinweis.",
          }
        : { badge: null, hint: "Die Eltern wurden noch nicht informiert." };
    case "sending":
      return {
        badge: { label: "Wird gesendet", tone: "blue" },
        hint: "Bitte warten Sie.",
      };
    case "no_mail":
      return {
        badge: { label: "Keine E-Mail nötig", tone: "gray" },
        hint: null,
      };
    case "sent":
      return {
        badge: { label: "Gesendet", tone: "green" },
        hint: null,
      };
    case "failed":
      return {
        badge: { label: "Nicht angekommen", tone: "red" },
        hint: "Bitte versuchen Sie es erneut.",
      };
    case "unknown":
      return {
        badge: { label: "Ergebnis unbekannt", tone: "warning" },
        hint: "Bitte lassen Sie den Versand vom moto-Team prüfen.",
      };
  }
}

/**
 * Die Tage des Hinweises: neu geschlossen, vorher geschlossen oder gelöscht.
 * Ein verschobener Schließtag nennt beide Zeiträume.
 */
function NoticeRanges({ notice }: Readonly<{ notice: HolidayClosureNotice }>) {
  const { current, previous } = notice;
  return (
    <ul className="mt-1 space-y-0.5 text-sm text-gray-700">
      {current ? (
        <li className="flex items-start gap-1.5">
          <CalendarX
            className="mt-0.5 h-4 w-4 shrink-0 text-gray-500"
            aria-hidden="true"
          />
          <span className="min-w-0 tabular-nums">
            {previous ? "Neu" : "Geschlossen"}: {formatClosingRange(current)}
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
  );
}

/** Ein Hinweis, wie die Liste ihn zeigt: während des Klicks „Wird gesendet“. */
function shownNotice<T extends HolidayClosureNotice>(
  notice: T,
  sending: boolean,
): T {
  return sending
    ? { ...notice, status: "sending", canNotify: false, changed: false }
    : notice;
}

function sendingToast(families: number): string {
  if (families === 1) return "Die E-Mail an 1 Familie wird gesendet.";
  if (families > 1)
    return `Die E-Mails an ${families} Familien werden gesendet.`;
  return "Die E-Mails werden gesendet.";
}

/**
 * Der Klick auf „Eltern informieren“: Die Zeile steht sofort auf „Wird
 * gesendet“, danach lädt die Liste den Stand vom Server. Hat sich der
 * Hinweis inzwischen geändert, sagt das die Meldung, und die Liste zeigt den
 * neuen Stand für einen erneuten Klick.
 */
export function useNoticeNotify(
  notify: (
    id: string,
    revision: number,
    token: string,
  ) => Promise<HolidayClosureNotice>,
  reload: () => Promise<unknown>,
) {
  const [sending, setSending] = useState<ReadonlySet<string>>(new Set());
  const { show: showError, showMessage } = useApiErrorDisplay();
  const { success, info } = useToast();

  const run = useCallback(
    async (notice: HolidayClosureNotice) => {
      setSending((current) => new Set(current).add(notice.id));
      try {
        const result = await notify(notice.id, notice.revision, notice.token);
        switch (result.status) {
          case "sent":
            success("Die E-Mails wurden gesendet.");
            break;
          case "no_mail":
            info(
              "Keine E-Mail nötig. Es gibt keine neuen Änderungen für die Eltern.",
            );
            break;
          case "failed":
            showMessage(
              "Der Versand hat nicht geklappt. Bitte versuchen Sie es erneut.",
            );
            break;
          case "unknown":
            info(
              "Ergebnis unbekannt. Bitte lassen Sie den Versand vom moto-Team prüfen.",
            );
            break;
          case "suggested":
            info("Der Hinweis wurde geändert. Bitte prüfen Sie ihn erneut.");
            break;
          case "sending":
            success(sendingToast(notice.families));
            break;
        }
      } catch (err) {
        logger.error("holiday_closure_notice_notify_failed", {
          notice_id: notice.id,
          status: err instanceof ApiError ? err.status : undefined,
          error: err instanceof Error ? err.message : String(err),
        });
        void showError(err, { object: "die E-Mail an die Eltern" });
      } finally {
        // Bewusst still: Scheitert nur das Nachladen, zeigt die Liste den
        // alten Stand; das nächste Laden der Seite holt den neuen.
        await reload().catch((err: unknown) => {
          logger.warn("holiday_closure_notices_reload_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
        });
        setSending((current) => {
          const next = new Set(current);
          next.delete(notice.id);
          return next;
        });
      }
    },
    [notify, reload, showError, showMessage, success, info],
  );

  return { isSending: (id: string) => sending.has(id), notify: run };
}

export interface HolidayClosureNoticeRowContent<T> {
  readonly notice: T;
  /** Überschrift der Zeile, etwa „Pädagogischer Tag“ oder Schule und Grund. */
  readonly title: string;
  /** Weitere Angaben unter dem Stand, etwa die Wochen im Träger-Büro. */
  readonly details?: ReactNode;
  /** Kurze Zahlen neben dem Stand, etwa „5 Kinder“. */
  readonly badges?: ReactNode;
}

function NoticeRow<T extends HolidayClosureNotice>({
  row,
  sending,
  onNotify,
  canAct,
}: Readonly<{
  row: HolidayClosureNoticeRowContent<T>;
  sending: boolean;
  onNotify: (notice: T) => void;
  canAct: boolean;
}>) {
  const notice = shownNotice(row.notice, sending);
  const status = noticeStatusView(notice);
  const showFamilies = notice.status === "suggested" && notice.families > 0;
  return (
    <li className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-start">
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
        <NoticeRanges notice={notice} />
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
      {canAct && notice.canNotify ? (
        <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
          <Button
            type="button"
            variant="surface"
            size="compact"
            aria-label={`Eltern informieren: ${row.title}`}
            onClick={() => onNotify(row.notice)}
          >
            <Mail className="h-4 w-4" aria-hidden="true" />
            Eltern informieren
          </Button>
        </div>
      ) : null}
    </li>
  );
}

/**
 * Die offenen Hinweise in einer Karte, die erledigten zugeklappt darunter.
 * Ohne Hinweise zeigt die Liste nichts.
 */
export function HolidayClosureNoticeList<T extends HolidayClosureNotice>({
  rows,
  title,
  description,
  action,
  intro,
  loadError,
  isSending,
  onNotify,
  canAct = true,
}: Readonly<{
  rows: readonly HolidayClosureNoticeRowContent<T>[];
  title: string;
  description: string;
  /** Eine Aktion für alle Zeilen im Kartenkopf. */
  action?: ReactNode;
  /** Text, der für alle Zeilen gilt; steht einmal über der Liste. */
  intro?: ReactNode;
  /** Fehler beim Laden, an der Stelle der Liste. */
  loadError?: ReactNode;
  isSending: (id: string) => boolean;
  onNotify: (notice: T) => void;
  canAct?: boolean;
}>) {
  const [expanded, setExpanded] = useState(false);
  const openListId = useId();
  const open = rows.filter(
    (row) => !isNoticeDone(shownNotice(row.notice, isSending(row.notice.id))),
  );
  const done = rows.filter((row) => !open.includes(row));

  if (loadError) {
    return (
      <SectionCard title={title} concept={holidayWeeksConcept}>
        {loadError}
      </SectionCard>
    );
  }
  if (rows.length === 0) return null;

  const hiddenCount = Math.max(0, open.length - INITIAL_OPEN_COUNT);
  const visible = expanded ? open : open.slice(0, INITIAL_OPEN_COUNT);
  const waiting = open.filter((row) => row.notice.canNotify).length;

  const renderRow = (row: HolidayClosureNoticeRowContent<T>) => (
    <NoticeRow
      key={row.notice.id}
      row={row}
      sending={isSending(row.notice.id)}
      onNotify={onNotify}
      canAct={canAct}
    />
  );

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
          action={action}
        >
          {intro ? <div className="mb-3">{intro}</div> : null}
          <ResponsiveActionList id={openListId}>
            {visible.map(renderRow)}
          </ResponsiveActionList>
          {hiddenCount > 0 ? (
            <Button
              type="button"
              variant="ghost"
              size="md"
              className="mt-2 w-full"
              aria-expanded={expanded}
              aria-controls={openListId}
              onClick={() => setExpanded((current) => !current)}
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
