import { CollectionGrid } from "~/components/ui/collection-grid";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type { FormerStaff } from "~/lib/staff-api";

/**
 * Former employments under the Ehemalige filter of the staff list (#3403).
 * The cards are a display: there is no read-only staff record to open yet
 * (#3405), so they carry no link, pointer or hover.
 */
export function FormerStaffCards({
  members,
}: {
  readonly members: readonly FormerStaff[];
}) {
  return (
    <CollectionGrid>
      {members.map((member) => (
        <SectionCard key={member.id}>
          <div className="flex min-h-[104px] flex-col">
            <div className="mb-3 flex items-start justify-between gap-3">
              <h3 className="min-w-0 flex-1 truncate text-base font-bold text-gray-900">
                {member.firstName} {member.lastName}
              </h3>
              <StatusBadge label="Ehemalig" tone="gray" />
            </div>
            <p className="text-sm text-gray-600">
              In moto eingetragen vom {formatDate(member.startedOn)} bis{" "}
              {formatDate(member.endedOn)}.
            </p>
          </div>
        </SectionCard>
      ))}
    </CollectionGrid>
  );
}
