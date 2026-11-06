"use client";

import { MobilePageHelpTarget } from "~/components/ui/mobile-page-help-target";
import type { TenantPageProps } from "./tenant-page-types";
export type { TenantPageProps, TenantPageTab } from "./tenant-page-types";
import { Info } from "lucide-react";
import type { PointerEvent, ReactNode, RefObject } from "react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { AnchoredPopover } from "~/components/ui/anchored-popover";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { useNotBookedSlots } from "./not-booked-slot";
import { useNotBookedPageModule } from "~/lib/not-booked-page-context";
import { errorAlertActions } from "~/components/ui/form-error-alert";
import { BackLink } from "~/components/ui/back-link";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { createLogger } from "~/lib/logger";
import dynamic from "next/dynamic";
import { cn } from "~/lib/utils";

const TenantPageHeaderSearch = dynamic(() =>
  import("./tenant-page-header-search").then(
    (module) => module.TenantPageHeaderSearch,
  ),
);

const TenantPageTabs = dynamic(() =>
  import("./tenant-page-tabs").then((module) => module.TenantPageTabs),
);

/**
 * Das Seitengerüst des Tenant-Portals. JEDE Seite unter
 * `app/[tenant]/(protected)` rendert genau dieses Gerüst als Wurzel
 * (Ausnahmen gibt es nicht.), ebenso jede Seite des Träger-, Schul- und
 * Operator-Portals nach der Anmeldung (`app/traeger`, #5146; `app/school`,
 * `app/operator`, #5604).
 *
 * Die Reihenfolge der Bausteine ist fest und liegt hier, nicht in der Seite:
 *
 *   1. Kopfkarte:  Titel  ...............  Aktionen
 *                  Statuszeile (echte Zahlen der Seite)
 *                  Suche + Filter
 *   2. Reiter:     horizontale Seitenreiter (immer dasselbe Bauteil)
 *   3. Inhalt:     Karten im 24-px-Rhythmus
 *
 * Damit kann eine Seite nicht mehr entscheiden, ob sie einen Kopf hat, wo ihre
 * Aktionen sitzen oder wie ihr Ladezustand aussieht. Sie liefert Daten, kein
 * Layout: kein eigenes `max-w`, kein `mx-auto`, kein eigenes Padding, kein
 * zweites `<main>`, keine eigene `<h1>`. Das Padding kommt aus der Shell.
 *
 * Es gibt bewusst KEINE Mini-Überschrift über dem Titel. Wo man ist, sagen die
 * Brotkrumen in der Kopfzeile und die Seitenleiste.
 */

/** Kurzes Vorbeifahren öffnet nichts; der Weg zum Link schließt nichts. */
const INFO_HOVER_OPEN_DELAY_MS = 150;
const INFO_HOVER_CLOSE_DELAY_MS = 250;

/**
 * Der Info-Knopf neben dem Seitentitel. Ein Popover statt eines Tooltips:
 * auf dem Telefon gibt es kein Hover, und die Erklärung darf einen Link
 * tragen. Escape und ein Klick daneben schließen sie wieder.
 *
 * Mit der Maus öffnet schon das Darüberfahren (#5682). Der Hinweis bleibt
 * offen, solange die Maus über Knopf oder Hinweis steht, und nimmt niemandem
 * den Fokus. Ein Klick darauf hält ihn fest, dann gilt das Verhalten von
 * Klick und Tastatur.
 *
 * Beim Öffnen per Klick oder Tastatur springt der Fokus in den Rahmen: er hängt
 * am Ende des Dokuments, und ohne den Sprung erreichte die Tastatur seinen Link
 * erst nach der ganzen Seite.
 */
