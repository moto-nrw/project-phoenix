"use client";

import { RoomDetailSkeleton } from "~/components/rooms/room-detail-content";
import { Skeleton } from "~/components/ui/skeleton";
import { TenantPage } from "~/components/ui/tenant-page";

/**
 * Das geladene Seitengerüst der Raumseite. Titel und Reiter hängen am noch
 * nicht geladenen Raum und an den Rechten; die Platzhalter-Reiter zeigen die
 * vollständige Struktur deaktiviert.
 */
export function RoomDetailLoadingPage({
  referrer = "/rooms",
  backLabel = "Zurück zu den Räumen",
  preview,
}: Readonly<{
  referrer?: string;
  backLabel?: string;
  preview?: { name: string; context?: string };
}>) {
  return (
    <TenantPage
      title={preview?.name ?? "Raum"}
      back
      backHref={referrer}
      backLabel={backLabel}
      leading={<Skeleton className="h-10 w-10 shrink-0 rounded-xl" />}
      stats={preview?.context}
      statsLoading={!preview}
      tabs={{
        value: "uebersicht",
        onChange: () => {},
        items: [
          { value: "uebersicht", label: "Übersicht", disabled: true },
          { value: "stammdaten", label: "Stammdaten", disabled: true },
        ],
      }}
    >
      <RoomDetailSkeleton />
    </TenantPage>
  );
}
