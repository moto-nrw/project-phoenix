"use client";

// Korrekturen, die eine neu ausgestellte Rechnung aus der stornierten
// übernimmt (#5663). Sie zählen in der Vorschau schon mit; das Büro prüft jede
// vor der Freigabe, ändert oder entfernt sie. Gutschriften der stornierten
// Rechnung sind eigene Dokumente: Sie stehen nur als Hinweis hier.

import { useState } from "react";
import { Alert } from "~/components/ui/alert";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type BillingCarriedCorrection,
  type BillingChild,
  type BillingRunDetail,
  removeCarriedCorrection,
} from "~/lib/traeger-billing-api";
import { totalAfterRemovingCarriedCorrection } from "./billing-carried-correction-model";
import { recipientName } from "./billing-document-card";
import { formatEuro } from "./contribution-format";

const logger = createLogger({ component: "BillingCarriedCorrectionsCard" });

interface CarriedRow {
  readonly familyName: string;
  readonly child: BillingChild;
  readonly carried: BillingCarriedCorrection;
}

/** „−20,00 € aus FE-0001“, und was die Grenze bei 0 € davon übrig lässt. */
function amountText(carried: BillingCarriedCorrection): string {
  const source = `${formatEuro(carried.sourceAmountCents)} aus ${carried.sourceNumber}`;
  return carried.amountCents === carried.sourceAmountCents
    ? source
    : `${source}. Hier ${formatEuro(carried.amountCents)}, weil der Betrag nicht unter 0 € fällt`;
}

export function BillingCarriedCorrectionsCard({
  run,
  onChange,
  onRemoved,
}: Readonly<{
  run: BillingRunDetail;
  /** Opens the form that replaces the correction with a new one. */
  onChange: (child: BillingChild, carried: BillingCarriedCorrection) => void;
  onRemoved: (run: BillingRunDetail) => void;
}>) {
  const [removing, setRemoving] = useState<CarriedRow | null>(null);
  const [busy, setBusy] = useState(false);
  const errors = useApiFormError();
  const { success } = useToast();

  const rows: CarriedRow[] = run.documents.flatMap((document) =>
    document.children.flatMap((child) =>
      child.carriedCorrections.map((carried) => ({
        familyName: recipientName(document),
        child,
        carried,
      })),
    ),
  );
  const credits = run.documents.flatMap((document) =>
    document.replacedCredits.map((credit) => ({
      familyName: recipientName(document),
      replaced: document.referenceNumber,
      credit,
    })),
  );

  const remove = async () => {
    if (!removing) return;
    errors.clear();
    setBusy(true);
    try {
      const updated = await removeCarriedCorrection(
        run.id,
        removing.carried.positionId,
      );
      setRemoving(null);
      success("Die Korrektur ist entfernt.");
      onRemoved(updated);
    } catch (err) {
      logger.error("billing_carried_correction_remove_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void errors.show(err, { object: "die Korrektur" });
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      {rows.length > 0 ? (
        <SectionCard
          title="Übernommene Korrekturen"
          description="Diese Korrekturen stammen aus der stornierten Rechnung und zählen schon in der neuen. Prüfen Sie jede vor der Freigabe."
        >
          <ResponsiveActionList>
            {rows.map((row) => (
              <li
                key={row.carried.positionId}
                className="flex items-start justify-between gap-3 py-3 sm:col-span-2 sm:grid sm:grid-cols-subgrid"
              >
                <div className="min-w-0 text-sm wrap-anywhere">
                  <p className="font-medium text-gray-900">
                    {row.child.firstName} {row.child.lastName}
                    <span className="font-normal text-gray-600">
                      {" "}
                      · Rechnung an {row.familyName}
                    </span>
                  </p>
                  <p className="text-gray-700 tabular-nums">
                    {amountText(row.carried)}
                  </p>
                  <p className="text-gray-700">{row.carried.reason}</p>
                </div>
                <OverflowMenu
                  ariaLabel={`Aktionen für die Korrektur von ${row.child.firstName} über ${formatEuro(row.carried.amountCents)}`}
                  items={[
                    {
                      label: "Ändern",
                      onClick: () => onChange(row.child, row.carried),
                    },
                    {
                      label: "Entfernen",
                      onClick: () => setRemoving(row),
                    },
                  ]}
                />
              </li>
            ))}
          </ResponsiveActionList>
        </SectionCard>
      ) : null}
      {credits.length > 0 ? (
        <Alert
          type="info"
          title="Gutschriften der stornierten Rechnung"
          message={`${credits
            .map(
              ({ familyName, replaced, credit }) =>
                `${familyName}: Gutschrift ${credit.number} vom ${formatDate(credit.issuedOn)} über ${formatEuro(credit.totalCents)} zu ${replaced}.`,
            )
            .join(
              " ",
            )} Eine Gutschrift ist ein eigenes Dokument und wird nicht übernommen. Prüfen Sie, ob die neue Rechnung das berücksichtigen muss.`}
        />
      ) : null}
      <ConfirmDeleteModal
        isOpen={removing !== null}
        title="Korrektur entfernen?"
        description={
          removing ? (
            <p>
              Die neue Rechnung enthält dann{" "}
              {formatEuro(removing.carried.amountCents)} für{" "}
              {removing.child.firstName} nicht mehr. Der Betrag wird{" "}
              {formatEuro(
                totalAfterRemovingCarriedCorrection(
                  removing.child,
                  removing.carried.positionId,
                ),
              )}
              . In diesem Lauf lässt sich das nicht rückgängig machen.
            </p>
          ) : null
        }
        gate={{ mode: "twoStep", firstStepLabel: "Entfernen" }}
        confirmLabel="Jetzt entfernen"
        loadingLabel="Wird entfernt …"
        loading={busy}
        error={errors.error}
        onClose={() => {
          setRemoving(null);
          errors.clear();
        }}
        onConfirm={remove}
      />
    </>
  );
}