function TenantPageInfo({
  title,
  children,
}: Readonly<{ title: string; children: ReactNode }>) {
  const [openMode, setOpenMode] = useState<"hover" | "pinned" | null>(null);
  const contentRef = useRef<HTMLDivElement>(null);
  const hoverTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const label = `Erklärung zu ${title}`;

  const clearHoverTimer = useCallback(() => {
    if (hoverTimerRef.current !== null) clearTimeout(hoverTimerRef.current);
    hoverTimerRef.current = null;
  }, []);
  useEffect(() => clearHoverTimer, [clearHoverTimer]);

  const handlePointerEnter = (event: PointerEvent) => {
    if (event.pointerType !== "mouse") return;
    clearHoverTimer();
    if (openMode !== null) return;
    hoverTimerRef.current = setTimeout(() => {
      setOpenMode((current) => current ?? "hover");
    }, INFO_HOVER_OPEN_DELAY_MS);
  };
  const handlePointerLeave = (event: PointerEvent) => {
    if (event.pointerType !== "mouse") return;
    clearHoverTimer();
    if (openMode !== "hover") return;
    hoverTimerRef.current = setTimeout(() => {
      setOpenMode((current) => (current === "hover" ? null : current));
    }, INFO_HOVER_CLOSE_DELAY_MS);
  };
  const handleTriggerClick = () => {
    clearHoverTimer();
    if (openMode === "hover") {
      // Der Rahmen ist schon offen, der Klick hält ihn nur fest. Wer per
      // Gewohnheit klickt, soll ihn nicht wieder zuklappen.
      setOpenMode("pinned");
      contentRef.current?.focus();
      return;
    }
    setOpenMode(openMode === null ? "pinned" : null);
  };

  return (
    <AnchoredPopover
      open={openMode !== null}
      onOpenChange={(next) => {
        clearHoverTimer();
        setOpenMode(next ? "pinned" : null);
      }}
      ariaLabel={label}
      initialFocusRef={openMode === "pinned" ? contentRef : undefined}
      restoreFocusOnClose={openMode === "pinned"}
      panelHandlers={{
        onPointerEnter: handlePointerEnter,
        onPointerLeave: handlePointerLeave,
        // Fokus im Rahmen, per Klick zum Markieren oder per Tab: ab dann
        // schließt die Maus ihn nicht mehr, und Escape führt den Fokus zurück.
        onFocus: () => {
          clearHoverTimer();
          setOpenMode("pinned");
        },
      }}
      preferredWidth={360}
      renderTrigger={({ ref, open: expanded, panelId }) => (
        <Button
          ref={ref}
          type="button"
          variant="ghost"
          size="icon"
          aria-label={label}
          aria-expanded={expanded}
          aria-controls={expanded ? panelId : undefined}
          onClick={handleTriggerClick}
          onPointerEnter={handlePointerEnter}
          onPointerLeave={handlePointerLeave}
          // Sichtbar 32 px wie die übrigen Symbolknöpfe; die Trefferfläche
          // reicht per Pseudoelement auf 44 px, die Untergrenze für eine
          // Touch-Fläche, ohne die Titelzeile höher zu machen.
          className="relative shrink-0 text-gray-500 after:absolute after:-inset-1.5 after:content-['']"
        >
          <Info aria-hidden="true" className="h-[18px] w-[18px]" />
        </Button>
      )}
    >
      {() => (
        <div
          ref={contentRef}
          tabIndex={-1}
          className="p-4 text-sm leading-6 text-gray-700 outline-none"
        >
          {children}
        </div>
      )}
    </AnchoredPopover>
  );
}

/**
 * Ein Wert-Label-Paar der Statuszeile. Trennzeichen setzt `TenantPageStats`.
 *
 * Höchstens DREI Paare: die Statuszeile ist eine Orientierung, keine
 * Kennzahlenleiste. Sechs Paare (gemessen auf /staff) liest niemand mehr als
 * Satz — sie sind eine Tabelle ohne Kopf. Was über drei hinausgeht, trägt
 * die Fläche darunter (Kacheln, Karten, Tabelle); das Gerüst schneidet hier
 * ab, damit das Budget nicht pro Seite neu verhandelt wird.
 */
