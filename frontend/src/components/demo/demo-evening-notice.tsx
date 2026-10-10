"use client";

import { ClockIcon } from "@phosphor-icons/react";
import { Button } from "~/components/ui/button";
import { Modal } from "~/components/ui/modal";
import { berlinClockFromISO } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useCurrentTimestamp } from "~/lib/hooks/use-current-timestamp";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { isWeekendDay, useDemoDayNotice } from "./demo-weekend-notice";

// Pro Tag einmal von selbst; danach öffnet „Warum?“ in der Zeile ihn.
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
 * Ob die Demo den heutigen Tag gerade zur Uhrzeit verschiebt: an einem
 * Werktag abends oder nachts. Vor dem ersten Takt der Uhr (Server,
 * Hydrierung) gilt das nicht, damit Server und Browser gleich rendern.
 * Die Shells rücken um die Hinweiszeile nach unten, solange es gilt.
 */
export function useDemoEveningActive(): boolean {
  const today = useBerlinToday();
  const timestamp = useCurrentTimestamp();
  return (
    timestamp > 0 &&
    !isWeekendDay(today) &&
    isOutsideSchoolDay(new Date(timestamp))
  );
}

/**
 * Hinweiszeile der öffentlichen Demo abends und nachts an Werktagen (#3921),
 * fest unter dem Demo-Streifen. Die Demo legt den Tag dann zur aktuellen
 * Uhrzeit, damit es etwas zu sehen gibt. Blöcke und Abholzeiten stehen
 * deshalb zu ungewohnten Uhrzeiten; ohne sichtbaren Hinweis sieht das wie
 * ein Fehler aus. Die Zeile bleibt stehen, der Dialog erklärt mehr.
 */
export function DemoEveningRow({
  inParentsApp,
}: Readonly<{ inParentsApp: boolean }>) {
  const today = useBerlinToday();
  const active = useDemoEveningActive();
  const { open, show, close } = useDemoDayNotice(active, SEEN_KEY, today);

  if (!active) return null;

  return (
    <div
      role="note"
      className="fixed inset-x-0 top-12 z-50 flex h-8 items-center gap-2 border-b border-gray-200 bg-gray-50 px-4 text-xs text-gray-900"
    >
      <ClockIcon
        aria-hidden="true"
        className="size-4 shrink-0"
        style={{ color: LOCATION_COLORS.OTHER_ROOM }}
      />
      <span className="min-w-0 truncate font-medium sm:hidden">
        Uhrzeiten heute verschoben.
      </span>
      <span className="hidden min-w-0 truncate font-medium sm:inline">
        Die Demo läuft zu Ihrer Uhrzeit. Blöcke und Abholzeiten sind deshalb
        verschoben.
      </span>
      <Button
        type="button"
        variant="ghost"
        size="compact"
        className="ml-auto h-6 shrink-0 text-xs underline underline-offset-2"
        onClick={show}
      >
        Warum?
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
            Damit Sie trotzdem etwas sehen, legt die Demo den Nachmittag auf
            jetzt.
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
    </div>
  );
}
