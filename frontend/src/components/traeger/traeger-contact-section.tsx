"use client";

import { useSession } from "next-auth/react";
import { Alert } from "~/components/ui/alert";
import { hasPermission } from "~/lib/auth-utils";
import type { TraegerMasterData } from "~/lib/traeger-api";
import { TraegerContactCard } from "./traeger-contact-card";
import { TraegerWithdrawalContactCard } from "./traeger-withdrawal-contact-card";

/** The Träger right that changes the master data (ADR 0084 section 4). */
const SETTINGS_RIGHT = "traeger_settings:manage";

/**
 * Die Ansprechperson des Trägers und sein Kontakt für Widerrufe im Bereich
 * „Kontakt“ unter „Allgemein“ (#5685, #6200). Liest die Stammdaten erst beim
 * Öffnen des Bereichs: Jedes Lesen steht im Protokoll des Trägers.
 */
export function TraegerContactSection({
  masterData,
  letterheadText,
  onSaved,
}: Readonly<{
  masterData: Pick<TraegerMasterData, "contact" | "withdrawal">;
  /** Der Absender aus dem Briefkopf; die Belehrung nennt ihn mit Anschrift. */
  letterheadText: string;
  onSaved: () => Promise<unknown>;
}>) {
  const { data: session } = useSession();
  const canEdit = hasPermission(session ?? null, SETTINGS_RIGHT);
  return (
    <>
      {canEdit ? null : (
        <Alert
          type="info"
          message="Sie können die Angaben nur ansehen. Ändern kann, wer im Träger-Büro die Rolle „Verwaltung“ hat."
        />
      )}
      <TraegerContactCard
        contact={masterData.contact}
        canEdit={canEdit}
        onSaved={onSaved}
      />
      <TraegerWithdrawalContactCard
        withdrawal={masterData.withdrawal}
        contact={masterData.contact}
        letterheadText={letterheadText}
        canEdit={canEdit}
        onSaved={onSaved}
      />
    </>
  );
}
