"use client";

// Konto-Seite des Caterer-Portals (#4496): die Anmelde-Adresse und
// „Passwort ändern“. Jeder Bereich ist eine eigene Karte, damit weitere
// Bereiche des Kontos (etwa die Zwei-Faktor-Anmeldung, #4497) als nächste
// Karte dazukommen.

import { useState } from "react";
import { useSession } from "next-auth/react";
import { TenantPasswordChangeModal } from "~/components/auth/tenant-password-change-modal";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { SectionCard } from "~/components/ui/section-card";
import { useShellAuth } from "~/lib/shell-auth-context";

export default function CatererAccountPage() {
  const { data: session } = useSession();
  const { logout } = useShellAuth();
  const [showPasswordModal, setShowPasswordModal] = useState(false);

  return (
    <div className="space-y-6">
      <SectionCard
        title="Anmeldung"
        description="Mit dieser E-Mail-Adresse melden Sie sich an."
      >
        <DataGrid>
          <DataField label="E-Mail" fullWidth>
            {session?.user?.email ?? "–"}
          </DataField>
        </DataGrid>
      </SectionCard>

      <SectionCard
        title="Passwort"
        description="Nach dem Ändern melden Sie sich auf allen Geräten neu an."
        actions={
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => setShowPasswordModal(true)}
          >
            Passwort ändern
          </Button>
        }
      />

      <TenantPasswordChangeModal
        isOpen={showPasswordModal}
        onClose={() => setShowPasswordModal(false)}
        apiEndpoint="/api/caterer/account/password"
        // The change ended every session of the account, this one too. If the
        // logout request itself fails, the revoked session still opens
        // nothing; its next request ends it (#5194).
        onSuccess={() => void logout().catch(() => undefined)}
      />
    </div>
  );
}