const MAX_STATS_ITEMS = 3;

export function TenantPageStats({
  items,
}: Readonly<{
  items: readonly { readonly value: ReactNode; readonly label: string }[];
}>) {
  return (
    <span className="inline-flex flex-wrap items-center gap-x-1.5 gap-y-1">
      {items.slice(0, MAX_STATS_ITEMS).map((item, index) => (
        // Paare ohne Label (nur ein Wert) teilen sich das leere Label; erst
        // die Position macht den Schlüssel eindeutig.
        <span
          key={`${index}-${item.label}`}
          className="inline-flex items-center gap-1.5"
        >
          {index > 0 && (
            <span aria-hidden="true" className="text-gray-300">
              ·
            </span>
          )}
          <span className="font-medium text-gray-900 tabular-nums">
            {item.value}
          </span>
          <span>{item.label}</span>
        </span>
      ))}
    </span>
  );
}

/**
 * Eine Bedienhöhe im Seitenkopf. Ohne diese Klammer stehen dort 32, 36 und
 * 40 px nebeneinander, je nachdem welches Kit-Bauteil eine Seite gerade
 * greift — gemessen auf /statistics: Zeitraum 32, Export 36, Filter 40.
 * Der Selektor greift auch verschachtelte Auslöser, deshalb Nachfahren und
 * nicht nur direkte Kinder.
 */
// Eine Bedienhöhe in der Kopfkarte: 40 px für Schaltflächen, Links in
// Button-Optik (`ButtonLink`), Auswahl- UND Eingabefelder, auf dem Telefon
// 44 px (die Untergrenze für eine Touch-Fläche, dieselbe wie die
// Fingerregel des Kits). Das Eingabefeld fehlte hier anfangs, deshalb stand das
// Suchfeld mit 42 px neben Filterknöpfen mit 36 — auf acht Seiten derselbe
// Versatz; ebenso stand ein `ButtonLink` mit 32 oder 36 px neben 40-px-Knöpfen
// (#5531). Der `SegmentedControl` behauptet seine Segmenthöhe mit `!` und
// kommt mitsamt Spur ebenfalls auf 40 px — siehe dort.
const CONTROL_HEIGHT =
  "[&_button]:h-10 [&_a[data-button-link]]:h-10 [&_select]:h-10 [&_input]:h-10 max-sm:[&_button]:h-11 max-sm:[&_a[data-button-link]]:h-11 max-sm:[&_select]:h-11 max-sm:[&_input]:h-11";

/** Der Rückweg ohne eigenes Ziel: die Unterseiten der Datenverwaltung. */
const DEFAULT_BACK_HREF = "/database";
const DEFAULT_BACK_LABEL = "Zurück zur Datenverwaltung";

/**
 * `BackLink` im Tenant-Portal: löst den Pfad für die Schule auf. Das Gerüst
 * setzt ihn über den Titel; nur eine Fläche ohne Kopfkarte (ein Skelett vor
 * dem Gerüst) rendert ihn selbst.
 */
export function TenantBackLink({
  href,
  label,
  className = "mb-2",
}: Readonly<{ href?: string; label?: string; className?: string }>) {
  const tenantPath = useTenantAwarePath();
  return (
    <BackLink
      href={tenantPath(href ?? DEFAULT_BACK_HREF)}
      // Ohne eigenen Text nennt der Rückweg die Datenverwaltung nur, wenn er
      // auch dorthin führt.
      label={label ?? (href ? "Zurück" : DEFAULT_BACK_LABEL)}
      className={className}
    />
  );
}

/**
 * Jede Seite der Mitarbeiter-Oberfläche rendert dieses Gerüst. Gehört die
 * Seite zu einem Modul, das die Schule nicht gebucht hat (Modul-Register,
 * #4894), steht unter ihrem Titel statt des Inhalts die Erklärung: zu
 * welchem Modul sie gehört und wer handelt. Welches Modul gesperrt ist,
 * ermittelt `NotBookedPageProvider` im Layout.
 */
