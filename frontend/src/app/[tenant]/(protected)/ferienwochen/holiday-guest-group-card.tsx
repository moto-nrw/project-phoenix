"use client";

import { useState } from "react";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import { groups as groupResource } from "~/lib/swr/groups";
import {
  assignHolidayGuestGroup,
  type HostedHolidayWeek,
} from "~/lib/holiday-host-api";
import { formatDate, berlinTodayISO } from "~/lib/date-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr";

const logger = createLogger({ component: "HolidayGuestGroupCard" });

function guestCount(count: number): string {
  return count === 1 ? "1 Feriengast" : `${count} Feriengäste`;
}

/** Die Meldung nach der Wahl: Feriengäste in der Gruppe, eigene Kinder eingeplant. */
function transferMessage(guests: number, ownChildren: number): string {
  const parts: string[] = [];
  if (guests > 0) parts.push(`${guestCount(guests)} in der Gruppe.`);
  if (ownChildren > 0) {
    parts.push(
      ownChildren === 1
        ? "1 Kind Ihrer OGS ist eingeplant."
        : `${ownChildren} Kinder Ihrer OGS sind eingeplant.`,
    );
  }
  return parts.length > 0
    ? parts.join(" ")
    : "Die Gruppe ist gewählt. Noch ist kein Kind bestätigt.";
}

/**
 * Feriengruppe einer Woche (#4666): Die Leitung wählt eine Gruppe ihrer OGS.
 * Die bestätigten Feriengäste der Woche kommen dann in diese Gruppe und in
 * den Check-in, mit Betreuung, Notfall und Abholung. Mit der Wahl plant moto
 * auch die gebuchten Tage der eigenen Kinder ein (#6678); sie bleiben in
 * ihrer Gruppe. Wer keine Kinder anlegen
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
  const groups = useSWRAuth(groupResource.key(), groupResource.fetcher);
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
      const transfer = await assignHolidayGuestGroup(week.id, groupId);
      success(transferMessage(transfer.guests, transfer.ownChildren));
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
    const group = `der Gruppe ${groupName ?? "der Woche"}`;
    const later = "Neu bestätigte Kinder kommen innerhalb einer Stunde dazu.";
    if (week.guests === 0) {
      // Eine Woche nur mit eigenen Kindern zählt keine „0 Feriengäste“.
      status = `Noch ist kein Feriengast bestätigt. Gewählt ist ${groupName ? `die Gruppe ${groupName}` : "eine Gruppe"}. ${later}`;
    } else if (week.startsOn > berlinTodayISO()) {
      // Before the week the guests wait outside the group, like children
      // before their first care day.
      status = `Ab ${formatDate(week.startsOn)} ${week.guests === 1 ? "steht" : "stehen"} ${guestCount(week.guests)} in ${group}. ${later}`;
    } else {
      status = `${guestCount(week.guests)} in ${group}. ${later}`;
    }
  }

  return (
    <SectionCard
      title="Feriengruppe"
      description="Feriengäste sind Kinder anderer Schulen und Kinder ohne Schule beim Träger. In der Feriengruppe melden Sie die Kinder an und ab. Sie sehen Betreuung, Notfall, Abholung, Gesundheit und die Angaben, die der Träger freigibt. Kinder Ihrer OGS bleiben in ihrer Gruppe."
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
              disabledReason={
                groupId ? undefined : "Wählen Sie zuerst eine Gruppe aus."
              }
              tooltip="Setzt die Feriengäste der Woche in diese Gruppe und in den Check-in."
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
