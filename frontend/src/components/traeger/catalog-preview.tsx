"use client";

// Vorschau und Ergebnis des Übertragens einer Anmeldephase (#4704): je
// Schule, was das Übertragen tut, welches Feld sich ändert und welches die
// Schule selbst geändert hat und deshalb behält.

import { Alert } from "~/components/ui/alert";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { SectionCard } from "~/components/ui/section-card";
import { SectionLabel } from "~/components/ui/section-label";
import { StatusBadge } from "~/components/ui/status-badge";
import type { TraegerSchool } from "~/lib/traeger-api";
import type {
  TraegerFieldChange,
  TraegerOfferingPlan,
  TraegerRollout,
  TraegerSchoolPlan,
} from "~/lib/traeger-catalog-api";
import { formatMonth } from "./care-billing-format";
import {
  actionBadge,
  fieldLabel,
  formatFieldValue,
  outcomeBadge,
  shownChanges,
} from "./catalog-format";
import { refusalText } from "./catalog-phase-cards";

const UNCHANGED = { label: "Bleibt gleich", tone: "gray" } as const;

/** An update that changes no field, price or contract template. */
function offeringUnchanged(plan: TraegerOfferingPlan): boolean {
  return (
    plan.action === "update" &&
    shownChanges(plan.fields).every((change) => change.state === "keeps") &&
    plan.pricesFrom === null &&
    !plan.contractTemplate
  );
}

function schoolUnchanged(plan: TraegerSchoolPlan): boolean {
  return (
    plan.action === "update" &&
    shownChanges(plan.phase).every((change) => change.state === "keeps") &&
    plan.offerings.every(offeringUnchanged)
  );
}

const KEEPS_TEXT = "Die Schule hat geändert – bleibt so";

/**
 * Die Kürzung beendet keine Betreuung (#6813, ADR 0211): das Büro erfährt,
 * wie viele Kinder länger betreut werden, und sagt der Schule Bescheid.
 */
function careBeyondEndText(count: number): string {
  const children = count === 1 ? "1 Kind wird" : `${count} Kinder werden`;
  return `${children} länger betreut, als die Anmeldephase dauert. moto beendet keine Betreuung. Sagen Sie der Schule Bescheid.`;
}

function schoolName(
  schools: readonly TraegerSchool[],
  schoolId: string,
): string {
  return (
    schools.find((school) => school.id === schoolId)?.name ??
    "Schule gehört nicht mehr zum Träger"
  );
}

const CHANGE_COLUMNS: DataTableColumn<TraegerFieldChange>[] = [
  {
    key: "field",
    header: "Feld",
    render: (change) => (
      <span className="font-medium text-gray-900">
        {fieldLabel(change.field)}
      </span>
    ),
    stacked: "title",
  },
  {
    key: "school",
    header: "Jetzt in der Schule",
    render: (change) => formatFieldValue(change.field, change.school),
    stacked: "field",
  },
  {
    key: "traeger",
    header: "Vom Träger",
    render: (change) => formatFieldValue(change.field, change.traeger),
    stacked: "field",
  },
  {
    key: "result",
    header: "Ergebnis",
    render: (change) =>
      change.state === "keeps" ? (
        <span className="font-medium text-gray-900">{KEEPS_TEXT}</span>
      ) : (
        <span className="text-gray-700">Wird übernommen</span>
      ),
    stacked: "field",
  },
];

function Changes({
  changes,
  action,
  label,
}: Readonly<{
  changes: readonly TraegerFieldChange[];
  action: TraegerSchoolPlan["action"];
  label: string;
}>) {
  if (action === "create") {
    return (
      <p className="text-sm text-gray-700">
        Wird mit allen Angaben des Trägers neu angelegt.
      </p>
    );
  }
  if (action === "withdraw") {
    return (
      <p className="text-sm text-gray-700">
        Wird entfernt. Mit Anmeldungen oder Buchungen schaltet moto es nur aus.
      </p>
    );
  }
  const shown = shownChanges(changes);
  if (shown.length === 0) {
    return <p className="text-sm text-gray-700">Keine Änderung.</p>;
  }
  return (
    <DataTable
      columns={CHANGE_COLUMNS}
      rows={shown}
      getRowKey={(change) => `${label}-${change.field}`}
      stackedOnMobile
      dense
    />
  );
}

