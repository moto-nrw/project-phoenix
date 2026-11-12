"use client";

import type { CSSProperties, MouseEvent, ReactNode } from "react";
import { useId, useState } from "react";
import { ChevronDown } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { Button } from "~/components/ui/button";
import { IconTile } from "~/components/ui/icon-tile";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import {
  SECTION_CARD_PADDING,
  SectionCardSurface,
} from "~/components/ui/section-card-surface";
import type { MotoConcept } from "~/lib/moto-concepts";
import type { MotoDuotoneTone } from "~/lib/location-helper";
import { cn } from "~/lib/utils";

// Mit `flush` oder `footer` liegt das Polster auf den Zonen statt auf der
// Fläche, damit Rumpf und Fußzone bis an den Rand laufen können, ohne das
// Polster per Negativrand wieder aufzuheben.
const ZONE_PADDING_X = "compact:px-4 px-5 max-sm:px-4";
const ZONE_PADDING_TOP = "compact:pt-4 pt-5 max-sm:pt-4";
const ZONE_PADDING_BOTTOM = "compact:pb-4 pb-5 max-sm:pb-4";

// Die Titelstufen einer Karte. `lg` steht als hervorgehobener Abschnitt mittig
// neben der Symbolfläche, `page` und `display` tragen den Seitenkopf des
// Elternportals (`display` die Begrüßung auf der Startseite).
const TITLE_SIZES = {
  sm: "text-sm",
  md: "text-base",
  lg: "flex min-h-9 items-center text-xl leading-tight tracking-tight sm:min-h-10",
  page: "text-xl leading-tight tracking-tight break-words sm:text-2xl",
  display: "text-2xl leading-tight tracking-tight break-words sm:text-[28px]",
} as const;

function SectionCardLoadingRows({ rows }: Readonly<{ rows: number }>) {
  return (
    <dl className="space-y-3">
      {Array.from({ length: rows }, (_, index) => (
        <div key={index} aria-hidden="true">
          <dt className="text-xs text-gray-500">
            <span className="block h-3 w-20 animate-pulse rounded bg-gray-200 motion-reduce:animate-none" />
          </dt>
          <dd className="mt-0.5 text-sm font-medium text-gray-900">
            <span className="block h-4 w-2/3 animate-pulse rounded bg-gray-200 motion-reduce:animate-none" />
          </dd>
        </div>
      ))}
    </dl>
  );
}

const CONCEPT_ICON_COLORS: Record<MotoDuotoneTone, string> = {
  blue: "text-moto-blue-strong [--moto-icon-secondary:var(--color-moto-blue-light)]",
  timeTracking:
    "text-moto-time-tracking-strong [--moto-icon-secondary:var(--color-moto-time-tracking-light)]",
  teal: "text-moto-teal-strong [--moto-icon-secondary:var(--color-moto-teal-light)]",
  magenta:
    "text-moto-magenta-strong [--moto-icon-secondary:var(--color-moto-magenta-light)]",
  orange:
    "text-moto-orange-strong [--moto-icon-secondary:var(--color-moto-orange)]",
  amber:
    "text-moto-amber [--moto-icon-secondary:var(--color-moto-amber-light)]",
  purple:
    "text-moto-purple-strong [--moto-icon-secondary:var(--color-moto-purple-light)]",
  greenDeep:
    "text-moto-green-strong [--moto-icon-secondary:var(--color-moto-green-light)]",
  greenVivid:
    "text-moto-green-vivid [--moto-icon-secondary:var(--color-moto-green-muted)]",
  indigo:
    "text-moto-indigo-strong [--moto-icon-secondary:var(--color-moto-indigo-light)]",
  red: "text-moto-red-strong [--moto-icon-secondary:var(--color-white)]",
  coral:
    "text-moto-coral-strong [--moto-icon-secondary:var(--color-moto-coral-light)]",
  cyan: "text-moto-cyan-strong [--moto-icon-secondary:var(--color-moto-cyan-light)]",
  navy: "text-moto-navy-strong [--moto-icon-secondary:var(--color-moto-navy-light)]",
  mint: "text-moto-mint-strong [--moto-icon-secondary:var(--color-moto-mint-light)]",
  wine: "text-moto-wine-strong [--moto-icon-secondary:var(--color-moto-wine-light)]",
  gold: "text-moto-gold-strong [--moto-icon-secondary:var(--color-moto-gold-light)]",
  petrol:
    "text-moto-petrol-strong [--moto-icon-secondary:var(--color-moto-petrol-light)]",
  neutral: "text-gray-700 [--moto-icon-secondary:var(--color-gray-400)]",
  stone:
    "text-moto-stone-strong [--moto-icon-secondary:var(--color-moto-stone-light)]",
};

