"use client";

// Der Ferienwunsch eines angemeldeten Kindes (#4703): was die OGS-Anmeldung
// dazu sagt, für das Träger-Büro. Nur das Büro sieht ihn; Eltern, Bestätigung
// und die ausrichtende OGS sehen ihn nicht.

import { CircleHelp, Sun, SunDim } from "lucide-react";
import { useState } from "react";
import { Button } from "~/components/ui/button";
import { InfoSection } from "~/components/ui/detail-modal-components";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type HolidayRegistrationChild,
  type HolidayRegistrationDetail,
  type HolidayWishFilter,
  setHolidayChildWish,
} from "~/lib/traeger-holiday-registrations-api";

const logger = createLogger({ component: "HolidayWish" });

interface WishView {
  readonly tone: StatusBadgeTone;
  readonly label: string;
  readonly icon?: typeof Sun | typeof CircleHelp;
}

/**
 * Die Plakette des Ferienwunschs; null für einen Feriengast und für einen
 * Zeitraum, der den Ferienwunsch nicht berücksichtigt.
 */
export function holidayWishView(
  child: HolidayRegistrationChild,
  wishRule: boolean,
): WishView | null {
  if (!wishRule || child.homeSchoolId === null) return null;
  if (child.holidayWishCheck) {
    // A task of the office, not a warning: blue keeps it apart from the
    // yellow hint of a possible duplicate in the same row.
    return { tone: "blue", label: "Ferienwunsch prüfen", icon: CircleHelp };
  }
  if (child.holidayWish === "yes") {
    return { tone: "green", label: "Mit Ferienwunsch", icon: Sun };
  }
  if (child.holidayWish === "no") {
    return { tone: "gray", label: "Ohne Ferienwunsch", icon: SunDim };
  }
  return null;
}

/** Woher der bekannte Ferienwunsch stammt, in einem Satz. */
function holidayWishSourceText(child: HolidayRegistrationChild): string {
  if (child.holidayWishCheck) {
    return "Der Ferienwunsch ist nicht bekannt. Bitte prüfen Sie ihn, zum Beispiel bei der OGS.";
  }
  if (child.holidayWishSource === "account") {
    return "Aus der OGS-Anmeldung, geprüft über das Elternkonto.";
  }
  if (child.holidayWishSource === "office") {
    return "Vom Träger-Büro festgelegt.";
  }
  return "";
}

export const HOLIDAY_WISH_FILTER_OPTIONS: readonly {
  value: HolidayWishFilter;
  label: string;
}[] = [
  { value: "", label: "Alle Ferienwünsche" },
  { value: "check", label: "Ferienwunsch prüfen" },
  { value: "yes", label: "Mit Ferienwunsch" },
  { value: "no", label: "Ohne Ferienwunsch" },
];

/**
 * Der Ferienwunsch in der Karte eines Kindes: was bekannt ist, woher, und die
 * Entscheidung des Büros. Die Entscheidung ordnet nur die Warteliste: Ein
 * Kind ohne Ferienwunsch wartet hinter den anderen, ein Platz bleibt.
 */
export function HolidayWishSection({
  periodId,
  registrationId,
  child,
  wishRule,
  cancelled,
  onChanged,
}: Readonly<{
  periodId: string;
  registrationId: string;
  child: HolidayRegistrationChild;
  wishRule: boolean;
  cancelled: boolean;
  onChanged: (registration: HolidayRegistrationDetail) => void;
}>) {
  const [saving, setSaving] = useState<"yes" | "no" | null>(null);
  const errors = useApiFormError();
  const view = holidayWishView(child, wishRule);
  if (!view) return null;

  const decide = async (wish: "yes" | "no") => {
    setSaving(wish);
    errors.clear();
    try {
      onChanged(
        await setHolidayChildWish(periodId, registrationId, child.id, wish),
      );
    } catch (err) {
      logger.error("holiday_child_wish_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void errors.show(err, { object: "der Ferienwunsch" });
    } finally {
      setSaving(null);
    }
  };
  const name = `${child.firstName} ${child.lastName}`;
  const choices: readonly ("yes" | "no")[] = child.holidayWishCheck
    ? ["yes", "no"]
    : [child.holidayWish === "yes" ? "no" : "yes"];

  return (
    <InfoSection title="Ferienwunsch">
      <div className="space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <StatusBadge tone={view.tone} label={view.label} icon={view.icon} />
          <span className="text-sm text-gray-600">
            {holidayWishSourceText(child)}
          </span>
        </div>
        {child.holidayWish === "no" ? (
          <p className="text-sm text-gray-600">
            Auf der Warteliste wartet das Kind hinter Kindern mit Ferienwunsch.
            Einen Platz, den es schon hat, behält es.
          </p>
        ) : null}
        <FormErrorAlert message={errors.error} />
        {cancelled ? null : (
          <div className="flex flex-wrap gap-2">
            {choices.map((wish) => (
              <Button
                key={wish}
                type="button"
                variant="outline"
                size="compact"
                isLoading={saving === wish}
                disabled={saving !== null}
                aria-label={`${name}: ${wish === "yes" ? "Mit Ferienwunsch" : "Ohne Ferienwunsch"}`}
                onClick={() => void decide(wish)}
              >
                {child.holidayWishCheck
                  ? wish === "yes"
                    ? "Mit Ferienwunsch"
                    : "Ohne Ferienwunsch"
                  : wish === "yes"
                    ? "Auf „Mit Ferienwunsch“ ändern"
                    : "Auf „Ohne Ferienwunsch“ ändern"}
              </Button>
            ))}
          </div>
        )}
      </div>
    </InfoSection>
  );
}
