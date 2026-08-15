"use client";

import { useTranslations } from "next-intl";
import { Avatar } from "~/components/ui/avatar";
import { Button, ButtonLink } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { MotoConceptKey } from "~/lib/moto-concepts";
import type { ChildFeatures, ChildToday } from "~/lib/parent-api";
import { parentPath } from "~/lib/parent-url";

/**
 * Die Tageskarte eines Kindes. Das Herzstueck der Eltern-App: auf der
 * Startseite und im Kinderbereich dieselbe Karte.
 *
 * Zweistufig nach Abschnitt 6 der Spezifikation. Ebene 1 beantwortet in einer
 * Sekunde "Ist mein Kind in der OGS?" und kommt AUSSCHLIESSLICH aus `at_ogs`.
 * Ebene 2 erklaert den Zeitpunkt und kommt aus `state`. Ist `at_ogs` null,
 * entfaellt Ebene 1 ersatzlos: eine Ja/Nein-Aussage zu treffen, die wir nicht
 * belegen koennen, waere schlimmer als zu schweigen.
 *
 * Gestaltung: die Flaeche bleibt weiss und ohne farbige Kante. Farbe traegt
 * allein das Duotone-Symbol des Zustands, und sie traegt nie allein, weil
 * jeder Zustand ein eigenes Symbol und einen eigenen Satz hat.
 */

export interface ChildDayCardChild {
  readonly studentId: string;
  readonly firstName: string;
  readonly lastName: string;
  readonly schoolClass?: string;
}

/**
 * Das Konzept je Zustand. Glyph und Ton kommen aus `MOTO_CONCEPTS`, damit
 * dieselbe Aussage in Eltern- und Personal-App dasselbe Bild ergibt.
 *
 * "Heute keine Betreuung" laeuft auf `closingDays` statt auf `calendar`:
 * `calendar` traegt denselben Indigo-Ton wie `careTimes` (erwartet / noch
 * nicht da), und beide Zustaende stehen auf der Startseite nebeneinander,
 * sobald ein Elternteil mehrere Kinder hat.
 */
const STATE_CONCEPT: Record<ChildToday["state"], MotoConceptKey> = {
  present: "present",
  left: "home",
  expected: "careTimes",
  not_arrived: "careTimes",
  absent: "notArrival",
  no_care: "closingDays",
  unknown: "unknown",
};

type TodayTranslator = ReturnType<typeof useTranslations<"parentToday">>;

/**
 * Ebene 2 im Klartext. Fehlt die Uhrzeit, die der Zustand erwartet, sagen wir
 * den Zustand ohne Zeit statt "Seit undefined Uhr da".
 */
function explainState(today: ChildToday, t: TodayTranslator): string {
  switch (today.state) {
    case "present":
      return today.since
        ? t("state.present", { time: today.since })
        : t("state.presentNoTime");
    case "left":
      return today.until
        ? t("state.left", { time: today.until })
        : t("state.leftNoTime");
    case "expected":
      return today.expected_from
        ? t("state.expected", { time: today.expected_from })
        : t("state.expectedNoTime");
    case "not_arrived":
      return today.expected_from
        ? t("state.notArrived", { time: today.expected_from })
        : t("state.notArrivedNoTime");
    case "absent":
      return t("state.absent");
    case "no_care":
      return t("state.noCare");
    default:
      return t("state.unknown");
  }
}

