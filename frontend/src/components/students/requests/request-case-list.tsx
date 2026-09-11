"use client";

/**
 * Die kompakte Liste der Kinder mit offenen Anfragen (#2267). Sie zeigt nur,
 * WER etwas offen hat — entschieden wird rechts (breit) oder auf der nächsten
 * Ansicht (schmal). Vorher standen alle Entscheiden-Karten aller Kinder
 * untereinander; bei zwanzig Kindern war das nicht mehr zu überblicken.
 */

import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import {
  caseDates,
  caseTypeLabels,
  openRequestCount,
  pastRequestCount,
  type CaseBuckets,
  type OpenCase,
} from "./case-model";

export function caseRowID(caseKey: string): string {
  return `request-case-${caseKey}`;
}

function countLabel(childCase: OpenCase): string {
  if (childCase.past) {
    const count = pastRequestCount(childCase);
    return count === 1
      ? "1 abgelaufene Anfrage"
      : `${count} abgelaufene Anfragen`;
  }
  const count = openRequestCount(childCase);
  return count === 1 ? "1 offene Anfrage" : `${count} offene Anfragen`;
}

function CaseRow({
  childCase,
  selected,
  onSelect,
}: Readonly<{
  childCase: OpenCase;
  selected: boolean;
  onSelect: (childCase: OpenCase) => void;
}>) {
  const dates = caseDates(childCase);
  const typeLabels = caseTypeLabels(childCase);
  return (
    <li>
      <button
        type="button"
        id={caseRowID(childCase.key)}
        aria-current={selected ? "true" : undefined}
        onClick={() => onSelect(childCase)}
        className={`flex min-h-[44px] w-full flex-col items-start gap-1 border-b border-gray-100 px-4 py-3 text-left last:border-b-0 hover:bg-gray-50 ${
          selected ? "bg-gray-50" : ""
        }`}
      >
        <span className="w-full truncate font-semibold text-gray-900">
          {childCase.studentName}
        </span>
        <span className="text-sm text-gray-600">
          {countLabel(childCase)}
          {typeLabels.length > 0 ? ` · ${typeLabels.join(", ")}` : ""}
        </span>
        <span className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-500">
          {childCase.groupName ? <span>{childCase.groupName}</span> : null}
          {dates.length > 0 ? (
            <span>
              Betrifft: {dates.map((date) => formatDate(date)).join(", ")}
            </span>
          ) : null}
        </span>
        <span className="flex flex-wrap items-center gap-1">
          {childCase.urgentToday && !childCase.past ? (
            <StatusBadge tone="orange" label="Heute betroffen" />
          ) : null}
          {childCase.conflicts.length > 0 ? (
            <StatusBadge tone="red" label="Diese Anfragen widersprechen sich" />
          ) : null}
          {childCase.familyProtected ? (
            <StatusBadge tone="blue" label="Familienschutz" />
          ) : null}
        </span>
      </button>
    </li>
  );
}

function CaseBucket({
  id,
  title,
  description,
  cases,
  selectedKey,
  onSelect,
}: Readonly<{
  id: string;
  title: string;
  description: string;
  cases: readonly OpenCase[];
  selectedKey: string | null;
  onSelect: (childCase: OpenCase) => void;
}>) {
  if (cases.length === 0) return null;
  return (
    <section aria-labelledby={id} className="space-y-1">
      <div className="px-4 pt-3">
        <h2 id={id} className="font-semibold text-gray-900">
          {title}
        </h2>
        <p className="text-sm text-gray-600">{description}</p>
      </div>
      <ul>
        {cases.map((childCase) => (
          <CaseRow
            key={childCase.key}
            childCase={childCase}
            selected={childCase.key === selectedKey}
            onSelect={onSelect}
          />
        ))}
      </ul>
    </section>
  );
}

export function RequestCaseList({
  buckets,
  selectedKey,
  onSelect,
}: Readonly<{
  buckets: CaseBuckets;
  selectedKey: string | null;
  onSelect: (childCase: OpenCase) => void;
}>) {
  return (
    <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
      <CaseBucket
        id="request-group-urgent"
        title="Heute wichtig"
        description="Diese Kinder sind heute betroffen."
        cases={buckets.urgent}
        selectedKey={selectedKey}
        onSelect={onSelect}
      />
      <CaseBucket
        id="request-group-later"
        title="Weitere Anfragen"
        description="Diese Anfragen sind nicht für heute dringend."
        cases={buckets.later}
        selectedKey={selectedKey}
        onSelect={onSelect}
      />
      <CaseBucket
        id="request-group-expired"
        title="Abgelaufen"
        description="Diese Anfragen betreffen nur vergangene Tage. Sie ändern nichts mehr."
        cases={buckets.expired}
        selectedKey={selectedKey}
        onSelect={onSelect}
      />
    </div>
  );
}
