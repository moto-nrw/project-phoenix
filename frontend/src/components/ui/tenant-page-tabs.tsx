"use client";

import Link from "~/components/ui/navigation-link";
import { StatusBadge } from "~/components/ui/status-badge";
import { ChevronDown } from "lucide-react";
import type { KeyboardEvent, MouseEvent } from "react";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { TenantPageProps, TenantPageTab } from "./tenant-page-types";
import { BELOW_SM, useMediaQuery } from "~/lib/hooks/use-media-query";
import { cn } from "~/lib/utils";

/**
 * Die EINE Bauart für Seitenreiter im Tenant-Portal. Bewusst einfache
 * Schaltflächen statt Radix: Radix-Reiter schalten auf `mousedown`, was die
 * bestehenden `fireEvent.click`-Tests still ins Leere laufen lässt.
 * `ui/Tabs` bleibt für Reiter INNERHALB einer Karte, `SegmentedControl` für
 * eine Wertauswahl.
 */
/** Abstand zwischen zwei Reitern (gap-6) -- die Messung braucht ihn als Zahl. */
const TAB_GAP = 24;

/** Auffangreiter, wenn die Breite nicht fuer alle Reiter reicht. */
const MORE_LABEL = "Mehr";

export function TenantPageTabs({
  value,
  onChange,
  items,
  label = "Seitenbereiche",
}: NonNullable<TenantPageProps["tabs"]>) {
  // Reiter werden NICHT von Hand gebuendelt. Ein benannter Sammelreiter
  // („Verwaltung") ist geraten: er verraet nicht, was in ihm liegt, und er
  // buendelt auch dann, wenn der Platz laengst reicht. Stattdessen wird
  // gemessen -- sichtbar ist, was hineinpasst, der Rest steht unter „Mehr".
  const rowRef = useRef<HTMLDivElement>(null);
  const tablistRef = useRef<HTMLDivElement>(null);
  const measureRef = useRef<HTMLDivElement>(null);
  // Vor der ersten Client-Auflösung gilt die Desktop-Variante. Das entspricht
  // dem Server-Render und verhindert eine Hydrations-Abweichung.
  const isPhone = useMediaQuery(BELOW_SM);
  const [visibleCount, setVisibleCount] = useState(items.length);
  // Liegen im scrollenden Band (unter sm) rechts noch Reiter? Dann zeigt ein
  // Verlaufs-Fade an der Kartenkante, dass man scrollen kann — eine hart
  // angeschnittene Pille liest sich sonst als Darstellungsfehler, und ein
  // ganz hinausgescrollter Reiter wäre unsichtbar, ohne dass etwas auf ihn
  // zeigt.
  const [canScrollRight, setCanScrollRight] = useState(false);

  const updateScrollHint = useCallback(() => {
    const tablist = tablistRef.current;
    if (!tablist) return;
    setCanScrollRight(
      tablist.scrollWidth - tablist.clientWidth - tablist.scrollLeft > 1,
    );
  }, []);

  const measure = useCallback(() => {
    const row = rowRef.current;
    const shadow = measureRef.current;
    if (!row || !shadow) return;

    const children = Array.from(shadow.children) as HTMLElement[];
    if (children.length < 2) return;
    // Das letzte Kind der Schattenzeile ist der „Mehr"-Auslöser.
    const moreWidth = children[children.length - 1]!.offsetWidth;
    const widths = children.slice(0, -1).map((child) => child.offsetWidth);
    const available = row.clientWidth;

    // Ohne Messwerte (Testumgebung, noch nicht gelayoutet, Zeile verborgen)
    // wird NICHT geraten: dann stehen alle Reiter da. Ein „Mehr" zu bauen,
    // weil die Breite unbekannt ist, versteckt Bereiche ohne Grund.
    if (available === 0 || widths.every((width) => width === 0)) {
      setVisibleCount(items.length);
      return;
    }

    let used = 0;
    let count = 0;
    for (const width of widths) {
      const next = used + (count > 0 ? TAB_GAP : 0) + width;
      if (next > available) break;
      used = next;
      count += 1;
    }

    // Passt nicht alles, braucht auch „Mehr" seinen Platz.
    if (count < widths.length) {
      while (count > 0 && used + TAB_GAP + moreWidth > available) {
        used -= widths[count - 1]! + (count > 1 ? TAB_GAP : 0);
        count -= 1;
      }
    }

    setVisibleCount(Math.max(count, 1));
  }, [items.length]);

  useLayoutEffect(() => {
    measure();
    updateScrollHint();
    const row = rowRef.current;
    const tablist = tablistRef.current;
    if (!row || !tablist) return;
    tablist.addEventListener("scroll", updateScrollHint, { passive: true });
    let observer: ResizeObserver | undefined;
    if (typeof ResizeObserver !== "undefined") {
      observer = new ResizeObserver(() => {
        measure();
        updateScrollHint();
      });
      observer.observe(row);
      observer.observe(tablist);
    }
    return () => {
      tablist.removeEventListener("scroll", updateScrollHint);
      observer?.disconnect();
    };
  }, [measure, updateScrollHint, items]);

  // Auf dem Telefon rueckt der aktive Reiter ins Sichtfenster des Bandes,
  // sonst steht die Seite auf einem Bereich, den man nicht sieht.
  useEffect(() => {
    const active = tablistRef.current?.querySelector<HTMLElement>(
      '[role="tab"][aria-selected="true"]',
    );
    if (typeof active?.scrollIntoView !== "function") return;
    active.scrollIntoView({ inline: "nearest", block: "nearest" });
  }, [value]);

  const visible = items.slice(0, visibleCount);
  const hidden = items.slice(visibleCount);
  const hiddenActive = hidden.find((item) => item.value === value);
  const activeItem = items.find((item) => item.value === value);
  const visibleRovingItem = visible.find((item) => !item.disabled);
  const fallbackRovingValue = isPhone
    ? items.find((item) => !item.disabled)?.value
    : (visibleRovingItem?.value ??
      (hidden.some((item) => !item.disabled) ? "__mehr__" : undefined));
  // Auf dem Desktop vertritt „Mehr“ den aktiven verborgenen Reiter. Auf dem
  // Telefon sind alle Reiter sichtbar, der aktive Reiter bleibt daher selbst
  // der eine Tabulator-Stopp. Hat kein Reiter die aktuelle Filterlage,
  // bleibt die Auswahl leer; ein sichtbarer Reiter ist trotzdem per Tab
  // erreichbar und eröffnet die Pfeiltasten-Navigation.
  const rovingValue =
    hiddenActive && !isPhone && !hiddenActive.disabled
      ? "__mehr__"
      : activeItem && !activeItem.disabled
        ? value
        : fallbackRovingValue;

  const tabClass = (active: boolean, disabled?: boolean) =>
    cn(
      "flex shrink-0 items-center gap-1.5 border-b-[3px] pb-3 text-base whitespace-nowrap transition-colors",
      // Auf kleinen und flachen Bildschirmen eine Stufe enger (#2824): das
      // Reiterband liegt zwischen Titel und Suchzeile und kostet dort Hoehe,
      // die dem Inhalt fehlt.
      "compact:pb-2 compact:text-sm",
      // Auf dem Telefon Pillen statt Grundlinie: eine graue Schrift mit
      // 3-px-Strich liest sich in einer 390-px-Spalte als Text, nicht als
      // Navigation. Die gefüllte Pille ist erkennbar ein Bedienelement.
      "max-sm:rounded-full max-sm:border-0 max-sm:px-3 max-sm:py-1.5 max-sm:pb-1.5 max-sm:text-sm",
      active
        ? "border-moto-green font-semibold text-gray-900 max-sm:bg-gray-900 max-sm:text-white"
        : "border-transparent font-medium text-gray-500 hover:border-gray-300 hover:text-gray-900 max-sm:bg-gray-100 max-sm:text-gray-700",
      disabled && "cursor-not-allowed opacity-50",
    );

  const renderInner = (item: TenantPageTab) => (
    <>
      {item.label}
      {item.badge !== undefined && item.badge > 0 && (
        <StatusBadge tone="green" size="xs" label={String(item.badge)} />
      )}
    </>
  );

  const renderTab = (
    item: TenantPageTab,
    measuring = false,
    extraClassName?: string,
  ) => {
    const active = item.value === value;
    const className = cn(tabClass(active, item.disabled), extraClassName);
    if (item.href && !item.disabled) {
      return (
        <Link
          key={item.value}
          href={item.href}
          role={measuring ? undefined : "tab"}
          aria-selected={measuring ? undefined : active}
          tabIndex={measuring ? -1 : item.value === rovingValue ? 0 : -1}
          data-tab-value={item.value}
          className={className}
          onClick={(event: MouseEvent<HTMLAnchorElement>) => {
            // Mittelklick und Klick mit Zusatztaste öffnen ein zweites
            // Dokument. Der schlichte Linksklick meldet den Reiterwechsel
            // zusätzlich, lässt die Link-Navigation aber unangetastet.
            if (
              event.defaultPrevented ||
              event.button !== 0 ||
              event.metaKey ||
              event.ctrlKey ||
              event.shiftKey ||
              event.altKey
            ) {
              return;
            }
            onChange(item.value);
          }}
        >
          {renderInner(item)}
        </Link>
      );
    }
    return (
      <button
        key={item.value}
        type="button"
        role={measuring ? undefined : "tab"}
        aria-selected={measuring ? undefined : active}
        tabIndex={measuring ? -1 : item.value === rovingValue ? 0 : -1}
        data-tab-value={item.value}
        disabled={item.disabled}
        onClick={() => onChange(item.value)}
        className={className}
      >
        {renderInner(item)}
      </button>
    );
  };

  const moreTrigger = (measuring = false) => (
    <OverflowMenu
      key="__mehr__"
      ariaLabel={MORE_LABEL}
      triggerRole="tab"
      triggerAriaSelected={Boolean(hiddenActive)}
      triggerTabIndex={measuring ? -1 : rovingValue === "__mehr__" ? 0 : -1}
      triggerClassName={cn(tabClass(Boolean(hiddenActive)), "max-sm:hidden")}
      // Die feste Zeilenhöhe verhindert, dass das Pfeil-Symbol den Reiter um
      // einen Pixel nach unten drückt. In der kompakten Stufe entspricht sie
      // der Zeilenhöhe der text-sm-Reiter.
      triggerContent={
        <span className="compact:leading-5 flex items-center gap-1 leading-6">
          {hiddenActive ? hiddenActive.label : MORE_LABEL}
          <ChevronDown className="size-3.5" aria-hidden />
        </span>
      }
      // Der Auslöser gehört zur Roving-Tabulator-Reihenfolge des Bandes.
      // `onKeyDown` sitzt am tablist-Container, damit alle Reiter dieselben
      // Pfeiltasten verwenden und der portalisierte Menüinhalt davon getrennt
      // bleibt.
      items={(measuring ? items : hidden).map((item) => ({
        label: item.label,
        href: item.href,
        disabled: item.disabled,
        selected: item.value === value,
        onClick: () => onChange(item.value),
      }))}
    />
  );

  const handleTabKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (
      event.key !== "ArrowLeft" &&
      event.key !== "ArrowRight" &&
      event.key !== "Home" &&
      event.key !== "End"
    ) {
      return;
    }

    const current = (event.target as HTMLElement).closest<HTMLElement>(
      '[role="tab"]',
    );
    if (!current || !tablistRef.current?.contains(current)) return;

    const tabs = Array.from(
      tablistRef.current.querySelectorAll<HTMLElement>('[role="tab"]'),
    ).filter((tab) => {
      if (tab.hasAttribute("disabled")) return false;
      // Desktop: die via `sm:hidden` gerenderten Überlauf-Reiter sind nicht
      // sichtbar. Telefon: „Mehr“ ist via `max-sm:hidden` ausgeblendet.
      return isPhone
        ? !tab.classList.contains("max-sm:hidden")
        : !tab.classList.contains("sm:hidden");
    });
    const index = tabs.indexOf(current);
    if (index === -1 || tabs.length === 0) return;

    event.preventDefault();
    const nextIndex =
      event.key === "Home"
        ? 0
        : event.key === "End"
          ? tabs.length - 1
          : (index + (event.key === "ArrowRight" ? 1 : -1) + tabs.length) %
            tabs.length;
    const next = tabs[nextIndex]!;
    next.focus();

    // Der Überlauf öffnet erst auf ausdrückliche Aktivierung. Jeder konkrete
    // Reiter wechselt beim Pfeil wie ein automatisch aktiviertes Tab-Panel.
    const nextValue = next.dataset.tabValue;
    if (nextValue) onChange(nextValue);
  };

  return (
    // Die Randaufhebung gehoert dem Reiterband: seine Grundlinie laeuft ueber
    // die volle Kartenbreite, auf jedem Geraet.
    <div className="compact:-mx-4 compact:mt-3 -mx-5 mt-4 max-sm:-mx-4 max-sm:mt-3">
      {/* EIN Band auf allen Breiten. Unter sm scrollt es waagerecht und zeigt
          jeden Reiter; ab sm wird gemessen und der Ueberhang steht unter
          „Mehr". Vorher stand unter sm eine Auswahlliste -- sie zeigte nur den
          aktiven Wert („Status", „Betrieb") und las sich als Filter, nicht als
          Seitenbereich. Ein sichtbares Band mit derselben Grundlinie wie auf
          dem Desktop sagt auf den ersten Blick: hier wechselt man den
          Bereich, und so viele gibt es.

          Die Grundlinie gehört dem BAND, nicht dem einzelnen Reiter: eine
          Haarlinie über die volle Kartenbreite, der aktive Reiter färbt nur
          sein Stück davon ein. Dadurch sind alle Reiter gleich hohe Kästen
          und der Abstand hängt nicht mehr an einem Strich, den nur einer von
          ihnen trägt. Die Linie verbindet den Reiter zugleich sichtbar mit
          dem Inhalt darunter; eine einzeln getönte Pille sagt das nicht, sie
          liest sich als Filter. */}
      <div className="compact:px-4 relative border-b border-gray-200 px-5 max-sm:border-0 max-sm:px-0">
        <div ref={rowRef} className="flex items-end gap-6 max-sm:gap-0">
          <div
            ref={tablistRef}
            role="tablist"
            aria-label={label}
            tabIndex={-1}
            onKeyDown={handleTabKeyDown}
            // Unter sm scrollt das Band bis an den Kartenrand; der Innenrand
            // der Karte sitzt als Polster im Scrollbereich, damit der erste
            // und der letzte Reiter nicht am Rahmen kleben.
            // Ab sm schneidet die Zeile ab (`clip`, kein Scrollkasten): die
            // Messung sorgt dafür, dass im Ruhezustand alles passt, aber
            // während die Seitenleiste auf- oder zuklappt, kann ein Bild lang
            // ein Reiter über den Kartenrand ragen, bis die Zeile neu gemessen
            // ist. Ohne Clip stand er in einem Screenshot halb außerhalb der
            // Karte.
            className="flex min-w-0 flex-1 items-end gap-6 max-sm:[scrollbar-width:none] max-sm:gap-2 max-sm:overflow-x-auto max-sm:px-4 sm:overflow-x-clip max-sm:[&::-webkit-scrollbar]:hidden"
          >
            {visible.map((item) => renderTab(item))}
            {/* Auf dem Telefon bleiben auch die gemessen verborgenen Reiter im
                Band stehen; der Sammelreiter verschwindet dort. */}
            {hidden.map((item) => renderTab(item, false, "sm:hidden"))}
            {/* Der Sammelreiter gehört semantisch zum Reiterband. Außerhalb
                davon würde er sich für Screenreader zwar als Reiter, aber
                nicht als Teil dieser Seitennavigation ankündigen. */}
            {hidden.length > 0 && moreTrigger()}
          </div>
        </div>
        {/* Scroll-Hinweis des mobilen Bandes: solange rechts Reiter liegen,
            blendet die Kante sie sichtbar aus — die Pille wirkt angeschnitten
            statt abgeschnitten, und beim Zu-Ende-Scrollen verschwindet der
            Verlauf. Ohne ihn wäre ein hinausgescrollter Reiter unsichtbar. */}
        {canScrollRight && (
          <div
            aria-hidden
            className="pointer-events-none absolute inset-y-0 right-0 w-10 bg-gradient-to-l from-white to-transparent sm:hidden"
          />
        )}

        {/* Schattenzeile für die Messung: sie steht in einem Kasten ohne Höhe
            und ist unsichtbar, behält aber die natürlichen Breiten aller
            Reiter. Ohne sie liesse sich nicht feststellen, wie viele Reiter
            in die Zeile passen -- nur, wie breit die bereits gekürzte Zeile
            ist. */}
        <div className="relative h-0 overflow-hidden" aria-hidden>
          <div
            ref={measureRef}
            className="pointer-events-none invisible absolute top-0 left-0 flex items-end gap-6"
          >
            {items.map((item) => renderTab(item, true))}
            {moreTrigger(true)}
          </div>
        </div>
      </div>
    </div>
  );
}
