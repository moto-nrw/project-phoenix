"use client";

// Die Versionen eines Betreuungsvertrags (#4681). Gibt die Schule eine
// Änderung der Betreuung frei, die Eltern beantragt haben, bekommt der
// Vertrag eine neue Version mit Wirkdatum. Ältere Versionen bleiben lesbar,
// jede mit ihrem PDF. Die Liste folgt dem Muster „mehrere Einträge mit
// Aktionen“ des UI-Kits: eine Karte, weiße Zeilen, eine bündige Spalte mit
// gleichrangigen Knöpfen.

import { Download, History } from "lucide-react";
import { useState } from "react";
import {
  formatService,
  serviceDays,
} from "~/components/traeger/care-contract-format";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/data-field";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { createLogger } from "~/lib/logger";
import {
  type ContractService,
  type ContractVersion,
  downloadCareContractVersion,
} from "~/lib/traeger-care-contracts-api";

const logger = createLogger({ component: "CareContractVersions" });

/** Where a version stands on a day, for its badge. */
export type VersionPhase =
  "current" | "planned" | "past" | "pending" | "rejected";

const PHASE_TONES: Record<VersionPhase, StatusBadgeTone> = {
  current: "green",
  planned: "blue",
  past: "outline",
  pending: "gray",
  rejected: "red",
};

/** The phase of a version on the Berlin day `today` ("YYYY-MM-DD"). */
export function versionPhase(
  version: ContractVersion,
  today: string,
): VersionPhase {
  if (version.status === "beantragt") return "pending";
  if (version.status === "abgelehnt") return "rejected";
  if (version.effectiveFrom > today) return "planned";
  if (version.validUntil !== null && version.validUntil < today) return "past";
  return "current";
}

function phaseLabel(version: ContractVersion, phase: VersionPhase): string {
  switch (phase) {
    case "current":
      return "Gilt jetzt";
    case "planned":
      return `Gilt ab ${formatDate(version.effectiveFrom)}`;
    case "past":
      return "Abgelöst";
    case "pending":
      return "Noch nicht wirksam";
    case "rejected":
      return "Abgelehnt";
  }
}

/** „01.03.2027 bis 31.07.2027“, or „ab 01.03.2027“ while it holds on. */
function formatValidity(version: ContractVersion): string {
  const until = version.validUntil ?? version.endsOn;
  return until
    ? `${formatDate(version.effectiveFrom)} bis ${formatDate(until)}`
    : `ab ${formatDate(version.effectiveFrom)}`;
}

function dayText(services: readonly ContractService[]): string {
  return services.map((service) => formatService(service)).join(", ");
}

/**
 * The weekdays whose booked care differs from the previous version: another
 * module, another end time, or care added or dropped.
 */
export function changedWeekdays(
  version: ContractVersion,
  previous: ContractVersion | undefined,
): Set<string> {
  const changed = new Set<string>();
  if (!previous) return changed;
  const key = (services: readonly ContractService[]) =>
    services
      .map((service) => `${service.careOfferingId}|${service.endsAt}`)
      .sort()
      .join(",");
  const before = new Map(
    serviceDays(previous.services).map((day) => [day.weekday, day.services]),
  );
  for (const day of serviceDays(version.services)) {
    if (key(day.services) !== key(before.get(day.weekday) ?? [])) {
      changed.add(day.weekday);
    }
  }
  return changed;
}

/**
 * The newest version that is planned but not yet in force, if any: the
 * screen tells the office from when the contract changes.
 */
export function plannedVersion(
  versions: readonly ContractVersion[],
  today: string,
): ContractVersion | undefined {
  return [...versions]
    .reverse()
    .find((version) => versionPhase(version, today) === "planned");
}

export function CareContractVersions({
  contractId,
  versions,
}: Readonly<{
  contractId: string;
  versions: readonly ContractVersion[];
}>) {
  const today = useBerlinToday();
  const actionError = useApiErrorDisplay();
  const [loading, setLoading] = useState<number | null>(null);

  const download = async (number: number) => {
    setLoading(number);
    try {
      const file = await downloadCareContractVersion(contractId, number);
      downloadBlob(file.blob, file.filename);
    } catch (err) {
      logger.error("traeger_care_contract_version_download_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void actionError.show(err, { object: `das PDF von Version ${number}` });
    } finally {
      setLoading(null);
    }
  };

  // Newest first: the office looks for what changes next.
  const ordered = [...versions].sort((a, b) => b.number - a.number);
  const byNumber = new Map(
    versions.map((version) => [version.number, version]),
  );

  return (
    <SectionCard
      title="Versionen"
      icon={History}
      titleBadge={
        versions.length > 1 ? (
          <StatusBadge label={`${versions.length} Versionen`} tone="gray" />
        ) : null
      }
      description="Gibt die Schule eine Änderung der Betreuung frei, entsteht eine neue Version. Frühere Versionen bleiben hier lesbar."
    >
      <ResponsiveActionList>
        {ordered.map((version) => {
          const phase = versionPhase(version, today);
          const changed = changedWeekdays(
            version,
            byNumber.get(version.number - 1),
          );
          return (
            <li
              key={version.number}
              className="py-4 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-start"
            >
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <h3 className="text-sm font-semibold text-gray-900">
                    Version {version.number}
                  </h3>
                  <StatusBadge
                    label={phaseLabel(version, phase)}
                    tone={PHASE_TONES[phase]}
                  />
                </div>
                <p className="mt-1 text-sm text-gray-600">
                  {formatValidity(version)} ·{" "}
                  {version.source === "buchungsaenderung"
                    ? "Änderung der Betreuung"
                    : "Vertragsabschluss"}
                  {changed.size > 0
                    ? ` · ${changed.size === 1 ? "1 Tag" : `${changed.size} Tage`} geändert`
                    : null}
                </p>
                <div className="mt-3">
                  <DataGrid>
                    {serviceDays(version.services).map((day) => (
                      <DataField key={day.weekday} label={day.label} inline>
                        <span className="inline-flex flex-wrap items-center justify-end gap-1.5">
                          {changed.has(day.weekday) ? (
                            <StatusBadge
                              label="Geändert"
                              tone="orange"
                              size="xs"
                            />
                          ) : null}
                          {day.services.length === 0 ? (
                            <span className="font-normal text-gray-500">
                              Keine Betreuung
                            </span>
                          ) : (
                            dayText(day.services)
                          )}
                        </span>
                      </DataField>
                    ))}
                  </DataGrid>
                </div>
              </div>
              <div className="mt-3 flex sm:mt-0 sm:justify-end">
                <Button
                  type="button"
                  variant="surface"
                  size="compact"
                  aria-label={`PDF von Version ${version.number} herunterladen`}
                  disabled={loading !== null}
                  onClick={() => void download(version.number)}
                >
                  <Download className="mr-1.5 h-4 w-4" aria-hidden="true" />
                  {loading === version.number ? "Wird geladen …" : "PDF"}
                </Button>
              </div>
            </li>
          );
        })}
      </ResponsiveActionList>
    </SectionCard>
  );
}
