"use client";

import type { ReactNode } from "react";
import { MasterDetailSkeleton } from "./master-detail-skeleton";
import { MobileBackButton } from "~/components/ui/mobile-back-button";
import { TenantPage } from "~/components/ui/tenant-page";

/** Kopfkarte der Seite. Der Titel ist statisch, deshalb rendert die Karte auch
 *  im Ladezustand sofort. */
interface DatabasePageHead {
  title: string;
  description?: ReactNode;
  /** Seitenaktionen, zum Beispiel DatabaseCreateAction oder OverflowMenu. */
  actions?: ReactNode;
}

interface DatabasePageLayoutProps {
  /** Whether the page is in a loading state */
  loading: boolean;
  /** Whether the auth session is loading */
  sessionLoading: boolean;
  /** Kopfkarte der Seite; entfällt nur bei Seiten ohne eigenen Kopf. */
  intro?: DatabasePageHead;
  /** Such- und Filterzeile der Liste. Sie steht im unteren Teil der Kopfkarte,
   *  damit Kopfkarte und Suchzeile nicht als zwei fast leere Zeilen
   *  übereinander liegen. */
  search?: ReactNode;
  /** Page content to render when not loading */
  children: ReactNode;
  /** Optional className for the content wrapper */
  className?: string;
  /**
   * Ladefehler der Seite. Er ersetzt den Inhalt im Gerüst; ein Fehler ist
   * niemals ein Leerzustand (BAUARTEN-SPEC, Querregel „Zustände").
   */
  error?: string | null;
  /**
   * Dialoge der Seite (Anlegen, Löschen, Einladen). Sie stehen NEBEN dem
   * Inhalt, nicht darin: der Leer- und der Fehlerzustand ersetzen den Inhalt,
   * dürfen aber kein offenes Modal aushängen.
   */
  overlays?: ReactNode;
  /** Leerzustand der Seite, wenn es nichts zu zeigen gibt. */
  empty?: {
    title: string;
    description?: string;
    icon?: ReactNode;
    action?: ReactNode;
  } | null;
}

/**
 * Adapter der Datenverwaltungs-Seiten auf das gemeinsame Seitengerüst
 * (`ui/TenantPage`). Er hält nur noch die Besonderheit dieser Seiten fest:
 * das Master-Detail-Skelett als Ladezustand. Kopfkarte, Abstände, Zurück-Knopf
 * und Reihenfolge kommen aus dem Gerüst.
 */
export function DatabasePageLayout({
  loading,
  sessionLoading,
  intro,
  search,
  children,
  className,
  error,
  empty,
  overlays,
}: Readonly<DatabasePageLayoutProps>) {
  const isLoading = sessionLoading || loading;

  if (isLoading && !intro) {
    return <MasterDetailSkeleton />;
  }

  // Ohne Kopfkarte gibt es kein Gerüst, das den Zurück-Knopf trägt — die
  // Seite braucht ihn auf dem Telefon trotzdem.
  if (!intro) {
    return (
      <div className={className ?? "w-full"}>
        <MobileBackButton />
        {children}
        {overlays}
      </div>
    );
  }

  return (
    <>
      <TenantPage
        title={intro.title}
        stats={intro.description}
        actions={intro.actions}
        searchSlot={!isLoading && search ? search : undefined}
        error={error ?? null}
        empty={!isLoading && !error ? (empty ?? null) : null}
        back
      >
        {isLoading ? <MasterDetailSkeleton intro={false} /> : children}
      </TenantPage>
      {overlays}
    </>
  );
}
