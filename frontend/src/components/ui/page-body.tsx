"use client";

import type { ReactNode } from "react";
import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import type { FormErrorDetail } from "~/components/ui/form-error";
import { errorAlertActions } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { useNotBookedSlots } from "./not-booked-slot";

/** Fehlerzustand eines Seitengerüsts; siehe `TenantPage.error`. */
export type PageBodyError =
  | string
  | { message: string; action?: ReactNode; keepContent?: boolean }
  /** Ein Ladefehler aus `useApiLoadError` (#2513): Wiederholen und
   *  Vorgangskennung kommen als Aktion mit. */
  | FormErrorDetail
  | null;

/** Leerzustand eines Seitengerüsts; siehe `TenantPage.empty`. */
export type PageBodyEmpty = {
  readonly title: string;
  readonly description?: string;
  readonly icon?: ReactNode;
  readonly action?: ReactNode;
} | null;

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
 * auftauchen. `TenantPage` und `ParentPage` rendern ihren Inhalt hierüber
 * (ADR 0149, Entscheidung 1).
 */
export function PageBody({
  loading,
  loadingLabel,
  error,
  empty,
  children,
}: Readonly<{
  loading: boolean | ReactNode;
  /** Vorlesbarer Text des Ladezustands, aus dem Seitentitel gebildet. */
  loadingLabel: string;
  error?: PageBodyError;
  empty?: PageBodyEmpty;
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
              <Skeleton shape="card" className="h-24 w-full" />
              <Skeleton shape="card" className="h-24 w-full" />
              <Skeleton shape="card" className="h-24 w-full" />
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
