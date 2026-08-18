"use client";

import { useMemo, type ReactNode } from "react";
import { XIcon } from "@phosphor-icons/react/ssr";
import { ChevronRight } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { Button, ButtonLink } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { Skeleton } from "~/components/ui/skeleton";
import type { ChildFeatures, ChildToday } from "~/lib/parent-api";
import type { MotoConceptKey } from "~/lib/moto-concepts";
import { parentPath } from "~/lib/parent-url";
import { formatDate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";

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
 * Gestaltung: die Flaeche bleibt weiss und ohne farbige Kante. Ein ruhiges
 * Statussymbol ergaenzt den vollstaendigen Klartext, ersetzt ihn aber nie.
 */

export interface ChildDayCardChild {
  readonly studentId: string;
  readonly firstName: string;
  readonly lastName: string;
  readonly schoolClass?: string;
}

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
  statusAside,
  actionDisplay = "detailed",
  hideIdentity = false,
  loading = false,
}: Readonly<{
  child: ChildDayCardChild;
  today: ChildToday;
  /** Undefiniert, solange die Funktionen der Schule noch laden. */
  features?: ChildFeatures;
  /** Gesetzt, wenn die Seite den Krank-Dialog selbst fuehrt. */
  onSick?: () => void;
  /** Gesetzt, wenn die Seite den Abhol-Dialog selbst fuehrt. */
  onPickup?: () => void;
  /** Zusaetzlicher Tageskontext, auf breiten Karten rechts vom Status. */
  statusAside?: ReactNode;
  /** Kompakte Aktionen fuer Uebersichten, ausfuehrliche fuer die Kinderseite. */
  actionDisplay?: "compact" | "detailed";
  /**
   * Ohne Name. Im Kinderbereich steht der Name schon als Seitentitel darueber;
   * ihn direkt darunter zu wiederholen ist Fuellstoff.
   */
  hideIdentity?: boolean;
  /** Reserviert Status und Aktionen, bis Tagesdaten und Funktionen feststehen. */
  loading?: boolean;
}>) {
  const t = useTranslations("parentToday");
  const locale = useLocale();
  const todayDateISO = useBerlinToday();
  const todayDateLabel = useMemo(
    () => formatDate(todayDateISO, true, locale),
    [locale, todayDateISO],
  );
  const fullName = `${child.firstName} ${child.lastName}`;
  const level1 =
    today.at_ogs === null ? null : today.at_ogs ? t("atOgs") : t("notAtOgs");
  const level2 = explainState(today, t);
  const stateConcept: MotoConceptKey =
    today.at_ogs === null ? "unknown" : today.at_ogs ? "present" : "home";

  return (
    /* @container: die Aktionsreihe richtet sich nach der Breite DIESER Karte,
       nicht nach der des Fensters. Die Karte steht mal allein in voller Breite,
       mal zu zweit nebeneinander, mal in einer schmalen Spalte; ein
       Breakpoint auf das Fenster hat die Beschriftungen deshalb je nach Fall
       umbrechen lassen, obwohl auf dem Schirm Platz war. */
    <article
      data-testid="child-day-card"
      className="@container relative overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm"
    >
      <div className="space-y-5 p-5 sm:p-6">
        {hideIdentity && (
          <div>
            <h2 className="text-xl leading-tight font-semibold tracking-tight text-balance text-gray-900">
              {t("sectionTitle")}
            </h2>
            <time
              dateTime={todayDateISO}
              className="mt-1 block text-sm leading-6 font-medium text-gray-600"
            >
              {todayDateLabel}
            </time>
            <p className="mt-1 max-w-2xl text-sm leading-6 text-pretty text-gray-600">
              {t("sectionDescription")}
            </p>
          </div>
        )}

        {!hideIdentity && (
          <div className="flex min-w-0 items-center justify-between gap-3">
            <div className="flex min-w-0 flex-1 items-center gap-3">
              <span
                data-testid="child-card-profile-icon"
                className="grid size-11 shrink-0 place-items-center rounded-xl bg-gray-50"
              >
                <MotoConceptIcon
                  concept="children"
                  size={28}
                  aria-hidden="true"
                />
              </span>
              <div className="min-w-0">
                <h2 className="text-lg leading-snug font-semibold break-words text-gray-900">
                  {fullName}
                </h2>
                {/* Der Wert kommt so, wie die Schule ihre Gruppe benennt, meist
                    bereits als "Klasse 1b". Ein zusaetzliches "Klasse" davor
                    ergaebe "Klasse Klasse 1b". Also unveraendert ausgeben. */}
                {child.schoolClass && (
                  <p className="truncate text-sm text-gray-600">
                    {child.schoolClass}
                  </p>
                )}
              </div>
            </div>
            <ButtonLink
              href={parentPath(`/parents/children/${child.studentId}`)}
              variant="ghost"
              size="md"
              className="min-h-11 shrink-0 px-3 text-sm font-medium hover:bg-gray-100"
            >
              {t("actions.profile")}
              <ChevronRight
                className="ml-1 size-4 shrink-0"
                aria-hidden="true"
              />
            </ButtonLink>
          </div>
        )}

        <div
          className={`grid min-w-0 gap-4 rounded-xl bg-gray-50 p-4 ${statusAside ? "@2xl:grid-cols-[minmax(0,1fr)_minmax(14rem,0.65fr)]" : ""}`}
        >
          {loading ? (
            <div className="flex min-h-20 items-start gap-3" aria-hidden="true">
              <Skeleton className="size-8 shrink-0 rounded-full" />
              <div className="min-w-0 flex-1 space-y-2">
                <Skeleton className="h-3 w-20" />
                <Skeleton className="h-7 w-36 max-w-full" />
                <Skeleton className="h-4 w-48 max-w-full" />
              </div>
            </div>
          ) : (
            <div className="flex min-w-0 items-start gap-3">
              <span
                className="mt-0.5 shrink-0"
                data-testid="child-day-state-icon"
                aria-hidden="true"
              >
                <MotoConceptIcon concept={stateConcept} size={30} />
              </span>
              <div className="min-w-0">
                <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                  {hideIdentity ? t("statusLabel") : t("today")}
                </p>
                {!hideIdentity && (
                  <time
                    dateTime={todayDateISO}
                    className="mt-0.5 block text-xs leading-5 font-medium text-gray-500"
                  >
                    {todayDateLabel}
                  </time>
                )}
                {level1 ? (
                  <>
                    <p className="mt-1 text-2xl leading-tight font-semibold tracking-tight text-gray-900">
                      {level1}
                    </p>
                    <p className="mt-1 text-sm leading-6 text-gray-600">
                      {level2}
                    </p>
                  </>
                ) : (
                  <p className="mt-1 text-base leading-6 font-medium text-gray-900">
                    {level2}
                  </p>
                )}
              </div>
            </div>
          )}
          {statusAside &&
            (loading ? (
              <div
                className="border-t border-gray-200 pt-4 @2xl:border-t-0 @2xl:border-l @2xl:pt-0 @2xl:pl-4"
                aria-hidden="true"
              >
                <Skeleton className="h-3 w-24" />
                <Skeleton className="mt-2 h-5 w-36" />
              </div>
            ) : (
              <div className="border-t border-gray-200 pt-4 @2xl:border-t-0 @2xl:border-l @2xl:pt-0 @2xl:pl-4">
                {statusAside}
              </div>
            ))}
        </div>

        {loading ? (
          <div className="grid gap-2 @md:grid-cols-2" aria-hidden="true">
            <Skeleton className="h-11 w-full rounded-lg" />
            <Skeleton className="h-11 w-full rounded-lg" />
          </div>
        ) : (
          <ChildDayActions
            studentId={child.studentId}
            firstName={child.firstName}
            features={features}
            onSick={onSick}
            onPickup={onPickup}
            display={actionDisplay}
          />
        )}
      </div>
    </article>
  );
}

