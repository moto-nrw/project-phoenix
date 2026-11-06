"use client";

import { useSession } from "next-auth/react";
import { Alert } from "~/components/ui/alert";
import { hasPermission } from "~/lib/auth-utils";
import type { TraegerContact } from "~/lib/traeger-api";
import { TraegerContactCard } from "./traeger-contact-card";

/** The Träger right that changes the master data (ADR 0084 section 4). */
const SETTINGS_RIGHT = "traeger_settings:manage";

/**
 * Die Ansprechperson des Trägers im Bereich „Kontakt“ unter „Allgemein“ (#5685).
 * Liest die Stammdaten erst beim Öffnen des Bereichs: Jedes Lesen steht im
 * Protokoll des Trägers.
 */
export function TraegerContactSection({
  contact,
  onSaved,
}: Readonly<{
  contact: TraegerContact;
  onSaved: () => Promise<unknown>;
}>) {
  const { data: session } = useSession();
  const canEdit = hasPermission(session ?? null, SETTINGS_RIGHT);
  return (
    <>
      {canEdit ? null : (
        <Alert
          type="info"
          message="Sie können die Angaben nur ansehen. Ändern kann, wer im Träger-Büro das Recht „Einstellungen“ hat."
        />
      )}
      <TraegerContactCard
        contact={contact}
        canEdit={canEdit}
        onSaved={onSaved}
      />
    </>
  );
}
