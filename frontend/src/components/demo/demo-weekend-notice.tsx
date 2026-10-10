"use client";

import { useEffect, useState } from "react";
import { CalendarBlankIcon } from "@phosphor-icons/react";
import { useModal } from "~/components/dashboard/modal-context";
import { Button } from "~/components/ui/button";
import { Modal } from "~/components/ui/modal";
import { parseISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";

// Pro Wochenendtag einmal von selbst; danach öffnet der Knopf im Streifen ihn.
const SEEN_KEY = "moto-demo-weekend-notice";

// Andere Dialoge der ersten Seite (etwa die Benachrichtigungs-Einrichtung)
// laden ihren Code erst nach. Der Hinweis wartet kurz und dann, bis sie zu
// sind, damit nie zwei Dialoge übereinander liegen.
const SETTLE_MS = 1500;

function readSeenDay(key: string): string | null {
  try {
    return globalThis.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function saveSeenDay(key: string, day: string) {
  try {
    globalThis.localStorage.setItem(key, day);
  } catch {
    // Ohne Speicher erscheint der Hinweis beim nächsten Laden wieder.
  }
}

/**
 * Öffnet einen Demo-Hinweis einmal pro Tag von selbst, sobald er gilt und kein
 * anderer Dialog offen ist. Danach öffnet ihn nur noch der Knopf im Streifen.
 */
export function useDemoDayNotice(
  active: boolean,
  seenKey: string,
  today: string,
) {
  const [open, setOpen] = useState(false);
  const [settled, setSettled] = useState(false);
  const { isModalOpen } = useModal();

  useEffect(() => {
    const timer = setTimeout(() => setSettled(true), SETTLE_MS);
    return () => clearTimeout(timer);
  }, []);

  useEffect(() => {
    if (active && settled && !isModalOpen && readSeenDay(seenKey) !== today) {
      setOpen(true);
    }
  }, [active, settled, isModalOpen, seenKey, today]);

  return {
    open,
    show: () => {
      if (!isModalOpen) {
        setOpen(true);
      }
    },
    close: () => {
      saveSeenDay(seenKey, today);
      setOpen(false);
    },
  };
}

export function isWeekendDay(isoDay: string): boolean {
  const weekday = parseISODate(isoDay).getDay();
  return weekday === 0 || weekday === 6;
}

/**
 * Hinweis der öffentlichen Demo am Wochenende (#3894). Eine Demo-Schule
 * betreut samstags und sonntags nach dem Plan vom Freitag
 * (operations.weekend_follows_friday, #3921), damit es etwas zu sehen gibt.
 * Eine echte OGS hat am Wochenende zu; ohne Hinweis sähe der volle Tag wie
 * ein Fehler aus.
 */
export function DemoWeekendNotice({ inParentsApp }: { inParentsApp: boolean }) {
  const today = useBerlinToday();
  const weekend = isWeekendDay(today);
  const { open, show, close } = useDemoDayNotice(weekend, SEEN_KEY, today);

  if (!weekend) return null;

  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="compact"
        className="shrink-0 text-sm"
        aria-label="Hinweis zum Wochenende"
        onClick={show}
      >
        <CalendarBlankIcon aria-hidden="true" className="size-4" />
        <span className="hidden sm:inline">Wochenende</span>
      </Button>
      <Modal
        isOpen={open}
        onClose={close}
        title="Heute ist Wochenende"
        footer={
          <Button type="button" variant="primary" size="md" onClick={close}>
            Verstanden
          </Button>
        }
      >
        <div className="flex flex-col gap-3 text-sm text-gray-700">
          <p>Die Demo zeigt trotzdem einen normalen Betreuungstag.</p>
          <p>Am Wochenende gilt in der Demo der Plan vom Freitag.</p>
          {inParentsApp ? <p>Deshalb ist Ihr Kind heute in der OGS.</p> : null}
          <p>An einem Werktag sehen Sie die Demo wie im echten Alltag.</p>
        </div>
      </Modal>
    </>
  );
}
