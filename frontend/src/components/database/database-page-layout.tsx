"use client";

import type { ReactNode } from "react";
import { MasterDetailSkeleton } from "./master-detail-skeleton";
import { MobileBackButton } from "~/components/ui/mobile-back-button";
import { PageIntro } from "~/components/ui/page-intro";

/** Kopfkarte der Seite (PageIntro). Titel und Kicker sind statisch, deshalb
 *  rendert die Karte auch im Ladezustand sofort. */
interface DatabasePageIntro {
  /** Name der Sidebar-Gruppe, in der Regel "Datenverwaltung". */
  kicker?: string;
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
  intro?: DatabasePageIntro;
  /** Such- und Filterzeile der Liste. Sie steht im unteren Teil der Kopfkarte,
   *  damit Kopfkarte und Suchzeile nicht als zwei fast leere Zeilen
   *  übereinander liegen. */
  search?: ReactNode;
  /** Page content to render when not loading */
  children: ReactNode;
  /** Optional className for the content wrapper */
  className?: string;
}

/**
 * Shared layout wrapper for database management pages.
 * Handles loading states, responsive layout, and mobile back button.
 *
 * Extracted to eliminate code duplication across database pages.
 */
export function DatabasePageLayout({
  loading,
  sessionLoading,
  intro,
  search,
  children,
  className = "w-full",
}: Readonly<DatabasePageLayoutProps>) {
  const isLoading = sessionLoading || loading;

  if (isLoading && !intro) {
    return <MasterDetailSkeleton />;
  }

  return (
    <div className={className}>
      <MobileBackButton />
      {intro && (
        <PageIntro
          kicker={intro.kicker}
          title={intro.title}
          description={intro.description}
          actions={intro.actions}
          className="mb-6"
        >
          {!isLoading && search ? search : null}
        </PageIntro>
      )}
      {isLoading ? <MasterDetailSkeleton intro={false} /> : children}
    </div>
  );
}
