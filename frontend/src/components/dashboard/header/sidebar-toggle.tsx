"use client";

import { useEffect } from "react";
import { PanelLeftClose, PanelLeftOpen } from "lucide-react";
import { Button } from "~/components/ui/button";
import { useSidebarCollapsed } from "~/lib/hooks/use-sidebar-collapsed";

// Nur oberhalb von lg existiert die Desktop-Seitenleiste; darunter zeigt
// die App die mobile Bottom-Nav und der Shortcut hätte nichts zu schalten.
const SIDEBAR_VISIBLE_QUERY = "(min-width: 1024px)";

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return (
    target.isContentEditable ||
    target.tagName === "INPUT" ||
    target.tagName === "TEXTAREA" ||
    target.tagName === "SELECT"
  );
}

/**
 * Ein-/Ausklappen der Desktop-Seitenleiste (#2825). Erstes Element der
 * Kopfzeile, links vom Logo — die Standardposition für Layouts mit
 * vollbreiter Topbar (Gmail, GitHub). Der Zustand syncht über den
 * geteilten useSidebarCollapsed-Store mit der Seitenleiste selbst.
 * Unterhalb lg gibt es keine Seitenleiste (mobile Bottom-Nav), also auch
 * keinen Schalter.
 *
 * Zusätzlich schaltet Cmd+B (Mac) bzw. Strg+B (Windows/Linux) um — der
 * etablierte Standard (VS Code, shadcn). In Eingabefeldern und Editoren
 * bleibt die Tastenkombination unangetastet (dort heißt sie „fett").
 */
export function SidebarToggle() {
  const { collapsed, toggleCollapsed } = useSidebarCollapsed();

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() !== "b") return;
      if (!(event.metaKey || event.ctrlKey)) return;
      if (event.altKey || event.shiftKey) return;
      if (isEditableTarget(event.target)) return;
      if (!globalThis.matchMedia(SIDEBAR_VISIBLE_QUERY).matches) return;
      event.preventDefault();
      toggleCollapsed();
    };
    globalThis.addEventListener("keydown", handleKeyDown);
    return () => globalThis.removeEventListener("keydown", handleKeyDown);
  }, [toggleCollapsed]);

  const label = collapsed
    ? "Seitenleiste ausklappen"
    : "Seitenleiste einklappen";
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      onClick={toggleCollapsed}
      title={label}
      aria-label={label}
      aria-expanded={!collapsed}
      aria-keyshortcuts="Control+B Meta+B"
      // -ml-4 zieht den Button aus dem Header-Padding (lg:px-8) nach links,
      // sodass die Icon-Mitte (48px Button-Mitte - 16px = 32px) exakt über
      // der Icon-Spalte der eingeklappten Seitenleiste (w-16, Mitte 32px)
      // sitzt — der Toggle liest sich als Kopf der Leiste, nicht als
      // freischwebendes Header-Element.
      className="-ml-4 hidden shrink-0 lg:inline-flex"
    >
      {collapsed ? (
        <PanelLeftOpen size={18} aria-hidden="true" />
      ) : (
        <PanelLeftClose size={18} aria-hidden="true" />
      )}
    </Button>
  );
}
