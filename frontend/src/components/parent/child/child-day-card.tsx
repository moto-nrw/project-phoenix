"use client";

import { useTranslations } from "next-intl";
import { Avatar } from "~/components/ui/avatar";
import { Button, ButtonLink } from "~/components/ui/button";
import {
  CalendarCheck,
  CalendarX,
  ChatCircleText,
  CheckCircle,
  Clock,
  FirstAid,
  House,
  Prohibit,
  Question,
  type Icon,
  type IconWeight,
} from "~/components/parent/shell/parent-icons";
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
 * Gestaltung: die Flaeche bleibt weiss. Farbe erscheint nur an der linken
 * Kante und im Icon-Feld, und sie traegt nie allein, weil jeder Zustand ein
 * eigenes Icon und einen eigenen Satz hat.
 */

export interface ChildDayCardChild {
  readonly studentId: string;
  readonly firstName: string;
  readonly lastName: string;
  readonly schoolClass?: string;
}

/** Kante, Icon-Feld und Icon je Zustand. Eine Farbe steht fuer einen Zustand. */
interface StateLook {
  readonly edge: string;
  readonly field: string;
  readonly icon: Icon;
  readonly weight: IconWeight;
}

const NEUTRAL: StateLook = {
  edge: "bg-gray-300",
  field: "bg-gray-100 text-gray-600",
  icon: Question,
  weight: "regular",
};

const STATE_LOOK: Record<ChildToday["state"], StateLook> = {
  present: {
    edge: "bg-moto-green",
    field: "bg-moto-green-soft text-moto-green-strong",
    icon: CheckCircle,
    weight: "fill",
  },
  left: {
    edge: "bg-gray-300",
    field: "bg-gray-100 text-gray-600",
    icon: House,
    weight: "regular",
  },
  expected: {
    edge: "bg-moto-blue",
    field: "bg-moto-blue-soft text-moto-blue-strong",
    icon: Clock,
    weight: "regular",
  },
  not_arrived: {
    edge: "bg-moto-blue",
    field: "bg-moto-blue-soft text-moto-blue-strong",
    icon: Clock,
    weight: "regular",
  },
  absent: {
    edge: "bg-parent-red",
    field: "bg-parent-red-soft text-parent-red-strong",
    icon: Prohibit,
    weight: "regular",
  },
  no_care: {
    edge: "bg-gray-300",
    field: "bg-gray-100 text-gray-600",
    icon: CalendarX,
    weight: "regular",
  },
  unknown: NEUTRAL,
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
}: Readonly<{
  child: ChildDayCardChild;
  today: ChildToday;
  /** Undefiniert, solange die Funktionen der Schule noch laden. */
  features?: ChildFeatures;
  /** Gesetzt, wenn die Seite den Krank-Dialog selbst fuehrt. */
  onSick?: () => void;
  /** Gesetzt, wenn die Seite den Abhol-Dialog selbst fuehrt. */
  onPickup?: () => void;
}>) {
  const t = useTranslations("parentToday");
  const look = STATE_LOOK[today.state] ?? NEUTRAL;
  const StateIcon = look.icon;
  const fullName = `${child.firstName} ${child.lastName}`;
  const level1 =
    today.at_ogs === null ? null : today.at_ogs ? t("atOgs") : t("notAtOgs");
  const level2 = explainState(today, t);

  return (
    <article className="relative overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
      <span
        className={`absolute inset-y-0 left-0 w-1 ${look.edge}`}
        aria-hidden="true"
      />

      <div className="space-y-4 py-4 pr-4 pl-5 sm:py-5 sm:pr-5 sm:pl-6">
        <div className="flex min-w-0 items-center gap-3">
          <Avatar
            name={fullName}
            shape="rounded"
            decorative
            className="size-11 text-[15px]"
          />
          <div className="min-w-0">
            <p className="truncate text-[20px] leading-tight font-semibold text-gray-900">
              {fullName}
            </p>
            {child.schoolClass && (
              <p className="truncate text-[15px] text-gray-600">
                {t("schoolClass", { class: child.schoolClass })}
              </p>
            )}
          </div>
        </div>

        <div className="flex items-center gap-3">
          <span
            data-testid="child-day-state-icon"
            className={`flex size-12 shrink-0 items-center justify-center rounded-xl ${look.field}`}
          >
            <StateIcon size={26} weight={look.weight} aria-hidden="true" />
          </span>
          <div className="min-w-0">
            {level1 ? (
              <>
                <p className="text-[24px] leading-tight font-extrabold text-gray-900">
                  {level1}
                </p>
                <p className="mt-0.5 text-[15px] text-gray-600">{level2}</p>
              </>
            ) : (
              /* Ohne Ebene 1 traegt die Erklaerung die Aussage allein und
                 bekommt deshalb das Gewicht einer Ueberschrift. */
              <p className="text-[17px] leading-snug font-semibold text-gray-900">
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
 * sonst wuerde das Backend sie mit 403 abweisen. Mobil untereinander in voller
 * Breite, ab sm nebeneinander gleich breit.
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
    <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
      {features.sick_note_enabled &&
        (onSick ? (
          <Button
            type="button"
            variant="outline"
            size="touch"
            className="w-full"
            onClick={onSick}
          >
            <FirstAid size={20} className="mr-2 shrink-0" aria-hidden="true" />
            {t("actions.sick")}
          </Button>
        ) : (
          <ButtonLink
            href={parentPath(`/parents/children/${studentId}?action=sick`)}
            variant="outline"
            size="touch"
            className="w-full"
          >
            <FirstAid size={20} className="mr-2 shrink-0" aria-hidden="true" />
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
            <CalendarCheck
              size={20}
              className="mr-2 shrink-0"
              aria-hidden="true"
            />
            {t("actions.pickup")}
          </Button>
        ) : (
          <ButtonLink
            href={parentPath(`/parents/children/${studentId}?action=pickup`)}
            variant="outline"
            size="touch"
            className="w-full"
          >
            <CalendarCheck
              size={20}
              className="mr-2 shrink-0"
              aria-hidden="true"
            />
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
          <ChatCircleText
            size={20}
            className="mr-2 shrink-0"
            aria-hidden="true"
          />
          {t("actions.message")}
        </ButtonLink>
      )}
    </div>
  );
}
