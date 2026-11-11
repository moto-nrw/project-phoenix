"use client";

// Verlauf der Vergabe nach Kriterien (#4700): jede Entscheidung des Büros mit
// Vorschlag, Begründung und den Kriterien, die das Kind zu diesem Zeitpunkt
// erfüllt hat. Nur zum Lesen; neueste zuerst.

import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import NavigationLink from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatChatDateTime } from "~/lib/date-helpers";
import type { HolidayAllocationDecision } from "~/lib/traeger-holiday-allocation-api";
import { traegerPath } from "~/lib/traeger-url";
import { formatHolidayWeek } from "./holiday-registration-format";

function columns(
  periodId: string,
): DataTableColumn<HolidayAllocationDecision>[] {
  return [
    {
      key: "child",
      header: "Kind",
      render: (decision) => (
        <NavigationLink
          href={traegerPath(
            `/traeger/ferien/${periodId}/anmeldungen/${decision.registrationId}`,
          )}
          className="font-medium text-gray-900 hover:underline"
        >
          {decision.childFirstName} {decision.childLastName}
        </NavigationLink>
      ),
      stacked: "title",
    },
    {
      key: "week",
      header: "Woche",
      render: (decision) =>
        formatHolidayWeek(decision.weekStartsOn, decision.weekEndsOn),
      stacked: "meta",
    },
    {
      key: "decision",
      header: "Entscheidung",
      render: (decision) => (
        <StatusBadge
          tone={decision.placement === "placed" ? "green" : "warning"}
          size="xs"
          label={decision.placement === "placed" ? "Platz" : "Warteliste"}
        />
      ),
      stacked: "meta",
    },
    {
      key: "proposal",
      header: "Vorschlag",
      render: (decision) => (
        <span className="text-gray-700">
          {decision.proposal === "placed" ? "Platz" : "Warteliste"}, Rang{" "}
          {decision.rank}
          {decision.placement === decision.proposal ? null : (
            <span className="block text-xs font-medium text-gray-900">
              Abgewichen
            </span>
          )}
        </span>
      ),
      stacked: "field",
    },
    {
      key: "reason",
      header: "Begründung",
      render: (decision) =>
        decision.reason ? (
          <span className="wrap-anywhere text-gray-700">{decision.reason}</span>
        ) : (
          "–"
        ),
      stacked: "field",
    },
    {
      key: "criteria",
      header: "Kriterien zu diesem Zeitpunkt",
      render: (decision) =>
        decision.criteriaMet.length > 0 ? decision.criteriaMet.join(", ") : "–",
      stacked: "field",
    },
    {
      key: "decidedAt",
      header: "Zeit",
      render: (decision) => `${formatChatDateTime(decision.decidedAt)} Uhr`,
      sortValue: (decision) => decision.decidedAt,
      stacked: "meta",
    },
  ];
}

export function HolidayAllocationHistory({
  periodId,
  decisions,
  defaultCollapsed,
}: Readonly<{
  periodId: string;
  decisions: readonly HolidayAllocationDecision[];
  /** Folded while open places wait, so the list stays the main part. */
  defaultCollapsed: boolean;
}>) {
  return (
    <SectionCard
      title={`Verlauf der Entscheidungen (${decisions.length})`}
      description="Jede Entscheidung des Büros mit dem Vorschlag von moto. Nur zum Lesen."
      collapsible
      defaultCollapsed={defaultCollapsed && decisions.length > 0}
      headingLevel={2}
    >
      {decisions.length === 0 ? (
        <p className="text-sm text-gray-600">Noch keine Entscheidung.</p>
      ) : (
        <DataTable
          columns={columns(periodId)}
          rows={[...decisions]}
          getRowKey={(decision) => decision.id}
          stackedOnMobile
          stackedLayout="row"
        />
      )}
    </SectionCard>
  );
}