function SectionCardConceptIcon({
  concept,
}: Readonly<{ concept: MotoConcept }>) {
  return (
    <IconTile
      size="section"
      icon={
        <MotoConceptIcon
          concept={concept}
          colorMode="class"
          className={CONCEPT_ICON_COLORS[concept.tone]}
        />
      }
    />
  );
}

/**
 * The disclosure toggle of a collapsible `SectionCard`. Exported for a
 * section whose expanded head is drawn by its content (the block roster on a
 * released room's page, #3281), so both states carry the same control.
 */
export function SectionCollapseToggle({
  title,
  collapsed,
  controls,
  onToggle,
}: Readonly<{
  title?: string;
  collapsed: boolean;
  /** Id of the body the toggle shows or hides, while it is rendered. */
  controls?: string;
  onToggle: () => void;
}>) {
  return (
    <Button
      type="button"
      // Zeigt die Rückmeldung auch, wenn die Maus über dem übrigen Kopf liegt:
      // der ganze Kopf schaltet (#5021).
      variant="disclosure"
      size="icon"
      data-section-collapse-toggle=""
      className="shrink-0"
      aria-label={
        collapsed
          ? `${title ?? "Abschnitt"} ausklappen`
          : `${title ?? "Abschnitt"} einklappen`
      }
      aria-expanded={!collapsed}
      aria-controls={controls}
      onClick={onToggle}
    >
      <ChevronDown
        className={`h-4 w-4 transition-transform ${collapsed ? "-rotate-90" : ""}`}
        aria-hidden="true"
      />
    </Button>
  );
}

// Eigene Bedienelemente im Kopf (Aktionen, Links, Felder) behalten ihren
// Klick und schalten die Karte nicht um.
const OWN_CONTROL_SELECTOR = [
  "a[href]",
  "button",
  "input",
  "select",
  "textarea",
  "label",
  "summary",
  "[contenteditable='true']",
  "[role='button']",
  "[role='link']",
  "[role='checkbox']",
  "[role='switch']",
  "[role='menuitem']",
  "[role='tab']",
].join(",");

function toggleFromHeadClick(event: MouseEvent<HTMLDivElement>) {
  const head = event.currentTarget;
  const target = event.target;
  if (event.defaultPrevented || !(target instanceof Element)) return;
  // React reicht Klicks aus Portalen (Menüs, Dialoge einer Kopf-Aktion) durch
  // den Komponentenbaum hoch; sie liegen nicht im Kopf und zählen nicht.
  if (!head.contains(target)) return;
  const ownControl = target.closest(OWN_CONTROL_SELECTOR);
  if (ownControl && head.contains(ownControl)) return;
  // Wer Text im Kopf markiert, will ihn kopieren, nicht umschalten.
  const selection = globalThis.getSelection?.();
  if (selection && !selection.isCollapsed && selection.toString() !== "") {
    return;
  }
  head
    .querySelector<HTMLButtonElement>("[data-section-collapse-toggle]")
    ?.click();
}

/**
 * The head row of a collapsible section: a click anywhere on it (icon, title,
 * badge, description) works the `SectionCollapseToggle` inside, so the small
 * chevron is not the only target (#5021). The toggle stays the one button —
 * one tab stop, Enter and Space, the accessible name — and the heading stays a
 * heading. Header actions keep their own click. Without a toggle inside the
 * head is a plain container: no pointer, no click.
 */
export function SectionCollapseHead({
  className,
  children,
}: Readonly<{ className?: string; children: ReactNode }>) {
  return (
    // oxlint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-static-element-interactions -- Mausfläche um den SectionCollapseToggle; Tastatur und Screenreader bedienen den Knopf selbst
    <div
      className={cn(
        "group/collapse-head has-[[data-section-collapse-toggle]]:cursor-pointer",
        className,
      )}
      onClick={toggleFromHeadClick}
    >
      {children}
    </div>
  );
}

