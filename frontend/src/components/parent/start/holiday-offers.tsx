"use client";

import { useMemo } from "react";
import { useLocale } from "next-intl";
import { ArrowRight } from "lucide-react";
import { ButtonLink } from "~/components/ui/button";
import { DataField } from "~/components/ui/data-field";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useCareTermTranslations } from "~/lib/care-facility-term";
import { holidayWeeksConcept } from "~/lib/concepts/holidayWeeks";
import { formatDate, parseISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { PARENT_RESOURCE_OPTIONS } from "~/lib/hooks/use-parent-resources";
import { listHolidayOffers, type HolidayOffer } from "~/lib/parent-api";
import { holidayRegistrationPath } from "~/lib/parent-holiday-registration";
import { useSWRAuth } from "~/lib/swr";
import { parentKeys } from "~/lib/swr/parent-keys";

// Dasselbe Polster wie die Begrüßung und „Zu erledigen“ darüber und darunter,
// damit die Kanten der Startseite in einer Flucht stehen.
const CARD_PADDING = "compact:p-5 max-sm:p-5";

/** Ab so vielen Tagen vor Fristende nennt die Karte die verbleibenden Tage. */
const DEADLINE_NOTICE_DAYS = 7;

/**
 * Ferienbetreuung auf der Startseite (#6671): die veröffentlichten
 * Ferienzeiträume der Träger der eigenen Kinder. Vor dem Anmeldestart nennt
 * die Karte den Starttag, in der Anmeldefrist führt der Knopf zum Formular,
 * danach verschwindet sie. Ohne passenden Zeitraum rendert sie nichts.
 */
export function HolidayOffers() {
  const today = useBerlinToday();
  // Bewusst still bei Lade- und Fehlerzustand: die Karte ist ein Hinweis.
  // Ohne sie bleibt die Startseite vollständig, und der Link aus dem
  // Elternbrief führt weiter zum Formular.
  const { data } = useSWRAuth(
    parentKeys.holidayOffers(),
    listHolidayOffers,
    PARENT_RESOURCE_OPTIONS,
  );
  // Der Server filtert nach dem Berliner Tag; eine über Mitternacht offene
  // Seite lässt einen abgelaufenen Zeitraum selbst weg.
  const offers = useMemo(
    () => (data ?? []).filter((offer) => offer.registration_closes_on >= today),
    [data, today],
  );

  if (offers.length === 0) return null;
  return offers.length === 1 ? (
    <SingleHolidayOffer offer={offers[0]!} today={today} />
  ) : (
    <HolidayOfferList offers={offers} today={today} />
  );
}

/** Der Regelfall: ein Zeitraum, die Karte trägt seinen Namen. */
function SingleHolidayOffer({
  offer,
  today,
}: Readonly<{ offer: HolidayOffer; today: string }>) {
  const t = useCareTermTranslations("parentStart");
  const open = isOpen(offer, today);
  return (
    <SectionCard
      testId="parent-holiday-offer"
      concept={holidayWeeksConcept}
      kicker={t("holiday.listTitle")}
      title={offer.name}
      description={t("holiday.offeredByTraeger", {
        traeger: offer.traeger_name,
      })}
      titleBadge={<HolidayOfferBadge offer={offer} today={today} />}
      className={CARD_PADDING}
    >
      {/* Der Knopf steht nach den Angaben: erst lesen, worum es geht, dann
          anmelden. Auf dem Telefon füllt er die Breite unter den Angaben. */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between sm:gap-6">
        <HolidayOfferFacts offer={offer} today={today} />
        {open ? <RegisterLink offer={offer} /> : null}
      </div>
    </SectionCard>
  );
}

/**
 * Mehrere Zeiträume: eine Karte, je Zeitraum eine dichte Zeile mit eigenem
 * Knopf. Was für alle Zeilen gilt, steht einmal im Kopf.
 */
function HolidayOfferList({
  offers,
  today,
}: Readonly<{ offers: readonly HolidayOffer[]; today: string }>) {
  const t = useCareTermTranslations("parentStart");
  const traeger = shared(offers.map((offer) => offer.traeger_name));
  const children = shared(offers.map(childIds));
  const names = useChildNames(offers[0]!);
  return (
    <SectionCard
      testId="parent-holiday-offer"
      concept={holidayWeeksConcept}
      title={t("holiday.listTitle")}
      className={CARD_PADDING}
      description={
        [
          children && names ? t("holiday.forChildren", { names }) : null,
          traeger ? t("holiday.offeredByTraeger", { traeger }) : null,
        ]
          .filter(Boolean)
          .join(" · ") || undefined
      }
    >
      <ResponsiveActionList>
        {offers.map((offer) => (
          <li
            key={offer.token}
            className="py-3.5 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center"
          >
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                <h3 className="text-[15px] font-semibold text-gray-900">
                  {offer.name}
                </h3>
                <HolidayOfferBadge offer={offer} today={today} />
              </div>
              <HolidayOfferLine
                offer={offer}
                today={today}
                showChildren={!children}
                showTraeger={!traeger}
              />
            </div>
            {isOpen(offer, today) ? (
              <div className="max-sm:mt-3">
                <RegisterLink offer={offer} />
              </div>
            ) : null}
          </li>
        ))}
      </ResponsiveActionList>
    </SectionCard>
  );
}

/** Der gemeinsame Wert aller Einträge, sonst `null`. */
function shared(values: readonly string[]): string | null {
  const [first] = values;
  return first !== undefined && values.every((value) => value === first)
    ? first
    : null;
}

function childIds(offer: HolidayOffer): string {
  return offer.children.map((child) => child.student_id).join(",");
}

function useChildNames(offer: HolidayOffer): string {
  const locale = useLocale();
  return useMemo(
    () =>
      new Intl.ListFormat(locale, { type: "conjunction" }).format(
        offer.children.map((child) => child.first_name),
      ),
    [locale, offer.children],
  );
}

function isOpen(offer: HolidayOffer, today: string): boolean {
  return offer.registration_opens_on <= today;
}

function HolidayOfferBadge({
  offer,
  today,
}: Readonly<{ offer: HolidayOffer; today: string }>) {
  const t = useCareTermTranslations("parentStart");
  const locale = useLocale();
  return isOpen(offer, today) ? (
    <StatusBadge tone="green" label={t("holiday.badgeOpen")} />
  ) : (
    <StatusBadge
      tone="blue"
      label={t("holiday.badgeUpcoming", {
        date: formatDate(offer.registration_opens_on, false, locale),
      })}
    />
  );
}

function RegisterLink({ offer }: Readonly<{ offer: HolidayOffer }>) {
  const t = useCareTermTranslations("parentStart");
  return (
    <ButtonLink
      href={holidayRegistrationPath(offer.token)}
      variant="primary"
      size="md"
      className="shrink-0 gap-2 max-sm:w-full"
      aria-label={t("holiday.registerLabel", { name: offer.name })}
    >
      {t("holiday.register")}
      <ArrowRight className="size-4" aria-hidden="true" />
    </ButtonLink>
  );
}

function HolidayOfferFacts({
  offer,
  today,
}: Readonly<{ offer: HolidayOffer; today: string }>) {
  const t = useCareTermTranslations("parentStart");
  const locale = useLocale();
  const date = (iso: string) => formatDate(iso, false, locale);
  const children = useChildNames(offer);
  const daysLeft = daysBetween(today, offer.registration_closes_on);
  const open = isOpen(offer, today);
  return (
    <dl className="grid min-w-0 flex-1 grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-3">
      {offer.first_day && offer.last_day ? (
        <DataField label={t("holiday.care")}>
          {t("holiday.range", {
            from: date(offer.first_day),
            to: date(offer.last_day),
          })}
        </DataField>
      ) : null}
      <DataField label={t("holiday.registration")}>
        {open
          ? t("holiday.until", { date: date(offer.registration_closes_on) })
          : t("holiday.range", {
              from: date(offer.registration_opens_on),
              to: date(offer.registration_closes_on),
            })}
        {open && daysLeft <= DEADLINE_NOTICE_DAYS ? (
          <span className="text-moto-orange-strong block">
            <DeadlineNotice daysLeft={daysLeft} />
          </span>
        ) : null}
      </DataField>
      {children ? (
        <DataField label={t("holiday.for")}>{children}</DataField>
      ) : null}
    </dl>
  );
}

function DeadlineNotice({ daysLeft }: Readonly<{ daysLeft: number }>) {
  const t = useCareTermTranslations("parentStart");
  return daysLeft === 0
    ? t("holiday.lastDay")
    : t("holiday.daysLeft", { days: daysLeft });
}

/** Die Angaben eines Zeitraums als eine Zeile, für die Liste. */
function HolidayOfferLine({
  offer,
  today,
  showChildren,
  showTraeger,
}: Readonly<{
  offer: HolidayOffer;
  today: string;
  showChildren: boolean;
  showTraeger: boolean;
}>) {
  const t = useCareTermTranslations("parentStart");
  const locale = useLocale();
  const date = (iso: string) => formatDate(iso, false, locale);
  const children = useChildNames(offer);
  const daysLeft = daysBetween(today, offer.registration_closes_on);
  const open = isOpen(offer, today);
  return (
    <p className="mt-1 flex flex-wrap gap-x-4 gap-y-0.5 text-sm leading-6 text-gray-600">
      {offer.first_day && offer.last_day ? (
        <span>
          {t("holiday.careLine", {
            from: date(offer.first_day),
            to: date(offer.last_day),
          })}
        </span>
      ) : null}
      <span>
        {open
          ? t("holiday.registrationLine", {
              date: date(offer.registration_closes_on),
            })
          : t("holiday.registrationRangeLine", {
              from: date(offer.registration_opens_on),
              to: date(offer.registration_closes_on),
            })}
        {open && daysLeft <= DEADLINE_NOTICE_DAYS ? (
          <span className="text-moto-orange-strong font-medium">
            {" "}
            (<DeadlineNotice daysLeft={daysLeft} />)
          </span>
        ) : null}
      </span>
      {showChildren && children ? (
        <span>{t("holiday.forChildren", { names: children })}</span>
      ) : null}
      {showTraeger ? (
        <span>
          {t("holiday.offeredByTraeger", { traeger: offer.traeger_name })}
        </span>
      ) : null}
    </p>
  );
}

/** Ganze Kalendertage von `from` bis `to`, beide als "YYYY-MM-DD". */
function daysBetween(from: string, to: string): number {
  const start = parseISODate(from);
  const end = parseISODate(to);
  return Math.round(
    (Date.UTC(end.getFullYear(), end.getMonth(), end.getDate()) -
      Date.UTC(start.getFullYear(), start.getMonth(), start.getDate())) /
      86_400_000,
  );
}