export function ChildDayCard({
  child,
  today,
  features,
  onSick,
  onPickup,
  hideIdentity = false,
}: Readonly<{
  child: ChildDayCardChild;
  today: ChildToday;
  /** Undefiniert, solange die Funktionen der Schule noch laden. */
  features?: ChildFeatures;
  /** Gesetzt, wenn die Seite den Krank-Dialog selbst fuehrt. */
  onSick?: () => void;
  /** Gesetzt, wenn die Seite den Abhol-Dialog selbst fuehrt. */
  onPickup?: () => void;
  /**
   * Ohne Name und Initialen. Im Kinderbereich steht der Name schon als
   * Seitentitel darueber; ihn direkt darunter zu wiederholen ist Fuellstoff.
   */
  hideIdentity?: boolean;
}>) {
  const t = useTranslations("parentToday");
  const concept = STATE_CONCEPT[today.state] ?? "unknown";
  const fullName = `${child.firstName} ${child.lastName}`;
  const level1 =
    today.at_ogs === null ? null : today.at_ogs ? t("atOgs") : t("notAtOgs");
  const level2 = explainState(today, t);

  return (
    /* @container: die Aktionsreihe richtet sich nach der Breite DIESER Karte,
       nicht nach der des Fensters. Die Karte steht mal allein in voller Breite,
       mal zu zweit nebeneinander, mal in einer schmalen Spalte; ein
       Breakpoint auf das Fenster hat die Beschriftungen deshalb je nach Fall
       umbrechen lassen, obwohl auf dem Schirm Platz war. */
    <article className="@container relative overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
      <div className="space-y-4 p-4 sm:p-5">
        {!hideIdentity && (
          <div className="flex min-w-0 items-center gap-3">
            <Avatar
              name={fullName}
              shape="rounded"
              decorative
              className="size-11 text-[15px]"
            />
            <div className="min-w-0">
              <p className="truncate text-[19px] leading-tight font-bold text-gray-900">
                {fullName}
              </p>
              {/* Der Wert kommt so, wie die Schule ihre Gruppe benennt, meist
                bereits als "Klasse 1b". Ein zusaetzliches "Klasse" davor
                ergaebe "Klasse Klasse 1b". Also unveraendert ausgeben. */}
              {child.schoolClass && (
                <p className="truncate text-[15px] text-gray-600">
                  {child.schoolClass}
                </p>
              )}
            </div>
          </div>
        )}

        <div className="flex items-center gap-3.5">
          <span
            data-testid="child-day-state-icon"
            className="flex size-14 shrink-0 items-center justify-center rounded-2xl bg-gray-100"
          >
            <MotoConceptIcon concept={concept} size={32} />
          </span>
          <div className="min-w-0">
            {level1 ? (
              <>
                {/* Der Statuswert ist die Aussage der Seite und deshalb mehr
                    als eine Stufe groesser als alles darunter. */}
                <p className="text-[32px] leading-none font-extrabold tracking-tight text-gray-900">
                  {level1}
                </p>
                <p className="mt-1.5 text-[15px] text-gray-500">{level2}</p>
              </>
            ) : (
              /* Ohne Ebene 1 traegt die Erklaerung die Aussage allein und
                 bekommt deshalb das Gewicht einer Ueberschrift. */
              <p className="text-[22px] leading-snug font-bold text-gray-900">
                {level2}
              </p>
            )}
          </div>
        </div>

        <ChildDayActions
          studentId={child.studentId}
          features={features}
          onSick={onSick}
          onPickup={onPickup}
        />
      </div>
    </article>
  );
}

/**
 * Die drei Alltagsaktionen. Sie erscheinen nur, wenn die Schule sie erlaubt,
 * sonst wuerde das Backend sie mit 403 abweisen. Untereinander in voller
 * Breite, und erst nebeneinander, wenn die Karte breit genug ist, dass keine
 * Beschriftung umbricht (Containerabfrage, nicht Fensterbreite).
 */
function ChildDayActions({
  studentId,
  features,
  onSick,
  onPickup,
}: Readonly<{
  studentId: string;
  features?: ChildFeatures;
  onSick?: () => void;
  onPickup?: () => void;
}>) {
  const t = useTranslations("parentToday");
  if (!features) return null;

  const anyAction =
    features.sick_note_enabled ||
    features.pickup_change_enabled ||
    features.notes_enabled;
  if (!anyAction) return null;

  return (
    <div className="grid grid-cols-1 gap-2 @md:grid-cols-3">
      {features.sick_note_enabled &&
        (onSick ? (
          <Button
            type="button"
            variant="outline"
            size="touch"
            className="w-full"
            onClick={onSick}
          >
            <MotoConceptIcon concept="sick" size={20} className="mr-2" />
            {t("actions.sick")}
          </Button>
        ) : (
          <ButtonLink
            href={parentPath(`/parents/children/${studentId}?action=sick`)}
            variant="outline"
            size="touch"
            className="w-full"
          >
            <MotoConceptIcon concept="sick" size={20} className="mr-2" />
            {t("actions.sick")}
          </ButtonLink>
        ))}

      {features.pickup_change_enabled &&
        (onPickup ? (
          <Button
            type="button"
            variant="outline"
            size="touch"
            className="w-full"
            onClick={onPickup}
          >
            <MotoConceptIcon concept="pickup" size={20} className="mr-2" />
            {t("actions.pickup")}
          </Button>
        ) : (
          <ButtonLink
            href={parentPath(`/parents/children/${studentId}?action=pickup`)}
            variant="outline"
            size="touch"
            className="w-full"
          >
            <MotoConceptIcon concept="pickup" size={20} className="mr-2" />
            {t("actions.pickup")}
          </ButtonLink>
        ))}

      {features.notes_enabled && (
        <ButtonLink
          href={parentPath(`/parents/messages/${studentId}`)}
          variant="outline"
          size="touch"
          className="w-full"
        >
          <MotoConceptIcon
            concept="parentConversations"
            size={20}
            className="mr-2"
          />
          {t("actions.message")}
        </ButtonLink>
      )}
    </div>
  );
}
