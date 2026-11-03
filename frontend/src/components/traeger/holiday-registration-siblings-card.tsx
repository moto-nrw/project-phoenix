"use client";

// Geschwister einer Ferienanmeldung (#5628): die anderen Anmeldungen, mit
// denen das Träger-Büro diese Anmeldung zusammengeführt hat, und „Trennen“.
// Zusammenführen geht in der Liste der Anmeldungen. Die Seite lädt diese
// Karte nur, wenn die Anmeldung Geschwister hat.

import { useState } from "react";
import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import NavigationLink from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import type { HolidaySibling } from "~/lib/holiday-contribution";
import { createLogger } from "~/lib/logger";
import { splitHolidaySibling } from "~/lib/traeger-sibling-groups-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "HolidayRegistrationSiblingsCard" });

export function HolidayRegistrationSiblingsCard({
  periodId,
  registrationId,
  siblings,
  invoiced,
  onSplit,
}: Readonly<{
  periodId: string;
  registrationId: string;
  siblings: readonly HolidaySibling[];
  /** An issued invoice bills this registration; the group stays. */
  invoiced: boolean;
  /** After the split; the page reads the registration again. */
  onSplit: () => Promise<void>;
}>) {
  const [confirming, setConfirming] = useState(false);
  const [splitting, setSplitting] = useState(false);
  const errorDisplay = useApiErrorDisplay();
  const { success } = useToast();

  const split = async () => {
    setSplitting(true);
    try {
      await splitHolidaySibling(periodId, registrationId);
      success("Die Anmeldung ist von den Geschwistern getrennt.");
      setConfirming(false);
      await onSplit();
    } catch (err) {
      logger.error("holiday_registration_sibling_split_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirming(false);
      void errorDisplay.show(err, { object: "die Geschwister" });
    } finally {
      setSplitting(false);
    }
  };

  return (
    <SectionCard
      title="Geschwister"
      description="Die Kinder dieser Anmeldungen zählen beim Ferienbeitrag zusammen. Das Kind mit dem höchsten Beitrag ist das 1. Kind."
      action={
        invoiced ? undefined : (
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => setConfirming(true)}
          >
            Trennen
          </Button>
        )
      }
    >
      <div className="space-y-2 text-sm text-gray-700">
        <ul className="space-y-1">
          {siblings.map((sibling) => (
            <li key={sibling.registrationId} className="wrap-anywhere">
              <NavigationLink
                href={traegerPath(
                  `/traeger/ferien/${periodId}/anmeldungen/${sibling.registrationId}`,
                )}
                className="font-medium text-gray-900 underline"
              >
                Anmeldung Nr. {sibling.registrationId}
              </NavigationLink>
              {": "}
              {sibling.children.map((child) => child.name).join(", ")}
              {sibling.cancelled ? " (storniert, zählt nicht mit)" : null}
            </li>
          ))}
        </ul>
        {invoiced ? (
          <p>
            Für diese Anmeldung gibt es schon eine Rechnung. Geschwister können
            Sie jetzt nicht mehr ändern.
          </p>
        ) : null}
      </div>
      <ConfirmationModal
        isOpen={confirming}
        title="Von den Geschwistern trennen?"
        confirmText="Trennen"
        isConfirmLoading={splitting}
        loadingText="Trennt …"
        onClose={() => setConfirming(false)}
        onConfirm={() => void split()}
      >
        <p className="text-sm text-gray-700">
          Die Kinder dieser Anmeldung zählen dann nicht mehr mit den anderen
          Anmeldungen zusammen. moto berechnet den Ferienbeitrag neu, auch einen
          schon bestätigten. Eltern bekommen keine Nachricht von moto.
        </p>
      </ConfirmationModal>
    </SectionCard>
  );
}