/**
 * Die drei Alltagsaktionen. Sie erscheinen nur, wenn die Schule sie erlaubt,
 * sonst wuerde das Backend sie mit 403 abweisen. Untereinander in voller
 * Breite. Drei Aktionen wechseln bei mittlerer Kartenbreite in ein 2+1-Raster
 * und erst bei wirklich breiten Karten in eine Zeile. So entscheidet die
 * Kartenbreite, nicht das Fenster, wann die Beschriftungen genug Platz haben.
 */
function ChildDayActions({
  studentId,
  firstName,
  features,
  onSick,
  onPickup,
  display,
}: Readonly<{
  studentId: string;
  firstName: string;
  features?: ChildFeatures;
  onSick?: () => void;
  onPickup?: () => void;
  display: "compact" | "detailed";
}>) {
  const t = useTranslations("parentToday");
  if (!features) return null;

  const anyAction =
    features.sick_note_enabled ||
    features.pickup_change_enabled ||
    features.notes_enabled;
  if (!anyAction) return null;

  const actionCount = [
    features.sick_note_enabled,
    features.pickup_change_enabled,
    features.notes_enabled,
  ].filter(Boolean).length;
  const gridColumns =
    actionCount === 3
      ? display === "compact"
        ? "@md:grid-cols-2 @2xl:grid-cols-3"
        : "@md:grid-cols-2 @3xl:grid-cols-3"
      : actionCount === 2
        ? "@md:grid-cols-2"
        : "";
  const actionClassName =
    display === "compact"
      ? "min-w-0 w-full gap-2 px-3 py-2 text-sm font-semibold"
      : "w-full gap-3 px-4 py-2";
  const absenceActionContent = (
    <>
      <MotoDuotoneIcon icon={XIcon} tone="red" size={20} weight="bold" />
      {display === "compact" ? (
        <span className="min-w-0 text-center leading-5">
          {t("actions.sick", { name: firstName })}
        </span>
      ) : (
        <span className="min-w-0 text-left">
          <span className="block text-base leading-5 font-semibold">
            {t("actions.sick", { name: firstName })}
          </span>
          <span className="mt-0.5 block text-xs leading-4 font-normal text-gray-500">
            {t("actions.sickHint")}
          </span>
        </span>
      )}
    </>
  );
  const pickupActionContent = (
    <>
      <MotoConceptIcon concept="pickup" size={20} aria-hidden="true" />
      {display === "compact" ? (
        <span className="min-w-0 text-center leading-5">
          {t("actions.pickupCompact")}
        </span>
      ) : (
        <span className="min-w-0 text-left">
          <span className="block text-base leading-5 font-semibold">
            {t("actions.pickup", { name: firstName })}
          </span>
          <span className="mt-0.5 block text-xs leading-4 font-normal text-gray-500">
            {t("actions.pickupHint")}
          </span>
        </span>
      )}
    </>
  );
  const messageActionContent = (
    <>
      <MotoConceptIcon
        concept="parentConversations"
        size={20}
        aria-hidden="true"
      />
      {display === "compact" ? (
        <span className="min-w-0 text-center leading-5">
          {t("actions.messageCompact")}
        </span>
      ) : (
        <span className="min-w-0 text-left">
          <span className="block text-base leading-5 font-semibold">
            {t("actions.message", { name: firstName })}
          </span>
          <span className="mt-0.5 block text-xs leading-4 font-normal text-gray-500">
            {t("actions.messageHint", { name: firstName })}
          </span>
        </span>
      )}
    </>
  );

  return (
    <div
      className={`grid grid-cols-1 gap-2 border-t border-gray-100 pt-4 ${gridColumns}`}
    >
      {features.sick_note_enabled &&
        (onSick ? (
          <Button
            type="button"
            variant="surface"
            size="touch"
            className={actionClassName}
            onClick={onSick}
          >
            {absenceActionContent}
          </Button>
        ) : (
          <ButtonLink
            href={parentPath(`/parents/children/${studentId}?action=sick`)}
            variant="surface"
            size="touch"
            className={actionClassName}
          >
            {absenceActionContent}
          </ButtonLink>
        ))}

      {features.pickup_change_enabled &&
        (onPickup ? (
          <Button
            type="button"
            variant="surface"
            size="touch"
            className={actionClassName}
            onClick={onPickup}
          >
            {pickupActionContent}
          </Button>
        ) : (
          <ButtonLink
            href={parentPath(`/parents/children/${studentId}?action=pickup`)}
            variant="surface"
            size="touch"
            className={actionClassName}
          >
            {pickupActionContent}
          </ButtonLink>
        ))}

      {features.notes_enabled && (
        <ButtonLink
          href={parentPath(`/parents/messages/${studentId}`)}
          variant="surface"
          size="touch"
          className={`${actionClassName} ${
            actionCount !== 3
              ? ""
              : display === "compact"
                ? "@md:col-span-2 @2xl:col-span-1"
                : "@md:col-span-2 @3xl:col-span-1"
          }`}
        >
          {messageActionContent}
        </ButtonLink>
      )}
    </div>
  );
}
