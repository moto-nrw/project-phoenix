"use client";

import { TenantPage } from "~/components/ui/tenant-page";
import { StaffCardsSkeleton } from "./page-skeleton";

export default function StaffLoading() {
  return (
    // Titel und Suchfeld sind fest und rendern sofort im echten Gerüst; nur
    // die Statuszeile und die Kartenliste darunter skelettieren.
    <TenantPage
      title="Mitarbeiter"
      statsLoading
      search={{
        value: "",
        onChange: () => {},
        placeholder: "Mitarbeiter suchen…",
        inputProps: { disabled: true },
      }}
    >
      <StaffCardsSkeleton />
    </TenantPage>
  );
}
