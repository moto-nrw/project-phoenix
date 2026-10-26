"use client";

import { useState } from "react";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import { groupService } from "~/lib/api";
import {
  assignHolidayGuestGroup,
  type HostedHolidayWeek,
} from "~/lib/holiday-host-api";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr";

const logger = createLogger({ component: "HolidayGuestGroupCard" });

function guestCount(count: number): string {
  return count === 1 ? "1 Feriengast" : `${count} Feriengäste`;
}

/**
 * Feriengruppe einer Woche (#4666): Die Leitung wählt eine Gruppe ihrer OGS.
 * Die bestätigten Feriengäste der Woche kommen dann in diese Gruppe und in
 * den Check-in, mit Betreuung, Notfall und Abholung. Wer keine Kinder anlegen
 * darf, sieht nur, welche Gruppe gewählt ist. Die Seite gibt der Karte je
 * Woche einen eigenen Schlüssel, damit eine nicht übernommene Auswahl nicht
 * in die nächste Woche wandert.
 */
export function HolidayGuestGroupCard({
  week,
  canAssign,
  onAssigned,
}: Readonly<{
  week: HostedHolidayWeek;
  canAssign: boolean;
  onAssigned: () => void;
}>) {
  const groups = useSWRAuth("holiday-guest-groups", () =>
    groupService.getGroups(),
  );
  const groupsError = useSwrLoadError(groups.error, "die Gruppen", () =>
    groups.mutate(),
  );
  const [chosen, setChosen] = useState<string>("");
  const [saving, setSaving] = useState(false);
  const { success } = useToast();
  const { show } = useApiErrorDisplay();

  const groupId = chosen || week.guestGroupId || "";
  const groupName = groups.data?.find(
    (group) => group.id === week.guestGroupId,
  )?.name;

  const assign = async () => {
    if (!groupId) return;
    setSaving(true);
    try {
      const guests = await assignHolidayGuestGroup(week.id, groupId);
      success(
        guests === 0
          ? "Die Gruppe ist gewählt. Noch ist kein Feriengast bestätigt."
          : `${guestCount(guests)} in der Gruppe.`,
      );
      setChosen("");
      onAssigned();
    } catch (error) {
      logger.error("holiday_guest_group_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      void show(error, {
        object: "die Feriengäste",
        retry: () => void assign(),
      });
    } finally {
      setSaving(false);
    }
  };

  let status: string;
  if (!week.guestGroupId) {
    status = canAssign
      ? "Noch keine Gruppe gewählt. Erst mit einer Gruppe sehen die Betreuungskräfte die Feriengäste in der Gruppe und im Check-in."
      : "Noch keine Gruppe gewählt. Die Gruppe wählt die Leitung.";
  } else {
    status = `${guestCount(week.guests)} in der Gruppe ${groupName ?? "der Woche"}. Neu bestätigte Kinder kommen innerhalb einer Stunde dazu.`;
  }

  return (
    <SectionCard
      title="Feriengruppe"
      description="Feriengäste sind Kinder anderer Schulen und Kinder ohne Schule beim Träger. In der Feriengruppe sehen Sie Betreuung, Notfall und Abholung und melden die Kinder an und ab."
    >
      <div className="flex flex-col gap-4">
        <p className="text-sm text-gray-700">{status}</p>
        {groupsError && <LoadErrorAlert error={groupsError} />}
        {canAssign && (
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
            <CustomSelect
              ariaLabel="Gruppe für die Feriengäste"
              placeholder="Gruppe wählen"
              value={groupId}
              onChange={setChosen}
              options={(groups.data ?? []).map((group) => ({
                value: group.id,
                label: group.name,
              }))}
              disabled={saving || !groups.data}
              className="sm:max-w-xs"
            />
            <Button
              type="button"
              size="md"
              variant="primary"
              disabled={!groupId || saving}
              onClick={() => void assign()}
            >
              {saving ? "Wird übernommen…" : "Feriengäste übernehmen"}
            </Button>
          </div>
        )}
      </div>
    </SectionCard>
  );
}
