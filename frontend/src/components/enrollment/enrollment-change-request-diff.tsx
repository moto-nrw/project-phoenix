"use client";

import { useMemo } from "react";
import {
  DataField,
  DataGrid,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import {
  buildEnrollmentChangeRequestDiffGroups,
  formatEnrollmentChangeRequestValue,
  type EnrollmentChangeRequestDiffCopy,
  type EnrollmentChangeRequestDiffGroup,
  type EnrollmentFormLabels,
} from "~/lib/enrollment-change-request-diff";
import { SectionLabel } from "~/components/ui/section-label";
import { cn } from "~/lib/utils";

interface EnrollmentChangeRequestDiffProps {
  readonly baseSnapshot: Record<string, unknown>;
  readonly proposedSnapshot: Record<string, unknown>;
  readonly diff: Record<string, unknown>;
  readonly beforeLabel?: string;
  readonly afterLabel?: string;
  readonly emptyLabel?: string;
  readonly emptyMessage?: string;
  readonly copy?: EnrollmentChangeRequestDiffCopy;
  /** Questions of the request's form version; without them rows show keys. */
  readonly formLabels?: EnrollmentFormLabels | null;
  readonly className?: string;
}

export function EnrollmentChangeRequestDiff({
  baseSnapshot,
  proposedSnapshot,
  diff,
  beforeLabel = "Bisher",
  afterLabel = "Neu",
  emptyLabel = "Nicht gesetzt",
  emptyMessage = "Die Anfrage enthält keine erkennbaren Feldänderungen.",
  copy,
  formLabels,
  className,
}: EnrollmentChangeRequestDiffProps) {
  const groups = useMemo(
    () =>
      buildEnrollmentChangeRequestDiffGroups({
        baseSnapshot,
        proposedSnapshot,
        diff,
        copy,
        formLabels,
      }),
    [baseSnapshot, proposedSnapshot, diff, copy, formLabels],
  );

  if (groups.length === 0) {
    return <p className="text-sm text-gray-600">{emptyMessage}</p>;
  }

  return (
    <div className={cn("space-y-3", className)}>
      {groups.map((group) => (
        <DiffGroup
          key={group.key}
          group={group}
          beforeLabel={beforeLabel}
          afterLabel={afterLabel}
          emptyLabel={emptyLabel}
          copy={copy}
        />
      ))}
    </div>
  );
}

function DiffGroup({
  afterLabel,
  beforeLabel,
  emptyLabel,
  group,
  copy,
}: {
  readonly group: EnrollmentChangeRequestDiffGroup;
  readonly beforeLabel: string;
  readonly afterLabel: string;
  readonly emptyLabel: string;
  readonly copy?: EnrollmentChangeRequestDiffCopy;
}) {
  return (
    <InfoSection title={group.label}>
      <div className="space-y-2">
        {group.rows.map((row) => (
          <div key={row.id} className="rounded-lg bg-white px-3 py-2">
            <SectionLabel as="p" className="break-words">
              {row.label}
            </SectionLabel>
            <div className="mt-2">
              <DataGrid>
                <DataField label={beforeLabel}>
                  {formatEnrollmentChangeRequestValue(
                    row.before,
                    emptyLabel,
                    copy,
                  )}
                </DataField>
                <DataField label={afterLabel}>
                  {formatEnrollmentChangeRequestValue(
                    row.after,
                    emptyLabel,
                    copy,
                  )}
                </DataField>
              </DataGrid>
            </div>
          </div>
        ))}
      </div>
    </InfoSection>
  );
}