export function TenantPage(props: TenantPageProps) {
  const notBookedModule = useNotBookedPageModule();
  const NotBookedState = useNotBookedSlots()?.State;
  if (notBookedModule && NotBookedState) {
    return (
      <TenantPageView
        title={props.title}
        back={props.back}
        backHref={props.backHref}
        backLabel={props.backLabel}
        printHideHeader={props.printHideHeader}
        testId={props.testId}
      >
        <SectionCard>
          <NotBookedState module={notBookedModule} />
        </SectionCard>
      </TenantPageView>
    );
  }
  return <TenantPageView {...props} />;
}

const logger = createLogger({ component: "TenantPage" });

/** Zugänglicher Name eines Bedienelements, ohne das „+“ der Anlegen-Knöpfe. */
function controlNames(root: HTMLElement | null): Set<string> {
  const names = new Set<string>();
  root?.querySelectorAll("button, a[href]").forEach((element) => {
    for (const raw of [
      element.getAttribute("aria-label"),
      element.textContent,
    ]) {
      const name = raw
        ?.replace(/^\s*\+\s*/, "")
        .replace(/\s+/g, " ")
        .trim();
      if (name) names.add(name.toLowerCase());
    }
  });
  return names;
}

/**
 * Wächter für #5034: Ein Leerzustand wiederholt keine Kopfaktion. Steht
 * „Neue Notiz“ im Kopf und noch einmal im Leerzustand, sieht die leere Seite
 * zwei gleiche Knöpfe. Eine Leerzustand-Aktion ist nur für eine ANDERE
 * Handlung da („Filter zurücksetzen“). Außerhalb der Produktion meldet das
 * Gerüst eine Wiederholung, damit sie im Test und beim Entwickeln auffällt.
 */
function useEmptyActionGuard(
  headRef: RefObject<HTMLElement | null>,
  bodyRef: RefObject<HTMLElement | null>,
  title: string,
  active: boolean,
) {
  const reportedViolations = useRef(new Set<string>());

  useEffect(() => {
    if (!active || process.env.NODE_ENV === "production") return;
    const head = controlNames(headRef.current);
    const repeated = [...controlNames(bodyRef.current)].filter((name) =>
      head.has(name),
    );
    const repeatedAction = repeated[0];
    if (repeatedAction !== undefined) {
      const violation = JSON.stringify([title, repeatedAction]);
      if (reportedViolations.current.has(violation)) return;
      reportedViolations.current.add(violation);
      logger.error("tenant_page_empty_action_repeats_head_action", {
        page: title,
        action: repeatedAction,
      });
    }
  });
}

