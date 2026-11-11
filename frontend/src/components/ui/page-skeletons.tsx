"use client";

// Shared realistic loading skeletons. Each primitive mirrors the markup of
// its real kit counterpart (DataTable, SectionCard, TenantPage,
// detail-modal-components) so the pulse state has the same silhouette as the
// loaded screen and the swap causes no layout reshuffle.
//
// Composition rule: exactly one wrapper per loading region announces itself
// (`<output aria-label=…>`); the primitives inside are plain presentational
// divs. Use `SkeletonRegion` (or a page-level `<output>`) as that wrapper.

import type React from "react";

import { ChoiceTile } from "~/components/ui/choice-tile";
import { MobilePageHelpTarget } from "~/components/ui/mobile-page-help-target";
import { Skeleton } from "~/components/ui/skeleton";

import {
  HEAD_CONTROL_HEIGHT,
  HEAD_CONTROL_HEIGHT_SELF,
  HEAD_PADDING,
  HEAD_TITLE_SIZE,
} from "./control-geometry";

// Deterministic width cycle — varied line lengths read as real text without
// Math.random() (which would break SSR hydration and test snapshots).
const LINE_WIDTHS = ["w-3/4", "w-1/2", "w-2/3", "w-5/6", "w-2/5", "w-3/5"];

function lineWidth(i: number): string {
  return LINE_WIDTHS[i % LINE_WIDTHS.length] ?? "w-2/3";
}

/**
 * Announcing wrapper: one per loading region. The visual loading state mounts
 * immediately so every skeleton follows the same timing contract and no
 * invisible placeholder phase can replace already-visible content.
 */
export function SkeletonRegion({
  label,
  className,
  children,
  testId,
}: Readonly<{
  label: string;
  className?: string;
  children: React.ReactNode;
  testId?: string;
}>) {
  return (
    <output
      aria-label={label}
      aria-live="polite"
      aria-busy="true"
      data-testid={testId}
      className={`block w-full ${className ?? ""}`}
    >
      <div>{children}</div>
      <span className="sr-only">{label}</span>
    </output>
  );
}

// Maße wie im Kopf von `TenantPage`: dieselben Ränder je Bildschirm, eine
// Titelzeile in der Höhe der Überschrift (Schriftgrad mal `leading-tight`) und
// eine 20-px-Statuszeile 4 px darunter. Das Elternportal lädt mit einem
// anderen Kopf (`ParentPageHeader`) und behält bis #6613 die Maße von vor
// #6568: 20 px Rand, ein 28-px-Titelbalken, 8 px darunter die Statuszeile.
// Der Layouttest `page-header-skeleton.spec.ts` misst beide Varianten nach.
//
// Unter `lg` setzt die Portalhülle die Seitenhilfe als Knopf in den Kopf von
// `TenantPage` (`MobilePageHelpTarget`), auf der Bedienhöhe der Kopfkarte.
// Der Ladekopf bietet ihr denselben Platz an. Sonst wächst der Kopf einer
// Seite ohne Statuszeile beim Wechsel um diesen Knopf.
const HEAD_GEOMETRY = {
  "tenant-page": {
    surface: HEAD_PADDING,
    titleLine: `h-lh ${HEAD_TITLE_SIZE.standard}`,
    titleBar: "h-[1em]",
    statusLine: "mt-1 h-5",
  },
  "parent-page": {
    surface: "p-5",
    titleLine: "",
    titleBar: "h-7",
    statusLine: "mt-2",
  },
} as const;

/**
 * Die Kopfkarte von `TenantPage` als Platzhalter: dieselbe Fläche, dieselben
 * Ränder, dieselbe Zeilenfolge (Titel, Statuszeile, optional Bedienzeile).
 * Jede Seite, die während des Ladens ihren Kopf zeigt, nimmt diese hier —
 * sonst springt der Inhalt, sobald die Daten da sind.
 */
