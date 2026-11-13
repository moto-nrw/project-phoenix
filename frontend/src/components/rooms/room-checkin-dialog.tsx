"use client";

// „Kinder anmelden“ on the room page (#6953): check children in straight
// into this room, in one step, as a scan at the room's terminal would. When
// the person may pull children from other rooms, present children can come
// along in the same step.

import { useMemo, useRef, useState } from "react";
import { DoorOpen } from "lucide-react";

import { CompactStudentCard } from "~/components/students/compact-student-card";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { EmptyState } from "~/components/ui/empty-state";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { FramedList } from "~/components/ui/framed-list";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
import { SkeletonRows } from "~/components/ui/skeleton";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import type { StudentMoveResult } from "~/lib/active-service";
import { deriveCheckinState } from "~/lib/hooks/use-school-checkin-mode";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import {
  countedDayStatus,
  isTransitLocation,
  parseLocation,
} from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import { checkInToTarget, type TargetRoomOption } from "~/lib/move-targets";
import { statusLabel } from "~/components/class-day/status-labels";
import { fetchStudents } from "~/lib/student-api";
import type { Student } from "~/lib/student-helpers";
import { useSWRAuth } from "~/lib/swr";
import { checkinTargetKeys } from "~/lib/swr/rooms-keys";

const logger = createLogger({ component: "RoomCheckinDialog" });

// The day status the child list shows, so a sick child reads „Krank“ here
// too and not „Zuhause“.
function dayStatus(student: Student): string | null {
  const status = countedDayStatus(student);
  return status ? statusLabel(status) : null;
}

/** One child the dialog offers, with where it is now. */
export interface RoomCheckinCandidate {
  readonly student: Student;
  readonly name: string;
  /** „Zuhause“, „Krank“, „In Mensa“, „Unterwegs“ … */
  readonly where: string;
  readonly present: boolean;
}

/**
 * The children the dialog offers for this room: every absent child, and the
 * present children elsewhere only when the person may pull them in. Absent
 * children come first, then by name.
 */
export function roomCheckinCandidates(
  students: readonly Student[],
  roomName: string,
  canPullPresent: boolean,
): RoomCheckinCandidate[] {
  const candidates: RoomCheckinCandidate[] = [];
  for (const student of students) {
    const state = deriveCheckinState(student.current_location);
    const present = state !== "abwesend";
    const { status, room } = parseLocation(student.current_location);
    if (present && room === roomName) continue;
    if (present && !canPullPresent) continue;
    const name =
      `${student.first_name ?? ""} ${student.second_name ?? ""}`.trim() ||
      student.name;
    const where = present
      ? room
        ? `In ${room}`
        : isTransitLocation(student.current_location)
          ? "Unterwegs"
          : status || "Anwesend"
      : (dayStatus(student) ?? (status || "Abwesend"));
    candidates.push({ student, name, where, present });
  }
  return candidates.sort(
    (a, b) =>
      Number(a.present) - Number(b.present) ||
      a.name.localeCompare(b.name, "de"),
  );
}

interface RoomCheckinDialogProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly roomName: string;
  /** This room as a target: its session, or its stay when it is released. */
  readonly target: TargetRoomOption;
  /** Present children of other rooms may be pulled in (supervisor, admin). */
  readonly canPullPresent: boolean;
  /** Called after the children were placed, to reload the room. */
  readonly onDone: () => Promise<unknown>;
}

