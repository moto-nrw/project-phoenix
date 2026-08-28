"use client";

import { LockKeyIcon } from "@phosphor-icons/react";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
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
export function ForbiddenPage({
  title = "Kein Zugriff",
  message = "Sie haben nicht die nötige Berechtigung für diese Seite. Ihre Leitung kann sie in den Einstellungen freischalten.",
  embedded = false,
}: ForbiddenPageProps) {
  const icon = <LockKeyIcon className="h-12 w-12" aria-hidden="true" />;

  if (embedded) {
    return <EmptyState icon={icon} title={title} description={message} />;
  }

  // Als ganze Seite trägt die Kopfkarte den Titel; der Rumpf wiederholt ihn
  // nicht, sondern erklärt nur.
  return (
    <TenantPage title={title}>
      <SectionCard>
        <div className="flex flex-col items-center gap-3 py-12 text-center">
          <span className="text-gray-400">{icon}</span>
          <p className="max-w-md text-sm leading-6 text-gray-500">{message}</p>
        </div>
      </SectionCard>
    </TenantPage>
  );
}