/** Unter `lg` liegt unten die Navigation, der Fuß klebt darüber. */
const STICKY_FOOTER =
  "sticky bottom-[calc(6rem+env(safe-area-inset-bottom))] lg:bottom-0";

/**
 * The canonical content section of the calm design language the app converged
 * on (Anmeldungen, Planung, Personal-Stammdaten, Elternportal): a
 * `moto-content-surface` card with a blue uppercase kicker, a `text-base`
 * title, a muted description and an action slot on the right.
 *
 * Use this instead of hand-rolling `<section className="rounded-2xl border …">`
 * with a local header component. It is the only card head in the kit (#5581):
 * `concept` draws the concept icon tile, `flush` lets a table or list run to
 * the card edge, `footer` adds a bottom zone, `padding="none"` drops the
 * surface padding for content that brings its own rows, and `loading` shows
 * field-shaped bars under the real head.
 *
 * `collapsible` opts into the disclosure toggle the staff Stammdaten tab uses
 * (many sections on one long page). Sections that are already short — the
 * parents portal's — leave it off. The open state is uncontrolled by default
 * (`defaultCollapsed`); pass `collapsed` when a parent owns it, e.g. to open
 * the section that holds a deep-linked field or to expand every section at
 * once (settings page, #2830).
 */
export function SectionCard({
  kicker,
  title,
  titleBadge,
  description,
  icon: Icon,
  concept,
  leading,
  action,
  actions,
  inlineActions = false,
  bare = false,
  testId,
  collapsible = false,
  defaultCollapsed = false,
  collapsed: collapsedProp,
  onCollapsedChange,
  headingLevel = 2,
  namedRegion = false,
  titleSize = "md",
  titleClassName,
  bodyClassName,
  className = "",
  /** `visible` lässt Dropdowns aus der Kopfzeile über den Kartenrand ragen. */
  overflow = "hidden",
  flush = false,
  footer,
  stickyFooter = false,
  padding = "default",
  loading = false,
  loadingRows = 3,
  as = "section",
  hoverElevated = false,
  children,
  id,
  style,
}: Readonly<{
  kicker?: string;
  /**
   * Ohne Titel rendert die Karte keinen Kopf und ist die reine Inhaltsfläche
   * einer Seite. Genau dafür haben Seiten sich vorher `<section
   * className="moto-content-surface …">` selbst gebaut.
   */
  title?: string;
  /** Small badge next to the title, e.g. a `StatusBadge` with a count. */
  titleBadge?: ReactNode;
  description?: ReactNode;
  icon?: LucideIcon;
  /** Concept icon tile of a fachliches Konzept (`lib/concepts/`). */
  concept?: MotoConcept;
  /** Existing icon tile or other leading visual for non-Lucide icon systems. */
  leading?: ReactNode;
  /** Single header action. `actions` is the multi-element form. */
  action?: ReactNode;
  actions?: ReactNode;
  /**
   * Aktionen bleiben auch auf dem Telefon neben dem Titel, statt als eigene
   * Zeile darunter umzubrechen. Passt für einen einzelnen kompakten Knopf
   * (Startseiten-Kacheln); breite Aktionsgruppen behalten den Umbruch.
   */
  inlineActions?: boolean;
  /**
   * Ohne eigene Kartenfläche, wenn der Inhalt selbst schon aus Karten besteht
   * (eine Reihe `StatCard`, eine Liste `TileCard`). Sonst steht eine weiße
   * Karte auf einer weißen Karte und beide Ränder werden schwach. Dieselbe
   * Entscheidung trifft das Eltern-Portal mit `ParentSection bare`.
   */
  bare?: boolean;
  /** Testanker; erspart eine zusätzliche Hülle nur für den Selektor. */
  testId?: string;
  collapsible?: boolean;
  defaultCollapsed?: boolean;
  /** Controlled open state; `onCollapsedChange` then reports the requested one. */
  collapsed?: boolean;
  onCollapsedChange?: (collapsed: boolean) => void;
  /**
   * 1 for a page's primary section, 2 (default) for the rest. 4 and 5 for a
   * card inside a section that already carries its own h3 (a Rückmeldung in
   * a detail block) and the parts of such a card.
   */
  headingLevel?: 1 | 2 | 3 | 4 | 5;
  /**
   * Names the section after its title (`aria-labelledby`), so a screen
   * reader lists it as a region. For a card that stands for itself among
   * others of its kind, e.g. one Rückmeldung per child.
   */
  namedRegion?: boolean;
  /** Schriftstufe des Titels; die Größe kommt nie über `titleClassName`. */
  titleSize?: keyof typeof TITLE_SIZES;
  titleClassName?: string;
  /** Overrides the default `mt-4` spacing above the body. */
  bodyClassName?: string;
  className?: string;
  overflow?: "hidden" | "visible";
  /**
   * Rumpf ohne Seiten- und Unterrand: eine Tabelle oder Liste mit eigener
   * Zellpolsterung läuft bis an die Kartenkante. Der Kopf behält sein Polster.
   */
  flush?: boolean;
  /**
   * Fußzone unter dem Rumpf, randlos mit Trennlinie (Aufklapper, Aktionen
   * zum Inhalt). Sie klappt mit der Karte ein.
   */
  footer?: ReactNode;
  /**
   * Die Fußzone klebt am unteren Bildschirmrand, solange die Karte höher als
   * der Bildschirm ist (langes Formular mit Speichern im Fuß). Am Ende der
   * Karte liegt sie an ihrem Platz. Die Karte schneidet dann nichts ab.
   */
  stickyFooter?: boolean;
  /** `none`: Fläche ohne Polster, wenn der Inhalt eigene Zeilen mitbringt. */
  padding?: "default" | "none";
  /** Zeigt Balken in Feldform statt des Inhalts; der Kopf bleibt echt. */
  loading?: boolean;
  loadingRows?: number;
  /**
   * `article` für eine Karte, die für sich allein steht und in einer Reihe
   * gleichartiger Karten liegt (eine Serie im Stundenplan).
   */
  as?: "section" | "article";
  /**
   * Hebt die Karte beim Hovern an wie eine `TileCard`, ohne sie klickbar zu
   * machen. Für Karten, deren Bedienelemente im Inhalt liegen; ist die ganze
   * Karte ein Ziel, ist `TileCard` richtig.
   */
  hoverElevated?: boolean;
  children?: ReactNode;
  id?: string;
  /**
   * Inline-Stil der Fläche, für CSS-Variablen, die der Inhalt liest (die
   * Spaltenvorlage eines Rasters). Nicht für Farbe oder Maß der Karte.
   */
  style?: CSSProperties;
}>) {
  const [collapsedState, setCollapsedState] = useState(defaultCollapsed);
  const collapsed = collapsedProp ?? collapsedState;
  const bodyId = useId();
  const headingId = useId();
  const Heading = `h${headingLevel}` as "h1" | "h2" | "h3" | "h4" | "h5";
  const headerActions = actions ?? action;
  const open = !(collapsible && collapsed);
  const hasBody =
    loading || (children != null && children !== false && children !== "");
  const showBody = hasBody && open;
  const showFooter = footer != null && footer !== false && open;
  const leadingVisual =
    leading ?? (concept ? <SectionCardConceptIcon concept={concept} /> : null);
  const hasHeader =
    title != null ||
    kicker != null ||
    description != null ||
    leadingVisual != null ||
    Icon != null ||
    headerActions != null ||
    collapsible;
  // Zonen tragen das Polster, wenn ein Teil bis an den Rand laufen soll.
  const zoned = !bare && padding === "default" && (flush || footer != null);
  let surfacePadding = "";
  if (padding === "default" && !zoned) surfacePadding = SECTION_CARD_PADDING;
  const bodySpacing =
    hasHeader && !bare ? (bodyClassName ?? "compact:mt-3 mt-4") : bodyClassName;
  const bodyClasses = zoned
    ? cn(
        !flush && ZONE_PADDING_X,
        !flush && !hasHeader && ZONE_PADDING_TOP,
        !flush && !showFooter && ZONE_PADDING_BOTTOM,
        bodySpacing,
      )
    : bodySpacing;

  return (
    <SectionCardSurface
      id={id}
      style={style}
      data-testid={testId}
      aria-labelledby={namedRegion && title != null ? headingId : undefined}
      bare={bare}
      as={as}
      hoverElevated={hoverElevated}
      overflow={stickyFooter ? "visible" : overflow}
      className={cn(!bare && surfacePadding, className)}
    >
      {hasHeader && (
        // `flex-wrap` + `flex-1` am Titelblock: der Einklapp-Pfeil bleibt auf
        // dem Telefon in der Titelzeile, nur die Aktionen brechen als volle
        // Zeile um (dev-Fix "keep SectionCard collapse chevron on the title
        // row on mobile").
        <SectionCollapseHead
          className={cn(
            "flex flex-wrap items-start gap-3",
            zoned && cn(ZONE_PADDING_X, ZONE_PADDING_TOP),
            zoned && !showBody && !showFooter && ZONE_PADDING_BOTTOM,
          )}
        >
          {/* Ohne Kicker und Beschreibung ist der Titel eine Zeile neben einer
              40px-Symbolfläche: oben bündig liegen die beiden Mitten 8px
              auseinander und der Titel wirkt zu hoch. Mit Beschreibung bleibt
              es bei oben bündig, sonst rutschte das Symbol in die Mitte des
              Textblocks. */}
          {/* Mindestbreite 12rem: in einer schmalen Karte (Seitenspalte)
              rutschen die Aktionen unter den Titel, statt Titel und
              Beschreibung auf wenige Wörter je Zeile zusammenzudrücken. Der
              Einklapp-Pfeil passt auch auf 320 px noch daneben. */}
          <div
            className={cn(
              "flex min-w-0 flex-[1_1_12rem] gap-3",
              !description && !kicker && "items-center",
            )}
          >
            {leadingVisual ??
              (Icon && <IconTile icon={<Icon aria-hidden="true" />} />)}
            <div className="min-w-0">
              {kicker && (
                <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
                  {kicker}
                </p>
              )}
              {(title != null || titleBadge != null) && (
                <div
                  className={cn(
                    "flex flex-wrap items-center gap-2",
                    kicker && "mt-1",
                  )}
                >
                  {title != null && (
                    <Heading
                      id={namedRegion ? headingId : undefined}
                      className={cn(
                        "font-semibold text-balance text-gray-900",
                        TITLE_SIZES[titleSize],
                        titleClassName,
                      )}
                    >
                      {title}
                    </Heading>
                  )}
                  {titleBadge}
                </div>
              )}
              {description && (
                <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
                  {description}
                </p>
              )}
            </div>
          </div>
          {headerActions != null && (
            // Inline: der Titel ist auf die 40px-Symbolfläche zentriert, also
            // bekommt die Aktion dieselbe Höhe und liegt auf derselben Mitte.
            // Mit Beschreibung ist die Kopfzeile oben bündig; dann bleibt es
            // bei der Oberkante.
            <div
              className={
                inlineActions
                  ? cn(
                      "flex shrink-0 items-center gap-2",
                      !description && !kicker && "h-10",
                    )
                  : // max-w-full: viele Aktionen brechen in der Karte um,
                    // statt über ihren Rand zu ragen (#4975).
                    "order-last flex w-full max-w-full flex-wrap items-center gap-2 sm:order-none sm:w-auto sm:shrink-0"
              }
            >
              {headerActions}
            </div>
          )}
          {collapsible && (
            <SectionCollapseToggle
              title={title}
              collapsed={collapsed}
              controls={showBody ? bodyId : undefined}
              onToggle={() => {
                const next = !collapsed;
                if (collapsedProp === undefined) setCollapsedState(next);
                onCollapsedChange?.(next);
              }}
            />
          )}
        </SectionCollapseHead>
      )}
      {showBody && (
        <div id={collapsible ? bodyId : undefined} className={bodyClasses}>
          {loading ? (
            // Wie SkeletonRegion, ohne page-skeletons (das zöge die DataTable
            // in jede Seite mit einer Karte) und als div, weil <output> keine
            // Liste aufnehmen darf.
            <div
              role="status"
              aria-label={`${title ?? "Inhalt"} wird geladen`}
              aria-live="polite"
              aria-busy="true"
            >
              <SectionCardLoadingRows rows={loadingRows} />
            </div>
          ) : (
            children
          )}
        </div>
      )}
      {showFooter && (
        <div
          className={cn(
            "border-t border-gray-100 bg-gray-50/70 py-3",
            ZONE_PADDING_X,
            stickyFooter &&
              `${STICKY_FOOTER} z-10 rounded-b-2xl bg-gray-50/95 backdrop-blur-sm`,
          )}
        >
          {footer}
        </div>
      )}
    </SectionCardSurface>
  );
}
