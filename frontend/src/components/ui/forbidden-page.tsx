"use client";

import { LockKeyIcon } from "@phosphor-icons/react";
import { EmptyState } from "~/components/ui/empty-state";
import { TenantPage } from "~/components/ui/tenant-page";

interface ForbiddenPageProps {
  readonly title?: string;
  readonly message?: string;
  /**
   * Die Seite bringt ihren eigenen Kopf schon mit und zeigt die Sperre nur für
   * einen Bereich. Ohne das Kennzeichen rendert die Sperre die ganze Seite,
   * inklusive Kopf — sonst begann eine gesperrte Seite als einziger Ort der
   * App ohne Titel und ohne Ortsangabe.
   */
  readonly embedded?: boolean;
}

/**
 * Kein Zugriff ist ein Zustand, kein Fehler: die Person hat nichts falsch
 * gemacht, ihr fehlt ein Recht. Deshalb der ruhige Leerzustand des Kits und
 * kein roter Alarmkasten — und ein Satz dazu, wer das ändern kann.
 */
/** Was fehlt, in einer Zeile. Der Satz darunter sagt, wer es ändern kann. */
const HEADLINE = "Ihnen fehlt eine Berechtigung";

export function ForbiddenPage({
  title = "Kein Zugriff",
  message = "Bitte wenden Sie sich an Ihre Leitung.",
  embedded = false,
}: ForbiddenPageProps) {
  const icon = <LockKeyIcon className="h-12 w-12" aria-hidden="true" />;

  if (embedded) {
    return <EmptyState icon={icon} title={HEADLINE} description={message} />;
  }

  // Als ganze Seite trägt die Kopfkarte den Titel; der Rumpf wiederholt ihn
  // nicht, sondern erklärt nur. Die Fläche liefert der Leerzustand des
  // Gerüsts, damit die gesperrte Seite genauso aussieht wie eine leere.
  return (
    <TenantPage
      title={title}
      empty={{ title: HEADLINE, description: message, icon }}
    />
  );
}