export function TenantPageHeaderSkeleton({
  leading = false,
  stats = true,
  info = false,
  controls = false,
  back,
  matches = "tenant-page",
}: Readonly<{
  /** Platz für Avatar oder Konzept-Symbol links vom Titel. */
  leading?: boolean;
  /** Zeile für die Statuszeile; aus für Seiten ohne `stats`. */
  stats?: boolean;
  /** Platz für den Info-Knopf neben dem Titel. */
  info?: boolean;
  /**
   * Der Rückweg über dem Titel, wie `TenantPage` ihn mit `back` setzt. Er
   * steht schon während des Ladens als echter Link da, nicht als Platzhalter.
   */
  back?: React.ReactNode;
  /** Zweite Zeile für Suche, Filter oder Zeitnavigation. */
  controls?: boolean;
  /**
   * Welcher Kopf danach kommt. `parent-page` hält Rand, Titel und
   * Statuszeile des Elternportals auf den bisherigen Maßen (#6613).
   */
  matches?: keyof typeof HEAD_GEOMETRY;
}> = {}) {
  const geometry = HEAD_GEOMETRY[matches];
  // Bedienelemente mit 40 px, auf dem Telefon 44 px; `leading` hängt an der
  // Größe, die die Seite für Avatar oder Symbol wählt.
  const titleRow = (
    <div className="flex min-w-0 items-center gap-3">
      {leading && <Skeleton shape="inner" className="h-12 w-12 shrink-0" />}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1">
          <div
            className={`flex w-52 max-w-3/4 items-center ${geometry.titleLine}`}
          >
            <Skeleton shape="round" className={`w-full ${geometry.titleBar}`} />
          </div>
          {info && <Skeleton className="h-8 w-8 shrink-0" />}
        </div>
        {stats && (
          <div className={`flex items-center ${geometry.statusLine}`}>
            <Skeleton shape="round" className="h-4 w-64 max-w-full" />
          </div>
        )}
      </div>
    </div>
  );
  return (
    <header
      className={`moto-content-surface rounded-2xl border ${geometry.surface}`}
    >
      {back}
      {matches === "tenant-page" ? (
        <div className="flex items-start gap-x-4 max-sm:gap-x-2">
          <div className="min-w-0 flex-1">{titleRow}</div>
          <div className={`contents ${HEAD_CONTROL_HEIGHT}`}>
            <MobilePageHelpTarget />
          </div>
        </div>
      ) : (
        titleRow
      )}
      {controls && (
        <div className="compact:mt-3 mt-4 flex items-center gap-3 max-sm:mt-3">
          <Skeleton
            shape="control"
            className={`w-48 min-w-0 ${HEAD_CONTROL_HEIGHT_SELF}`}
          />
          <Skeleton
            shape="control"
            className={`w-32 min-w-0 ${HEAD_CONTROL_HEIGHT_SELF}`}
          />
        </div>
      )}
    </header>
  );
}

/** One card-shaped skeleton: icon tile, title, then label/value rows. */
export function CardSkeleton({ rows = 3 }: Readonly<{ rows?: number }>) {
  return (
    <div className="moto-content-surface flex h-full flex-col rounded-2xl border p-4 sm:p-6">
      <div className="mb-4 flex items-center gap-3">
        <Skeleton
          shape="inner"
          className="h-9 w-9 flex-shrink-0 sm:h-10 sm:w-10"
        />
        <Skeleton className="h-5 w-36" />
      </div>
      <div className="flex flex-1 flex-col space-y-3">
        {Array.from({ length: rows }, (_, i) => (
          <div key={i} className="space-y-1">
            <Skeleton className="h-3 w-20" />
            <Skeleton className={`h-4 ${lineWidth(i)}`} />
          </div>
        ))}
      </div>
    </div>
  );
}

/** Responsive grid of card-shaped skeletons (same breakpoints as the list pages). */
export function CardGridSkeleton({
  cards = 6,
  rowsPerCard = 3,
  className = "grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-3",
}: Readonly<{ cards?: number; rowsPerCard?: number; className?: string }>) {
  return (
    <div className={className}>
      {Array.from({ length: cards }, (_, i) => (
        <CardSkeleton key={i} rows={rowsPerCard} />
      ))}
    </div>
  );
}

/**
 * Mirrors InfoSection + DataGrid from detail-modal-components: section
 * heading, then a two-column grid of label/value field pairs.
 */
export function DetailSectionSkeleton({
  fields = 4,
}: Readonly<{ fields?: number }>) {
  return (
    <div className="moto-content-surface rounded-2xl border p-4 sm:p-6">
      <Skeleton className="mb-4 h-5 w-40" />
      <div className="grid grid-cols-1 gap-x-6 gap-y-4 sm:grid-cols-2">
        {Array.from({ length: fields }, (_, i) => (
          <div key={i} className="space-y-1.5">
            <Skeleton className="h-3 w-24" />
            <Skeleton className={`h-4 ${lineWidth(i)}`} />
          </div>
        ))}
      </div>
    </div>
  );
}

