"use client";

import { TenantPage } from "~/components/ui/tenant-page";
import { RoomsGridSkeleton } from "./page-skeleton";

export default function RoomsLoading() {
  return (
    // Titel und Suchfeld sind fest und rendern sofort im echten Gerüst; nur
    // die Statuszeile und das Raster darunter skelettieren.
    <TenantPage
      title="Räume"
      statsLoading
      search={{ value: "", onChange: () => {}, placeholder: "Raum suchen…" }}
    >
      <RoomsGridSkeleton />
    </TenantPage>
  );
}