function TenantPageView({
  title,
  stats,
  statsLoading = false,
  info,
  actions,
  leading,
  prominent = false,
  back = false,
  backHref,
  backLabel,
  printHideHeader = false,
  search,
  filters,
  activeFilters,
  onClearAllFilters,
  overflowMenu,
  badge,
  filterVariant,
  filterSections,
  desktopFiltersFrom,
  activeFilterDisplay,
  primaryAction,
  searchSlot,
  searchSlotHeight = "controls",
  tabs,
  error,
  loading = false,
  empty,
  children,
  loadingLabel,
  overlays,
  testId,
}: TenantPageProps) {
  const hasSearchRow =
    search !== undefined ||
    (filters?.length ?? 0) > 0 ||
    (overflowMenu?.length ?? 0) > 0 ||
    primaryAction != null ||
    badge !== undefined;

  const headRef = useRef<HTMLElement>(null);
  const bodyRef = useRef<HTMLDivElement>(null);
  useEmptyActionGuard(
    headRef,
    bodyRef,
    title,
    Boolean(empty?.action) && !loading && !error,
  );

  const statusLine = statsLoading ? (
    <Skeleton className="h-4 w-56" />
  ) : (
    (stats ?? null)
  );

  return (
    // `flex-1 flex-col`: die Seite wächst in der Inhaltshülle der Shell bis
    // zur Unterkante des Bildschirms, damit der Rumpf darunter die Höhe
    // füllen kann (siehe `.moto-tenant-body`).
    <div className="flex w-full flex-1 flex-col" data-testid={testId}>
      {/* Die Kopfkarte trägt Titel, Statuszeile, Aktionen und die Such- und
          Filterzeile. Sie schließt eng um ihren Inhalt: 20 px Rand, 4 px
          zwischen Titel und Statuszeile, 16 px vor der Suchzeile. Kein
          reservierter Leerraum darunter — genau der war der tote Streifen. */}
      {/* Eine Fläche von Titel bis Reiter. Die Reiter lagen vorher frei auf
          dem gemusterten Grund und sahen aus wie nachträglich dazwischen
          geschoben; sie sind jetzt die letzte Zeile des Kopfes. */}
      {/* Auf kleinen Bildschirmen liegt der Kopf flach auf dem gemusterten
          Grund. Die spezielle Klasse überschreibt die ungeschichtete
          Flächenklasse auch dann zuverlässig, wenn Tailwind-Utilities in
          einer CSS-Layer stehen. */}
      <header
        ref={headRef}
        className={cn(
          "moto-content-surface compact:p-4 rounded-2xl border p-5 max-sm:p-4",
          printHideHeader && "print:hidden",
        )}
      >
        {back && <TenantBackLink href={backHref} label={backLabel} />}
        <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2 max-sm:gap-x-2">
          {/* `flex-1 basis-0`: der Titelblock nimmt, was die Aktionen übrig
              lassen, statt mit seiner natürlichen Breite zu ringen. Vorher
              schob eine zweizeilige Statuszeile auf dem Telefon selbst das
              einzelne Kebab-Menü in eine eigene, linksbündige Zeile -- und
              die Kopfkarte wuchs um genau den toten Streifen, den sie auf
              dem Desktop nicht mehr hat. */}
          <div className="flex min-w-0 flex-1 basis-0 items-center gap-3 max-sm:min-w-[6rem]">
            {leading}
            <div className="min-w-0">
              <div className="flex items-center gap-1">
                <h1
                  className={cn(
                    "min-w-0 font-semibold tracking-tight text-balance break-words text-gray-900",
                    // Auch auf dem Telefon sichtbar: unter lg gibt es keine
                    // Shell-Kopfzeile mehr (wie in der Eltern-App), die
                    // Kopfkarte ist die einzige Trägerin des Seitennamens.
                    // Der Desktop-Grad ist der Grundwert, Telefon und die
                    // Dichte-Stufe kleiner Bildschirme ziehen ihn je eine
                    // Stufe herunter (#2824). Beide Varianten schliessen
                    // einander aus, es gilt also immer genau eine.
                    prominent
                      ? "compact:text-2xl text-[28px] leading-tight max-sm:text-2xl"
                      : "compact:text-xl text-2xl leading-tight max-sm:text-xl",
                  )}
                >
                  {title}
                </h1>
                {info != null && (
                  <TenantPageInfo title={title}>{info}</TenantPageInfo>
                )}
              </div>
              {/* Ein div, kein p: `stats` ist beliebiges ReactNode, und
                  Seiten setzen Blockelemente hinein (das LocationBadge der
                  Kinderseite rendert ein div). In einem p wäre das ungültiges
                  HTML und ein Hydration-Fehler. */}
              {statusLine != null && (
                <div className="mt-1 text-sm leading-5 text-gray-600">
                  {statusLine}
                </div>
              )}
            </div>
          </div>
          {/* Eine Bedienhöhe und eine gemeinsame Mitte für Seitenaktionen
              und Hilfe. Symbolhüllen sind flex, damit die Text-Grundlinie
              keinen zusätzlichen Abstand unter dem Knopf erzeugt. */}
          <div
            className={cn(
              // Die Aktionen stehen neben dem Titel, solange sie neben
              // seiner Mindestbreite (6rem unter sm) Platz haben; sonst
              // brechen sie als Gruppe in eine eigene Zeile um und stehen
              // dort rechts (`ml-auto`), wie auf dem Desktop. Linksbündig
              // stand der tote Raum rechts neben den Knöpfen. `max-w-full`:
              // Sind die Knöpfe zusammen breiter als der Kopf, brechen sie
              // in der Gruppe um, statt die Seite seitlich scrollen zu
              // lassen (#4975).
              actions
                ? "ml-auto flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2"
                : "contents",
              "[&>div[data-icon-only]]:inline-flex",
              // Unter sm gilt: JEDE Zeile ist voll. Textknöpfe bekommen
              // eine eigene, volle Zeile (auch ein einzelner — rechtsbündig
              // neben Leerraum las er sich als toter Bereich); reine
              // Symbolknöpfe (das Kebab-Menü, `Button size="icon"`, per
              // `data-icon-only`) behalten ihr Maß und bleiben in der
              // Titel-/Statuszeile, die ihr Text füllt.
              "max-sm:contents",
              "max-sm:[&>*:not([data-icon-only])]:order-last max-sm:[&>*:not([data-icon-only])]:basis-full",
              CONTROL_HEIGHT,
            )}
          >
            {actions}
            <MobilePageHelpTarget />
          </div>
        </div>

        {/* Der Reiter bestimmt, WAS man ansieht; die Suche filtert DARIN.
            Deshalb steht er über der Suchzeile — darunter liest er sich als
            ein weiteres Filterelement. */}
        {tabs && <TenantPageTabs {...tabs} />}

        {(searchSlot ?? hasSearchRow) && (
          <TenantPageHeaderSearch
            search={search}
            filters={filters}
            activeFilters={activeFilters}
            onClearAllFilters={onClearAllFilters}
            overflowMenu={overflowMenu}
            badge={badge}
            filterVariant={filterVariant}
            filterSections={filterSections}
            desktopFiltersFrom={desktopFiltersFrom}
            activeFilterDisplay={activeFilterDisplay}
            primaryAction={primaryAction}
            searchSlot={searchSlot}
            searchSlotHeight={searchSlotHeight}
            hasSearchRow={hasSearchRow}
          />
        )}
      </header>

      {/* Der Rumpf füllt die Höhe des Bildschirms: seine letzte Fläche
          wächst bis zur Unterkante (`.moto-tenant-body` in globals.css).
          Vorher endete eine Seite mit einem Eintrag oder mit dem Leersatz
          nach 60 px Karte, und darunter stand der halbe Bildschirm nacktes
          Punktraster -- ein anderer Umriss auf jeder Seite. Jetzt haben die
          Seite mit einer Zeile, die mit dreißig und die leere denselben. */}
      <div
        ref={bodyRef}
        className="moto-tenant-body compact:mt-4 compact:space-y-4 mt-6 space-y-6 max-sm:mt-4 max-sm:space-y-3"
      >
        <TenantPageBody
          loading={loading}
          loadingLabel={loadingLabel ?? `Der Bereich „${title}“ wird geladen…`}
          error={error}
          empty={empty}
        >
          {children}
        </TenantPageBody>
      </div>
      {overlays}
    </div>
  );
}

