"use client";

/**
 * Ferienbetreuung in den Planungsrastern: Tage, an denen die Schule eine
 * Ferienwoche für ihren Träger ausrichtet (ADR 0116, 0120).
 *
 * Gegenstück zur Schließtag-Markierung (`closing-day-marker.tsx`), aber mit
 * eigener Sprache, weil sie das Gegenteil sagt: an einem Schließtag entfällt
 * die Betreuung, hier findet sie statt. Deshalb Symbol und Farbe der Seite
 * „Ferienwochen“ (Sonne über dem Horizont, Orange als StatusBadge-Ton ohne
 * Warnbedeutung) statt des grauen Schließtag-Kalenders.
 *
 * Wer die Seite „Ferienwochen“ öffnen darf, bekommt die Markierung als Link
 * dorthin; alle anderen sehen dieselbe Markierung als reine Anzeige.
 */

import { ActionHint } from "~/components/ui/action-hint";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import NavigationLink from "~/components/ui/navigation-link";
import { StatusBadge } from "~/components/ui/status-badge";
import { holidayWeeksConcept } from "~/lib/concepts/holidayWeeks";
import {
  holidayCareChildren,
  holidayCareLabel,
  holidayCareSentence,
  holidayCareWindow,
  type HolidayCareDay,
} from "~/lib/holiday-care";
import { MOTO_COLOR_PALETTE } from "~/lib/kit-palette";

interface HolidayCareChipProps {
  readonly day: HolidayCareDay;
  /** Ziel „Ferienwochen“; ohne Ziel ist der Chip eine reine Anzeige. */
  readonly href?: string;
  /** Sichtbarer Kurztext statt der vollen Beschriftung, z. B. die Anzahl. */
  readonly text?: string;
  /** Betreuungszeit sichtbar zeigen; aus, wo das Raster sie selbst zeigt. */
  readonly withWindow?: boolean;
  /**
   * Zweizeilig für Spalten mit Platz nach unten: der Chip sagt
   * „Ferienbetreuung“, darunter stehen Kinder und Zeit als ruhige Zeile.
   */
  readonly stacked?: boolean;
  readonly className?: string;
}

/**
 * Kennzeichnung eines Tages mit Ferienbetreuung. Der ganze Satz steht im
 * Tooltip und für Screenreader; `text` ersetzt den sichtbaren Teil in
 * schmalen Spalten. `compactOnMobile` zeigt unter `sm` nur das Symbol: in
 * den schmalen Tagesspalten am Telefon bliebe vom Text nur ein Bruchstück.
 */
export function HolidayCareChip({
  compactOnMobile = false,
  ...props
}: HolidayCareChipProps & { readonly compactOnMobile?: boolean }) {
  if (!compactOnMobile) return <HolidayCareChipBody {...props} />;
  return (
    <>
      <span className="hidden min-w-0 sm:contents">
        <HolidayCareChipBody {...props} />
      </span>
      <span className="contents sm:hidden">
        <HolidayCareChipBody
          day={props.day}
          href={props.href}
          text=""
          className="mx-auto flex w-fit"
        />
      </span>
    </>
  );
}

function HolidayCareChipBody({
  day,
  href,
  text,
  withWindow = true,
  stacked = false,
  className,
}: HolidayCareChipProps) {
  const sentence = holidayCareSentence(day);
  const ownsText = href !== undefined || text !== undefined || stacked;
  let label = text ?? holidayCareLabel(day, { withWindow });
  if (stacked) label = "Ferienbetreuung";
  const pill = (
    <StatusBadge
      ariaHidden={ownsText}
      className={href === undefined && !stacked ? className : "max-w-full"}
      label={label}
      title={href === undefined ? sentence : undefined}
      tone="orange"
      size="xs"
      icon={<MotoConceptIcon concept={holidayWeeksConcept} size={14} />}
      truncate
    />
  );
  const detail = [holidayCareChildren(day.children), holidayCareWindow(day)]
    .filter(Boolean)
    .join(" · ");
  const badge = stacked ? (
    <span
      className={`flex min-w-0 flex-col items-start gap-0.5 ${href === undefined ? (className ?? "") : ""}`}
      title={href === undefined ? sentence : undefined}
    >
      {pill}
      <span
        aria-hidden
        className="max-w-full truncate pl-1 text-xs leading-4 font-medium tabular-nums"
        style={{ color: MOTO_COLOR_PALETTE.orange.strong }}
      >
        {detail}
      </span>
    </span>
  ) : (
    pill
  );
  if (href !== undefined) {
    return (
      <ActionHint text={`${sentence} Öffnet die Ferienwochen.`}>
        <NavigationLink
          href={href}
          className={`inline-flex max-w-full min-w-0 rounded-full transition-[filter] hover:brightness-95 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:ring-offset-1 focus-visible:outline-none ${className ?? ""}`}
        >
          {badge}
          <span className="sr-only">{holidayCareLabel(day)}</span>
        </NavigationLink>
      </ActionHint>
    );
  }
  return (
    <>
      {badge}
      {ownsText ? <span className="sr-only">{sentence}</span> : null}
    </>
  );
}

/**
 * Die Betreuungszeit als ruhiges Band in einer Tagesspalte des Zeitrasters:
 * Termine liegen darüber, Klicks gehen hindurch. Das Band zeigt nur, wann
 * Ferienkinder da sind; Zahl und Ziel trägt der Chip im Tageskopf.
 */
export function HolidayCareWindowBand({
  top,
  height,
  label,
  mobileLabel,
}: {
  readonly top: number;
  readonly height: number;
  /** Sichtbare Zeit am oberen Rand, z. B. „08:00–16:00“. */
  readonly label: string;
  /**
   * Beschriftung unter `sm`, wo der Tageskopf nur das Symbol zeigt, z. B.
   * „Ferienbetreuung · 3 Kinder · 08:00–16:00“.
   */
  readonly mobileLabel?: string;
}) {
  return (
    <div
      aria-hidden
      data-holiday-care-band=""
      className="pointer-events-none absolute inset-x-0"
      style={{
        top: `${top}px`,
        height: `${height}px`,
        backgroundColor: `${MOTO_COLOR_PALETTE.orange.soft}80`,
      }}
    >
      {/* Ein Zeitbalken mit runden Enden, nicht bündig zur Spaltenkante:
      bündig läse er sich als Rahmen der Spalte. */}
      <span
        className="absolute inset-y-1 right-1 w-[3px] rounded-full"
        style={{ backgroundColor: `${MOTO_COLOR_PALETTE.orange.base}8C` }}
      />
      <span
        className="absolute top-0.5 right-2.5 text-xs leading-4 font-medium tabular-nums"
        style={{ color: MOTO_COLOR_PALETTE.orange.strong }}
      >
        {mobileLabel === undefined ? (
          label
        ) : (
          <>
            <span className="sm:hidden">{mobileLabel}</span>
            <span className="hidden sm:inline">{label}</span>
          </>
        )}
      </span>
    </div>
  );
}
