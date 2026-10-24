import { CollectionGrid } from "~/components/ui/collection-grid";
import { SectionCard } from "~/components/ui/section-card";
import { TileCard } from "~/components/ui/tile-card";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type { FormerStaff } from "~/lib/staff-api";

export function FormerStaffCards({
  members,
  referrer,
  canOpen,
}: {
  readonly referrer: string;
  readonly canOpen: boolean;
  readonly members: readonly FormerStaff[];
}) {
  const tenantPath = useTenantAwarePath();
  return (
    <CollectionGrid>
      {members.map((member) => {
        const Card = canOpen ? TileCard : SectionCard;
        return (
          <Card
            key={member.id}
            href={
              canOpen
                ? tenantPath(
                    `/staff/${member.id}?from=${encodeURIComponent(referrer)}`,
                  )
                : undefined
            }
          >
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
          </Card>
        );
      })}
    </CollectionGrid>
  );
}