/**
 * Titel und Beschreibung des Leerzustands als EIN Satzgefüge. Titel ohne
 * Schlusszeichen bekommen einen Punkt, damit „Keine Räume vorhanden Legen
 * Sie…" nicht entsteht.
 */
function joinEmptySentence(title: string, description?: string): string {
  const trimmed = title.trim();
  const punctuated = /[.!?…]$/.test(trimmed) ? trimmed : `${trimmed}.`;
  return description ? `${punctuated} ${description}` : punctuated;
}

/**
 * Fehler, Laden und Leerzustand an EINER Stelle, damit sie auf keiner Seite
 * mehr als `return null`, freier Spinner oder „Wird geladen…"-Fließtext
 * auftauchen.
 */
function TenantPageBody({
  loading,
  loadingLabel,
  error,
  empty,
  children,
}: Readonly<{
  loading: boolean | ReactNode;
  /** Vorlesbarer Text des Ladezustands, aus dem Seitentitel gebildet. */
  loadingLabel: string;
  error?: TenantPageProps["error"];
  empty?: TenantPageProps["empty"];
  children?: ReactNode;
}>) {
  const NotBookedState = useNotBookedSlots()?.State;
  // Eine Anfrage, die das Backend mit `module.not_booked` ablehnt, ist kein
  // Fehler der Seite: die Schule hat das Modul nicht gebucht (#4894). Statt
  // der roten Meldung steht der erklärende Zustand.
  const notBookedModule =
    typeof error === "object" && error !== null && "notBookedModule" in error
      ? error.notBookedModule
      : undefined;
  if (notBookedModule && NotBookedState) {
    return (
      <SectionCard>
        <NotBookedState module={notBookedModule} />
      </SectionCard>
    );
  }
  const errorParts =
    typeof error === "string"
      ? { message: error }
      : error && !("action" in error || "keepContent" in error)
        ? { message: error.message, action: errorAlertActions(error) }
        : (error ?? undefined);
  if (errorParts && !errorParts.keepContent) {
    return (
      <Alert
        type="error"
        message={errorParts.message}
        action={errorParts.action}
      />
    );
  }
  const errorBanner = errorParts ? (
    <Alert
      type="error"
      message={errorParts.message}
      action={errorParts.action}
    />
  ) : null;
  if (loading) {
    return (
      <>
        {errorBanner}
        <div
          className={loading === true ? "space-y-3" : undefined}
          role="status"
          aria-busy="true"
          aria-live="polite"
          aria-label={loadingLabel}
        >
          {loading === true ? (
            <>
              <Skeleton className="h-24 w-full rounded-2xl" />
              <Skeleton className="h-24 w-full rounded-2xl" />
              <Skeleton className="h-24 w-full rounded-2xl" />
            </>
          ) : (
            loading
          )}
        </div>
      </>
    );
  }
  if (empty) {
    // Auf derselben Fläche wie der Inhalt, den sie ersetzt: sonst steht der
    // Leerzustand als einziger Seitenzustand ohne Karte auf dem Hintergrund
    // und die Seite sieht leer statt aufgeräumt aus. Laden (Skelettkarten)
    // und Fehler (getönte Fläche) tragen ihre Fläche schon.
    //
    // Zwei Formen: mit Aktion (nächster Schritt) oder Symbol (benannter
    // Zustand wie ForbiddenPage/FeatureOffPage) bekommt der Leerzustand die
    // volle Bühne (EmptyState). Eine schmucklose Feststellung ist EIN Satz
    // in der Karte, wie im Eltern-Portal — eine zentrierte Inszenierung
    // über einem Sachverhalt, an dem niemand etwas tun kann, ist eine
    // Aufgabe, die keine ist.
    return (
      <>
        {errorBanner}
        <SectionCard>
          {empty.action || empty.icon ? (
            <EmptyState
              title={empty.title}
              description={empty.description}
              icon={empty.icon}
              action={empty.action}
            />
          ) : (
            <p className="text-sm leading-6 text-gray-600">
              {joinEmptySentence(empty.title, empty.description)}
            </p>
          )}
        </SectionCard>
      </>
    );
  }
  return (
    <>
      {errorBanner}
      {children}
    </>
  );
}
