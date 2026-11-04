"use client";

import { useSession } from "next-auth/react";
import { Alert } from "~/components/ui/alert";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Skeleton } from "~/components/ui/skeleton";
import { hasPermission } from "~/lib/auth-utils";
import { TraegerContactCard } from "./traeger-contact-card";
import { useTraegerMasterData } from "./use-traeger-master-data";

/** The Träger right that changes the master data (ADR 0084 section 4). */
const SETTINGS_RIGHT = "traeger_settings:manage";

/**
 * Die Ansprechperson des Trägers im Bereich „Kontakt“ unter „Allgemein“ (#5685).
 * Liest die Stammdaten erst beim Öffnen des Bereichs: Jedes Lesen steht im
 * Protokoll des Trägers.
 */
export function TraegerContactSection() {
  const { data: session } = useSession();
  const canEdit = hasPermission(session ?? null, SETTINGS_RIGHT);
  const { masterData, loaded, loading, error, refresh } =
    useTraegerMasterData("die Kontaktdaten");

  if (error) return <LoadErrorAlert error={error} />;
  if (loading && !loaded)
    return <Skeleton className="h-48 w-full rounded-2xl" />;
  if (!loaded) return null;
  return (
    <>
      {canEdit ? null : (
        <Alert
          type="info"
          message="Sie können die Angaben nur ansehen. Ändern kann, wer im Träger-Büro das Recht „Einstellungen“ hat."
        />
      )}
      <TraegerContactCard
        contact={masterData.contact}
        canEdit={canEdit}
        onSaved={refresh}
      />
    </>
  );
}