export function RoomCheckinDialog({
  isOpen,
  onClose,
  roomName,
  target,
  canPullPresent,
  onDone,
}: RoomCheckinDialogProps) {
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLDivElement>(null);
  const formError = useApiFormError(formRef);
  const { success: toastSuccess } = useToast();

  const { data, error, isLoading, mutate } = useSWRAuth<{
    students: Student[];
  }>(isOpen ? checkinTargetKeys.roomCandidates() : null, () =>
    fetchStudents({ pageSize: 1000 }),
  );
  const loadError = useSwrLoadError(error, "die Liste der Kinder", () =>
    mutate(),
  );

  const candidates = useMemo(
    () => roomCheckinCandidates(data?.students ?? [], roomName, canPullPresent),
    [data?.students, roomName, canPullPresent],
  );
  const visible = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase("de");
    if (!needle) return candidates;
    return candidates.filter((candidate) =>
      `${candidate.name} ${candidate.student.school_class ?? ""} ${
        candidate.student.group_name ?? ""
      }`
        .toLocaleLowerCase("de")
        .includes(needle),
    );
  }, [candidates, query]);

  const close = () => {
    if (saving) return;
    setQuery("");
    setSelected(new Set());
    formError.clear();
    onClose();
  };

  const toggle = (id: string) => {
    formError.clear();
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const submit = async () => {
    const ids = [...selected];
    if (ids.length === 0) return;
    setSaving(true);
    formError.clear();
    let result: StudentMoveResult;
    try {
      // The room page pulls present children in: they move under the
      // ordinary move rules (keepPresent false).
      result = await checkInToTarget(ids, target, false);
    } catch (err) {
      logger.warn("room_checkin_failed", {
        selected_count: ids.length,
        target_kind: target.kind,
        error: err instanceof Error ? err.message : String(err),
      });
      setSaving(false);
      void formError.show(err, { object: "das Anmelden der Kinder" });
      return;
    }
    const placed = result.moved.length + result.unchanged.length;
    const skippedIds = new Set(
      result.skipped.map((item) => String(item.student_id)),
    );
    setSaving(false);
    if (skippedIds.size > 0) {
      // Partly done: the placed children leave the list after the reload,
      // the skipped ones stay selected and are named where the person looks.
      const names = candidates
        .filter((candidate) => skippedIds.has(candidate.student.id))
        .map((candidate) => candidate.name);
      setSelected(skippedIds);
      formError.invalid(
        `${placed} ${placed === 1 ? "Kind ist" : "Kinder sind"} jetzt in ${roomName}. Nicht angemeldet: ${names.join(", ")}. Prüfen Sie die Angaben dieser Kinder.`,
      );
      await onDone();
      return;
    }
    toastSuccess(
      `${placed} ${placed === 1 ? "Kind ist" : "Kinder sind"} jetzt in ${roomName}.`,
    );
    setQuery("");
    setSelected(new Set());
    onClose();
    // The room reloads through the section's useRefreshAfterWrite: a failed
    // reload shows there and never repeats the check-in.
    await onDone();
  };

  const count = selected.size;
  const absentCount = candidates.filter(
    (candidate) => !candidate.present,
  ).length;

  return (
    <Modal
      isOpen={isOpen}
      onClose={close}
      title={`Kinder in ${roomName} anmelden`}
      icon={<DoorOpen className="size-5" aria-hidden />}
      description={
        canPullPresent
          ? `Abwesende Kinder werden angemeldet. Anwesende Kinder kommen aus ihrem Raum mit. Alle sind danach sofort in ${roomName}.`
          : `Die Kinder werden angemeldet und sind danach sofort in ${roomName}.`
      }
      size="md"
      mobileSheet
      footer={
        <>
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={close}
            disabled={saving}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="success"
            size="md"
            onClick={() => void submit()}
            disabled={count === 0}
            disabledReason="Wählen Sie zuerst mindestens ein Kind aus."
            isLoading={saving}
            loadingText="Melde an …"
            tooltip={`Meldet die ausgewählten Kinder sofort an und setzt sie in ${roomName}.`}
          >
            {count === 0
              ? "Anmelden"
              : `${count} ${count === 1 ? "Kind" : "Kinder"} anmelden`}
          </Button>
        </>
      }
    >
      <div ref={formRef} className="flex flex-col gap-3">
        <FormErrorAlert message={formError.error} />
        <Input
          type="search"
          aria-label="Kind suchen"
          placeholder="Name, Klasse oder Gruppe"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          controlSize="compact"
        />
        <LoadErrorAlert error={loadError} actionLayout="responsive" />
        {isLoading ? (
          <SkeletonRows rows={5} />
        ) : visible.length === 0 && !loadError ? (
          <EmptyState
            variant="compact"
            title={
              query.trim()
                ? "Kein Kind gefunden"
                : "Alle Kinder sind schon angemeldet"
            }
            description={
              query.trim()
                ? "Prüfen Sie die Schreibweise."
                : `Es gibt gerade kein Kind, das Sie in ${roomName} anmelden können.`
            }
          />
        ) : (
          <FramedList
            maxHeight="lg"
            header={
              <span className="text-xs text-gray-600 tabular-nums">
                {absentCount} abwesend
                {canPullPresent
                  ? ` · ${candidates.length - absentCount} schon anwesend`
                  : ""}
              </span>
            }
          >
            {visible.map((candidate) => {
              const id = candidate.student.id;
              const checked = selected.has(id);
              return (
                <ChoiceTile
                  key={id}
                  as="label"
                  htmlFor={`room-checkin-${id}`}
                  selected={checked}
                  tone="green"
                  className="gap-3 rounded-none border-0"
                >
                  <Checkbox
                    id={`room-checkin-${id}`}
                    checked={checked}
                    onChange={() => toggle(id)}
                    disabled={saving}
                  />
                  <span className="min-w-0 flex-1">
                    <CompactStudentCard
                      studentId={id}
                      firstName={candidate.student.first_name}
                      lastName={candidate.student.second_name}
                      schoolClass={candidate.student.school_class}
                      groupName={candidate.student.group_name ?? undefined}
                      photoUrl={candidate.student.photo_url ?? null}
                      chrome="plain"
                    />
                  </span>
                  <span className="shrink-0 text-xs text-gray-600">
                    {candidate.where}
                  </span>
                </ChoiceTile>
              );
            })}
          </FramedList>
        )}
      </div>
    </Modal>
  );
}