function OfferingPlan({ plan }: Readonly<{ plan: TraegerOfferingPlan }>) {
  const notes = [
    plan.pricesFrom ? `Neue Preise ab ${formatMonth(plan.pricesFrom)}.` : null,
    plan.contractTemplate ? "Die Vertragsvorlage wird übernommen." : null,
  ].filter((note): note is string => note !== null);
  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <SectionLabel as="h4">{plan.name}</SectionLabel>
        <StatusBadge
          {...(offeringUnchanged(plan) ? UNCHANGED : actionBadge(plan.action))}
        />
      </div>
      {notes.length > 0 ? (
        <p className="text-sm text-gray-700">{notes.join(" ")}</p>
      ) : null}
      <Changes changes={plan.fields} action={plan.action} label={plan.name} />
    </div>
  );
}

export function SchoolPlanCard({
  plan,
  schools,
}: Readonly<{
  plan: TraegerSchoolPlan;
  schools: readonly TraegerSchool[];
}>) {
  const rejected = plan.outcome === "rejected";
  return (
    <SectionCard
      title={schoolName(schools, plan.schoolId)}
      titleBadge={
        rejected ? (
          <StatusBadge label="Lehnt ab" tone="red" />
        ) : (
          <StatusBadge
            {...(schoolUnchanged(plan) ? UNCHANGED : actionBadge(plan.action))}
          />
        )
      }
    >
      <div className="space-y-5">
        {rejected ? (
          <Alert
            type="warning"
            message={`${refusalText(plan.errorCode) ?? "Die Schule kann die Angaben so nicht übernehmen."} Diese Schule behält ihren bisherigen Stand.`}
          />
        ) : null}
        {!rejected && plan.careBeyondEnd > 0 ? (
          <Alert
            type="warning"
            message={careBeyondEndText(plan.careBeyondEnd)}
          />
        ) : null}
        <div className="space-y-2">
          <SectionLabel as="h4">Anmeldephase</SectionLabel>
          <Changes changes={plan.phase} action={plan.action} label="phase" />
        </div>
        {plan.offerings.map((offering) => (
          <OfferingPlan key={offering.traegerOfferingId} plan={offering} />
        ))}
      </div>
    </SectionCard>
  );
}

export function RolloutResultCard({
  rollout,
  schools,
}: Readonly<{
  rollout: TraegerRollout;
  schools: readonly TraegerSchool[];
}>) {
  const columns: DataTableColumn<TraegerRollout["schools"][number]>[] = [
    {
      key: "school",
      header: "Schule",
      render: (row) => (
        <span className="font-medium text-gray-900">
          {schoolName(schools, row.schoolId)}
        </span>
      ),
      stacked: "title",
    },
    {
      key: "outcome",
      header: "Ergebnis",
      render: (row) => <StatusBadge {...outcomeBadge(row.outcome)} />,
      stacked: "meta",
    },
    {
      key: "reason",
      header: "Hinweis",
      render: (row) => {
        if (row.outcome === "rejected") {
          return refusalText(row.errorCode) ?? "Die Schule hat abgelehnt.";
        }
        if (row.outcome === "stale") {
          return "Die Schule hat seit der Vorschau etwas geändert. Sehen Sie sich die Vorschau noch einmal an.";
        }
        if (row.careBeyondEnd > 0) return careBeyondEndText(row.careBeyondEnd);
        return "–";
      },
      stacked: "field",
    },
  ];
  return (
    <SectionCard
      title="Ergebnis"
      description="Ihre Angaben sind freigegeben. So haben die Schulen sie übernommen."
    >
      <DataTable
        columns={columns}
        rows={[...rollout.schools]}
        getRowKey={(row) => row.schoolId}
        stackedOnMobile
      />
    </SectionCard>
  );
}
