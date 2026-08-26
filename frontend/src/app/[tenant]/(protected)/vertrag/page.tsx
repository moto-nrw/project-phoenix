"use client";

import useSWR from "swr";
import { Alert } from "~/components/ui/alert";
import {
  SkeletonRegion,
  CardSkeleton,
  TableSkeleton,
} from "~/components/ui/page-skeletons";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { fetchContractOverview, formatCents } from "~/lib/contract-api";
import { formatDate } from "~/lib/date-helpers";
import {
  BillingContactCard,
  ChildQuotaCard,
  ContractFactsCard,
  InvoiceTable,
} from "~/components/contract/contract-cards";

// Vertrag (#1459 Demo): was die Schule gebucht hat und wann sie zahlt.
//
// Die Seite heißt bewusst „Vertrag" und nicht „Abrechnung" — „Abrechnung"
// ist bereits die Lohnabrechnung unter /payroll. Zwei Einträge mit demselben
// Wortstamm würde man als Dopplung lesen (.claude/rules/verstaendlichkeit.md).
//
// Alles hier ist Anzeige. Geschrieben wird ausschließlich im Operator-Portal;
// das Backend stellt unter /api/contract auch gar keine Schreibroute bereit.

function ContractDataSkeleton() {
  return (
    <SkeletonRegion label="Vertragsdaten werden geladen">
      <div className="space-y-5">
        <CardSkeleton rows={3} />
        <CardSkeleton rows={3} />
        <TableSkeleton rows={4} columns={5} />
      </div>
    </SkeletonRegion>
  );
}

export default function ContractPage() {
  const { isReady, isLoading: permissionLoading } = useRequirePermission([
    "config:manage",
  ]);
  const tenantSlug = useTenantSlugSafe();

  const { data: overview, error } = useSWR(
    isReady && tenantSlug ? `${tenantSlug}:contract-overview` : null,
    fetchContractOverview,
    { keepPreviousData: false },
  );

  const showSkeleton = permissionLoading || !isReady || (!error && !overview);

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Vertrag" />
      {showSkeleton ? (
        <ContractDataSkeleton />
      ) : error ? (
        <Alert
          type="error"
          message="Die Vertragsdaten konnten nicht geladen werden."
        />
      ) : (
        overview && (
          <div className="space-y-5">
            {/* Der eine Satz, der die ganze Seite erklärt: Anzeige, keine
                Eingabe. Ohne ihn liest man die leeren Felder als „hier fehlt
                etwas, das ich eintragen soll". */}
            <p className="max-w-3xl text-sm text-gray-600">
              Ihr Vertrag mit moto und Ihre Rechnungen. Diese Angaben pflegt das
              moto-Team. Sie können hier nichts ändern.
            </p>

            {!overview.configured && (
              <Alert
                type="info"
                message="Für Ihre Schule ist noch kein Vertrag hinterlegt. Das moto-Team trägt die Daten ein, sobald der Vertrag steht."
              />
            )}

            {overview.nextDue && (
              <Alert
                type={overview.nextDue.overdue ? "warning" : "info"}
                message={
                  overview.nextDue.overdue
                    ? `Die Rechnung für ${overview.nextDue.periodLabel} über ${formatCents(overview.nextDue.amountCents)} war am ${formatDate(overview.nextDue.dueDate)} fällig und ist noch offen. Bitte prüfen Sie die Zahlung.`
                    : `Nächste Zahlung: ${formatCents(overview.nextDue.amountCents)} für ${overview.nextDue.periodLabel}, fällig am ${formatDate(overview.nextDue.dueDate)}.`
                }
              />
            )}

            <div className="grid gap-5 lg:grid-cols-3">
              <ContractFactsCard overview={overview} />
              <ChildQuotaCard overview={overview} />
              <BillingContactCard overview={overview} />
            </div>

            {overview.note.trim() !== "" && (
              <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
                <h2 className="text-base font-semibold text-gray-900 sm:text-lg">
                  Hinweis von moto
                </h2>
                <p className="mt-2 text-sm whitespace-pre-line text-gray-700">
                  {overview.note}
                </p>
              </div>
            )}

            <div className="space-y-3">
              <div>
                <h2 className="text-base font-semibold text-gray-900 sm:text-lg">
                  Zahlungsplan
                </h2>
                <p className="text-sm text-gray-600">
                  Wann welche Rechnung fällig ist und ob moto die Zahlung
                  erhalten hat. Den Status setzt das moto-Team.
                  {overview.openAmountCents > 0
                    ? ` Offen sind zurzeit ${formatCents(overview.openAmountCents)}.`
                    : ""}
                </p>
              </div>
              <InvoiceTable invoices={overview.invoices} />
            </div>
          </div>
        )
      )}
    </div>
  );
}
