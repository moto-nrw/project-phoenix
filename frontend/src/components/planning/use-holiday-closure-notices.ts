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

import type { StatusBadgeTone } from "~/components/ui/status-badge";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import {
  type HolidayClosureNotice,
  isNoticeDone,
} from "~/lib/holiday-closure-notices";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "HolidayClosureNotices" });

/** Offene Zeilen vor „N weitere anzeigen“. */
const INITIAL_OPEN_COUNT = 5;

/** „1 Familie hat …“ oder „12 Familien haben …“. */
export function familiesBookedSentence(families: number): string {
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
export function noticeStatusView(
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

/** Ein Hinweis, wie die Liste ihn zeigt: während des Klicks „Wird gesendet“. */
export function shownNotice<T extends HolidayClosureNotice>(
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

export function useNoticeList<T extends HolidayClosureNotice>(
  rows: readonly HolidayClosureNoticeRowContent<T>[],
  isSending: (id: string) => boolean,
) {
  const [expanded, setExpanded] = useState(false);
  const openListId = useId();
  const open = rows.filter(
    (row) => !isNoticeDone(shownNotice(row.notice, isSending(row.notice.id))),
  );
  const done = rows.filter((row) => !open.includes(row));
  return {
    open,
    done,
    expanded,
    openListId,
    hiddenCount: Math.max(0, open.length - INITIAL_OPEN_COUNT),
    visible: expanded ? open : open.slice(0, INITIAL_OPEN_COUNT),
    waiting: open.filter((row) => row.notice.canNotify).length,
    toggleExpanded: () => setExpanded((current) => !current),
  };
}
