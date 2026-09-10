"use client";

// Der Weg von einem Formular zu den Stammdaten, die ihm fehlen (#3114).
//
// Ein halb ausgefülltes Formular lebt nur im Zustand seiner Seite. Führt der
// Weg zur Verwaltung im selben Fenster, ist der Entwurf weg — genau der Fall,
// für den die Verwaltung früher als Overlay über dem Formular aufging. Der
// Link öffnet deshalb ein zweites Fenster, und die Auswahl im Formular lädt
// ihre Einträge nach, sobald das erste wieder im Vordergrund ist.

import { ExternalLink } from "lucide-react";
import { useEffect } from "react";

import { useTenantAwarePath } from "~/lib/tenant-path";

interface CatalogManageLinkProps {
  /** Pfad ohne Mandantenteil, z. B. „/database/categories". */
  readonly href: string;
  /** „Terminkategorien verwalten" — nennt die Stammdaten, nicht den Weg
   *  dorthin, und benennt sie so, wie die Seitenleiste sie führt. */
  readonly label: string;
}

export function CatalogManageLink({ href, label }: CatalogManageLinkProps) {
  const tenantPath = useTenantAwarePath();

  return (
    // Bewusst `<a>` statt NavigationLink: ein neues Fenster meldet der
    // Portalhülle keinen Seitenwechsel, den sie anzeigen müsste.
    <a
      href={tenantPath(href)}
      target="_blank"
      rel="noopener noreferrer"
      className="inline-flex items-center gap-1 text-xs font-medium text-gray-600 underline underline-offset-2 hover:text-gray-900"
    >
      {label}
      <ExternalLink className="size-3" aria-hidden="true" />
      <span className="sr-only">(öffnet ein neues Fenster)</span>
    </a>
  );
}

/**
 * Lädt die Auswahl nach, sobald das Fenster wieder im Vordergrund ist: der
 * Eintrag, den jemand nebenan angelegt hat, steht dann ohne Zutun in der
 * Liste. SWR-gestützte Auswahlen brauchen den Haken nicht, sie tun das schon.
 */
export function useCatalogRefreshOnFocus(
  refresh: () => void | Promise<void>,
  enabled = true,
) {
  useEffect(() => {
    if (!enabled) return;
    const run = () => void refresh();
    window.addEventListener("focus", run);
    return () => window.removeEventListener("focus", run);
  }, [refresh, enabled]);
}
