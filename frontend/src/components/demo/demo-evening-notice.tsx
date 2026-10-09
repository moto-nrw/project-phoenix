"use client";

import { ClockIcon } from "@phosphor-icons/react";
import { Button } from "~/components/ui/button";
import { Modal } from "~/components/ui/modal";
import { berlinClockFromISO } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useCurrentTimestamp } from "~/lib/hooks/use-current-timestamp";
import { isWeekendDay, useDemoDayNotice } from "./demo-weekend-notice";

// Pro Tag einmal von selbst; danach öffnet der Knopf im Streifen ihn.
const SEEN_KEY = "moto-demo-evening-notice";

// Der Schultag der Demo reicht von 7 bis 17 Uhr. Außerhalb davon verschiebt
// die Demo den Tag an Werktagen zur aktuellen Uhrzeit (#3921).
const DAY_START_HOUR = 7;
const DAY_END_HOUR = 17;

/** Ob die Berliner Uhrzeit außerhalb des Schultags der Demo liegt. */
export function isOutsideSchoolDay(at: Date): boolean {
  const hour = Number(berlinClockFromISO(at.toISOString()).slice(0, 2));
  return hour < DAY_START_HOUR || hour >= DAY_END_HOUR;
}

/**
 * Hinweis der öffentlichen Demo abends und nachts an Werktagen (#3921). Die
 * Demo legt den Tag dann zur aktuellen Uhrzeit, damit es etwas zu sehen gibt.
 * Blöcke und Abholzeiten stehen deshalb zu ungewohnten Uhrzeiten. Ohne
 * Hinweis sieht das wie ein Fehler aus.
 */
export function DemoEveningNotice({
  inParentsApp,
}: Readonly<{ inParentsApp: boolean }>) {
  const today = useBerlinToday();
  const timestamp = useCurrentTimestamp();
  // Vor dem ersten Takt der Uhr (Server, Hydrierung) gilt der Hinweis nicht.
  const active =
    timestamp > 0 &&
    !isWeekendDay(today) &&
    isOutsideSchoolDay(new Date(timestamp));
  const { open, show, close } = useDemoDayNotice(active, SEEN_KEY, today);

  if (!active) return null;

  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="compact"
        className="shrink-0 text-sm"
        aria-label="Hinweis zu den Uhrzeiten"
        onClick={show}
      >
        <ClockIcon aria-hidden="true" className="size-4" />
        <span className="hidden sm:inline">Uhrzeiten</span>
      </Button>
      <Modal
        isOpen={open}
        onClose={close}
        title="Der Demo-Tag läuft zu Ihrer Uhrzeit"
        footer={
          <Button type="button" variant="primary" size="md" onClick={close}>
            Verstanden
          </Button>
        }
      >
        <div className="flex flex-col gap-3 text-sm text-gray-700">
          <p>Um diese Zeit ist in einer echten OGS niemand mehr da.</p>
          <p>
            Damit Sie trotzdem etwas sehen, legt die Demo den Tag auf jetzt.
          </p>
          {inParentsApp ? (
            <p>Deshalb hat Ihr Kind heute ungewohnte Abholzeiten.</p>
          ) : (
            <p>
              Deshalb stehen Blöcke und Abholzeiten heute zu ungewohnten
              Uhrzeiten.
            </p>
          )}
          <p>Tagsüber sehen Sie die Demo wie im echten Alltag.</p>
        </div>
      </Modal>
    </>
  );
}
