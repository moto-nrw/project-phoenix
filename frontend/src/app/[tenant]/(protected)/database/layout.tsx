"use client";

import { usePathname } from "next/navigation";
import { RoleGuard } from "~/components/auth/role-guard";
import { Loading } from "~/components/ui/loading";

export default function DatabaseLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const pathname = usePathname();
  // Wer Mitarbeiter-Datensätze pflegen darf (staff:manage, #2906), erreicht
  // dafür die Personalseite — auch ohne Leitungsrolle. Die übrige
  // Datenverwaltung bleibt der OGS-Leitung vorbehalten.
  const isPersonnelPage = pathname?.endsWith("/database/personal") ?? false;

  return (
    <RoleGuard
      variant="adminOnly"
      permission={isPersonnelPage ? "staff:manage" : undefined}
      message="Du verfügst nicht über die notwendigen Berechtigungen, um die Datenverwaltung aufzurufen."
      fallback={
        <Loading message="Berechtigungen werden geprüft…" fullPage={false} />
      }
    >
      {children}
    </RoleGuard>
  );
}