/** Stacked detail sections — a whole detail page body. */
export function DetailSkeleton({
  sections = 2,
  fieldsPerSection = 4,
}: Readonly<{ sections?: number; fieldsPerSection?: number }>) {
  return (
    <div className="space-y-4 sm:space-y-6">
      {Array.from({ length: sections }, (_, i) => (
        <DetailSectionSkeleton key={i} fields={fieldsPerSection} />
      ))}
    </div>
  );
}

/** Mirrors a kit form: label + input pairs, footer action buttons. */
export function FormSkeleton({ fields = 5 }: Readonly<{ fields?: number }>) {
  return (
    <div className="moto-content-surface rounded-2xl border p-4 sm:p-6">
      <div className="space-y-5">
        {Array.from({ length: fields }, (_, i) => (
          <div key={i} className="space-y-1.5">
            <Skeleton className="h-4 w-28" />
            <Skeleton className="h-11 w-full" />
          </div>
        ))}
      </div>
      <div className="mt-6 flex justify-end gap-2">
        <Skeleton className="h-9 w-24" />
        <Skeleton className="h-9 w-32" />
      </div>
    </div>
  );
}

/**
 * Stacked list rows (avatar + two text lines + trailing badge) inside the
 * canonical surface — for inboxes, request queues, and tab content.
 */
export function ListSkeleton({
  rows = 6,
  avatar = true,
}: Readonly<{ rows?: number; avatar?: boolean }>) {
  return (
    <div className="moto-content-surface divide-y divide-gray-100 overflow-hidden rounded-2xl border">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="flex items-center gap-3 px-4 py-3.5 sm:px-5">
          {avatar && (
            <Skeleton shape="round" className="h-9 w-9 flex-shrink-0" />
          )}
          <div className="min-w-0 flex-1 space-y-1.5">
            <Skeleton className={`h-4 ${lineWidth(i)}`} />
            <Skeleton className={`h-3 ${lineWidth(i + 3)}`} />
          </div>
          <Skeleton shape="round" className="h-6 w-16 flex-shrink-0" />
        </div>
      ))}
    </div>
  );
}

/**
 * Getrennte Auswahlzeilen mit Kompaktkarte und Aktionsplatz. Für Listen, die
 * aus `ChoiceTile` und einer Profil- oder Detailaktion bestehen, nicht für
 * durchgehende Listen mit Trennlinien.
 */
export function ChoiceTileListSkeleton({
  rows = 3,
  avatar = false,
}: Readonly<{ rows?: number; avatar?: boolean }>) {
  return (
    <div className="flex flex-col gap-2">
      {Array.from({ length: rows }, (_, index) => (
        <ChoiceTile key={index} as="div" className="gap-2">
          <div className="flex min-w-0 flex-1 items-center gap-3">
            {avatar ? (
              <Skeleton shape="round" className="h-8 w-8 shrink-0" />
            ) : null}
            <div className="min-w-0 flex-1">
              <Skeleton className={index % 2 === 0 ? "h-5 w-40" : "h-5 w-32"} />
              <Skeleton className="mt-0.5 h-4 w-24" />
            </div>
          </div>
          <Skeleton shape="control" className="h-8 w-16 shrink-0" />
        </ChoiceTile>
      ))}
    </div>
  );
}

/**
 * Gruppierte Verlaufskarten: Datum, Titel mit Dauer, zwei Fakten und eine
 * Zeitzeile. Entspricht einer Historie mit einzelnen Inhaltskarten, nicht
 * einer durchgehenden Liste.
 */
export function HistorySkeleton() {
  return (
    <div className="space-y-6">
      <div>
        <Skeleton className="mb-3 h-5 w-32" />
        <div className="space-y-3">
          {Array.from({ length: 2 }, (_, index) => (
            <div
              key={index}
              className="moto-content-surface rounded-xl border p-4"
            >
              <div className="mb-2 flex items-start justify-between gap-2">
                <Skeleton className={index === 0 ? "h-5 w-40" : "h-5 w-32"} />
                <Skeleton className="h-3 w-12" />
              </div>
              <div className="space-y-1">
                <Skeleton className="h-4 w-36" />
                <Skeleton className="h-4 w-24" />
              </div>
              <div className="mt-3 flex justify-between border-t border-gray-100 pt-3">
                <Skeleton className="h-3 w-24" />
                <Skeleton className="h-3 w-24" />
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
