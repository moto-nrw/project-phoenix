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

function readSeenDay(): string | null {
  try {
    return globalThis.localStorage.getItem(SEEN_KEY);
  } catch {
    return null;
  }
}

function saveSeenDay(day: string) {
  try {
    globalThis.localStorage.setItem(SEEN_KEY, day);
  } catch {
    // Ohne Speicher erscheint der Hinweis beim nächsten Laden wieder.
  }
}

export function isWeekendDay(isoDay: string): boolean {
  const weekday = parseISODate(isoDay).getDay();
  return weekday === 0 || weekday === 6;
}

/**
 * Hinweis der öffentlichen Demo am Wochenende (#3894). Die Demo spielt auch
 * samstags und sonntags einen Betreuungstag, damit es etwas zu sehen gibt.
 * Geplante Zeiten gibt es aber nur von Montag bis Freitag, also zeigt die App
 * jedes anwesende Kind als „Ungeplant anwesend". Ohne Hinweis sieht das wie
 * ein Fehler aus.
 */
export function DemoWeekendNotice({ inParentsApp }: { inParentsApp: boolean }) {
  const today = useBerlinToday();
  const weekend = isWeekendDay(today);
  const [open, setOpen] = useState(false);
  const [settled, setSettled] = useState(false);
  const { isModalOpen } = useModal();

  useEffect(() => {
    const timer = setTimeout(() => setSettled(true), SETTLE_MS);
    return () => clearTimeout(timer);
  }, []);

  useEffect(() => {
    if (weekend && settled && !isModalOpen && readSeenDay() !== today) {
      setOpen(true);
    }
  }, [weekend, settled, isModalOpen, today]);

  if (!weekend) return null;

  const close = () => {
    saveSeenDay(today);
    setOpen(false);
  };

  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="compact"
        className="shrink-0 text-sm"
        aria-label="Hinweis zum Wochenende"
        onClick={() => {
          if (!isModalOpen) {
            setOpen(true);
          }
        }}
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
          {inParentsApp ? (
            <p>Deshalb ist Ihr Kind heute in der OGS.</p>
          ) : (
            <p>
              Geplante Zeiten gibt es nur von Montag bis Freitag. Deshalb steht
              bei den Kindern heute „Ungeplant anwesend“.
            </p>
          )}
          <p>An einem Werktag sehen Sie die Demo wie im echten Alltag.</p>
        </div>
      </Modal>
    </>
  );
}
