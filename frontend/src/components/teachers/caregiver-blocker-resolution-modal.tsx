"use client";

import { useCallback, useEffect, useState } from "react";
import { FormModal } from "~/components/ui/form-modal";
import { Alert } from "~/components/ui/alert";
import { CustomSelect } from "~/components/ui/custom-select";
import { InfoSection } from "~/components/ui/detail-modal-components";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { useToast } from "~/contexts/ToastContext";
import type {
  BlockerActivity,
  BlockerGroup,
  BlockerSubstitution,
  BlockerSupervision,
  CaregiverCapabilityState,
} from "~/lib/caregiver-capability-api";
import {
  groupTransferService,
  type StaffWithRole,
} from "~/lib/group-transfer-api";
import { createLogger } from "~/lib/logger";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";

const logger = createLogger({ component: "CaregiverBlockerResolution" });

interface CaregiverBlockerResolutionModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly state: CaregiverCapabilityState;
  readonly onResolved: () => void;
}

async function endActiveSupervision(id: string): Promise<void> {
  const response = await fetch(
    `/api/active/supervisors/${encodeURIComponent(id)}/end`,
    {
      method: "POST",
      credentials: "include",
    },
  );
  if (!response.ok) {
    throw new Error(
      `Gruppenaufsicht konnte nicht beendet werden (${response.status})`,
    );
  }
}

async function deleteSubstitution(id: string): Promise<void> {
  const response = await fetch(`/api/substitutions/${encodeURIComponent(id)}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!response.ok) {
    throw new Error(
      `Vertretung konnte nicht entfernt werden (${response.status})`,
    );
  }
}

async function removeActivitySupervisor(
  activityId: string,
  supervisorId: string,
  replacementStaffId?: string,
): Promise<void> {
  const searchParams = new URLSearchParams();
  if (replacementStaffId) {
    searchParams.set("replacement_staff_id", replacementStaffId);
  }

  const response = await fetch(
    `/api/activities/${encodeURIComponent(activityId)}/supervisors/${encodeURIComponent(supervisorId)}${searchParams.size > 0 ? `?${searchParams.toString()}` : ""}`,
    { method: "DELETE", credentials: "include" },
  );
  if (!response.ok) {
    let errorMessage = replacementStaffId
      ? `Aktivitätsleitung konnte nicht übertragen werden (${response.status})`
      : `Aktivitätsleitung konnte nicht entfernt werden (${response.status})`;
    let errorCode: string | undefined;

    try {
      const payload = (await response.json()) as {
        error?: string;
        message?: string;
        code?: string;
      };
      errorMessage = payload.message ?? payload.error ?? errorMessage;
      errorCode = payload.code;
    } catch {
      // Fall back to the generic status-based message when the proxy did not
      // return structured JSON.
    }

    const error = new Error(errorMessage) as Error & { code?: string };
    error.code = errorCode;
    throw error;
  }
}

async function updateGroupTeachers(
  groupId: string,
  groupName: string,
  teacherIds: number[],
): Promise<void> {
  const response = await fetch(`/api/groups/${encodeURIComponent(groupId)}`, {
    method: "PUT",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name: groupName, teacher_ids: teacherIds }),
  });
  if (!response.ok) {
    throw new Error(
      `Gruppenleitung konnte nicht aktualisiert werden (${response.status})`,
    );
  }
}

function buildUpdatedTeacherIds(
  group: BlockerGroup,
  replacementTeacherId?: string,
): number[] {
  const currentTeacherIds =
    group.teacherIds.length > 0 ? group.teacherIds : [group.teacherId];
  const nextTeacherIds = currentTeacherIds.filter(
    (id) => id !== group.teacherId,
  );

  if (replacementTeacherId && !nextTeacherIds.includes(replacementTeacherId)) {
    nextTeacherIds.push(replacementTeacherId);
  }

  return nextTeacherIds.map((teacherId) => {
    const parsedTeacherId = Number.parseInt(teacherId, 10);
    if (Number.isNaN(parsedTeacherId)) {
      throw new Error(
        `Gruppenleitung für "${group.groupName}" enthält eine ungültige Lehrkraft-ID.`,
      );
    }
    return parsedTeacherId;
  });
}

export function CaregiverBlockerResolutionModal({
  isOpen,
  onClose,
  state,
  onResolved,
}: CaregiverBlockerResolutionModalProps) {
  const { success: toastSuccess } = useToast();
  const [availableStaff, setAvailableStaff] = useState<StaffWithRole[]>([]);
  const [loadingStaff, setLoadingStaff] = useState(false);

  const [supervisions, setSupervisions] = useState<BlockerSupervision[]>([]);
  const [substitutions, setSubstitutions] = useState<BlockerSubstitution[]>([]);
  const [activities, setActivities] = useState<BlockerActivity[]>([]);
  const [groups, setGroups] = useState<BlockerGroup[]>([]);

  const [activityReplacements, setActivityReplacements] = useState<
    Record<string, string>
  >({});
  const [groupReplacements, setGroupReplacements] = useState<
    Record<string, string>
  >({});

  const [processing, setProcessing] = useState<Record<string, boolean>>({});
  const [errorMessage, setErrorMessage] = useState("");

  const totalRemaining =
    supervisions.length +
    substitutions.length +
    activities.length +
    groups.length;

  const loadStaff = useCallback(async () => {
    try {
      setLoadingStaff(true);
      const staff = await groupTransferService.getAllAvailableStaff();
      setAvailableStaff(staff.filter((s) => s.id !== state.staffId));
    } catch (error) {
      logger.error("failed to load available staff", {
        error: error instanceof Error ? error.message : String(error),
      });
    } finally {
      setLoadingStaff(false);
    }
  }, [state.staffId]);

  useEffect(() => {
    if (!isOpen) return;
    setSupervisions(state.activeSupervisions);
    setSubstitutions(state.activeSubstitutions);
    setActivities(state.activitySupervisions);
    setGroups(state.groupAssignments);
    setActivityReplacements({});
    setGroupReplacements({});
    setErrorMessage("");
    void loadStaff();
  }, [isOpen, state, loadStaff]);

  function setItemProcessing(key: string, value: boolean) {
    setProcessing((prev) => ({ ...prev, [key]: value }));
  }

  async function handleEndSupervision(item: BlockerSupervision) {
    const key = `sup-${item.id}`;
    try {
      setItemProcessing(key, true);
      setErrorMessage("");
      await endActiveSupervision(item.id);
      setSupervisions((prev) => prev.filter((s) => s.id !== item.id));
      toastSuccess(`Gruppenaufsicht für "${item.groupName}" beendet.`);
    } catch (error) {
      logger.error("failed to end supervision", {
        id: item.id,
        error: String(error),
      });
      setErrorMessage(
        error instanceof Error
          ? error.message
          : "Fehler beim Beenden der Aufsicht.",
      );
    } finally {
      setItemProcessing(key, false);
    }
  }

  async function handleEndSubstitution(item: BlockerSubstitution) {
    const key = `sub-${item.id}`;
    try {
      setItemProcessing(key, true);
      setErrorMessage("");
      await deleteSubstitution(item.id);
      setSubstitutions((prev) => prev.filter((s) => s.id !== item.id));
      toastSuccess(`Vertretung für "${item.groupName}" entfernt.`);
    } catch (error) {
      logger.error("failed to end substitution", {
        id: item.id,
        error: String(error),
      });
      setErrorMessage(
        error instanceof Error
          ? error.message
          : "Fehler beim Entfernen der Vertretung.",
      );
    } finally {
      setItemProcessing(key, false);
    }
  }

  async function handleResolveActivity(item: BlockerActivity) {
    const replacementStaffId = activityReplacements[item.id];
    const key = `act-${item.id}`;
    try {
      setItemProcessing(key, true);
      setErrorMessage("");
      await removeActivitySupervisor(
        item.activityId,
        item.id,
        replacementStaffId,
      );
      setActivities((prev) => prev.filter((a) => a.id !== item.id));
      toastSuccess(
        replacementStaffId
          ? `Aktivitätsleitung für "${item.activityName}" übertragen.`
          : `Aktivitätsleitung für "${item.activityName}" entfernt.`,
      );
    } catch (error) {
      const errorCode =
        error instanceof Error && "code" in error
          ? (error as Error & { code?: string }).code
          : undefined;
      logger.error("failed to resolve activity", {
        id: item.id,
        error: String(error),
        errorCode,
      });
      setErrorMessage(
        errorCode === "ONLY_SUPERVISOR_REPLACEMENT_REQUIRED"
          ? `"${item.activityName}": Einzige Leitung — bitte Ersatzkraft auswählen.`
          : error instanceof Error
            ? error.message
            : "Fehler bei der Aktivitätsleitung.",
      );
    } finally {
      setItemProcessing(key, false);
    }
  }

  async function handleResolveGroup(item: BlockerGroup) {
    const replacementStaffId = groupReplacements[item.id];
    const key = `grp-${item.id}`;
    try {
      setItemProcessing(key, true);
      setErrorMessage("");
      const replacementTeacherId = replacementStaffId
        ? availableStaff.find((staff) => staff.id === replacementStaffId)
            ?.teacherId
        : undefined;
      if (replacementStaffId && !replacementTeacherId) {
        throw new Error(
          "Die ausgewählte Ersatzkraft kann keiner Stammgruppe zugeordnet werden.",
        );
      }
      const newTeacherIds = buildUpdatedTeacherIds(item, replacementTeacherId);
      await updateGroupTeachers(item.groupId, item.groupName, newTeacherIds);
      setGroups((prev) => prev.filter((g) => g.id !== item.id));
      toastSuccess(
        replacementStaffId
          ? `Gruppenleitung für "${item.groupName}" übertragen.`
          : `Gruppenleitung für "${item.groupName}" entfernt.`,
      );
    } catch (error) {
      logger.error("failed to resolve group", {
        id: item.id,
        error: String(error),
      });
      setErrorMessage(
        error instanceof Error
          ? error.message
          : "Fehler bei der Gruppenleitung.",
      );
    } finally {
      setItemProcessing(key, false);
    }
  }

  function handleDone() {
    onResolved();
    onClose();
  }

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title="Zuordnungen auflösen"
      size="xl"
      footer={
        <>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
          >
            Abbrechen
          </button>
          {totalRemaining === 0 ? (
            <button
              type="button"
              onClick={handleDone}
              className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800"
            >
              Fertig
            </button>
          ) : null}
        </>
      }
    >
      <div className="space-y-4">
        {totalRemaining === 0 ? (
          <Alert
            type="success"
            message="Alle Zuordnungen wurden aufgelöst. Sie können nun die Betreuung deaktivieren."
          />
        ) : (
          <p className="text-sm text-gray-600">
            Lösen Sie die folgenden Zuordnungen auf, um die Betreuung
            deaktivieren zu können. Sie können Zuordnungen entfernen oder an
            andere Betreuungskräfte übertragen.
          </p>
        )}

        {/* Active supervisions */}
        {supervisions.length > 0 ? (
          <InfoSection
            title={`Aktive Gruppenaufsichten (${supervisions.length})`}
            icon={
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.supervision.icon}
                tone={MOTO_CONCEPTS.supervision.tone}
                size={18}
              />
            }
            accentColor="purple"
          >
            <div className="space-y-2">
              {supervisions.map((item) => {
                const key = `sup-${item.id}`;
                return (
                  <div
                    key={item.id}
                    className="flex items-center justify-between rounded-lg border border-gray-100 bg-white px-3 py-2"
                  >
                    <div>
                      <span className="text-sm font-medium text-gray-900">
                        {item.groupName}
                      </span>
                      <span className="ml-2 text-xs text-gray-500">
                        seit {item.startDate}
                      </span>
                    </div>
                    <button
                      type="button"
                      onClick={() => void handleEndSupervision(item)}
                      disabled={processing[key]}
                      className="border-moto-red/20 text-moto-red-strong hover:bg-moto-red-soft rounded-md border px-3 py-1 text-xs font-medium transition-colors disabled:opacity-50"
                    >
                      {processing[key] ? "..." : "Beenden"}
                    </button>
                  </div>
                );
              })}
            </div>
          </InfoSection>
        ) : null}

        {/* Active substitutions */}
        {substitutions.length > 0 ? (
          <InfoSection
            title={`Aktive Vertretungen (${substitutions.length})`}
            icon={
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.substitution.icon}
                tone={MOTO_CONCEPTS.substitution.tone}
                size={18}
              />
            }
            accentColor="indigo"
          >
            <div className="space-y-2">
              {substitutions.map((item) => {
                const key = `sub-${item.id}`;
                return (
                  <div
                    key={item.id}
                    className="flex items-center justify-between rounded-lg border border-gray-100 bg-white px-3 py-2"
                  >
                    <div>
                      <span className="text-sm font-medium text-gray-900">
                        {item.groupName}
                      </span>
                      <span className="ml-2 text-xs text-gray-500">
                        {item.role === "substitute"
                          ? "Vertretung"
                          : "Stammkraft"}{" "}
                        ({item.startDate} — {item.endDate})
                      </span>
                    </div>
                    <button
                      type="button"
                      onClick={() => void handleEndSubstitution(item)}
                      disabled={processing[key]}
                      className="border-moto-red/20 text-moto-red-strong hover:bg-moto-red-soft rounded-md border px-3 py-1 text-xs font-medium transition-colors disabled:opacity-50"
                    >
                      {processing[key] ? "..." : "Entfernen"}
                    </button>
                  </div>
                );
              })}
            </div>
          </InfoSection>
        ) : null}

        {/* Activity supervisions */}
        {activities.length > 0 ? (
          <InfoSection
            title={`Aktivitätsleitungen (${activities.length})`}
            icon={
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.activities.icon}
                tone={MOTO_CONCEPTS.activities.tone}
                size={18}
              />
            }
            accentColor="orange"
          >
            <div className="space-y-2">
              {activities.map((item) => {
                const key = `act-${item.id}`;
                return (
                  <div
                    key={item.id}
                    className="flex flex-col gap-2 rounded-lg border border-gray-100 bg-white px-3 py-2 sm:flex-row sm:items-center sm:justify-between"
                  >
                    <div className="min-w-0">
                      <span className="text-sm font-medium text-gray-900">
                        {item.activityName}
                      </span>
                      {item.isPrimary ? (
                        <span className="bg-moto-orange/20 text-moto-orange-hover ml-2 inline-flex rounded-full px-2 py-0.5 text-xs font-medium">
                          Hauptleitung
                        </span>
                      ) : null}
                    </div>
                    <div className="flex items-center gap-2">
                      <CustomSelect
                        ariaLabel={`Ersatz für ${item.activityName}`}
                        value={activityReplacements[item.id] ?? ""}
                        onChange={(next) =>
                          setActivityReplacements((prev) => ({
                            ...prev,
                            [item.id]: next,
                          }))
                        }
                        options={[
                          { value: "", label: "Ohne Ersatz entfernen" },
                          ...availableStaff.map((staff) => ({
                            value: staff.id,
                            label: staff.fullName,
                          })),
                        ]}
                        disabled={loadingStaff || processing[key]}
                      />
                      <button
                        type="button"
                        onClick={() => void handleResolveActivity(item)}
                        disabled={processing[key]}
                        className="border-moto-red/20 text-moto-red-strong hover:bg-moto-red-soft rounded-md border px-3 py-1 text-xs font-medium whitespace-nowrap transition-colors disabled:opacity-50"
                      >
                        {processing[key]
                          ? "..."
                          : activityReplacements[item.id]
                            ? "Übertragen"
                            : "Entfernen"}
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          </InfoSection>
        ) : null}

        {/* Group-teacher assignments */}
        {groups.length > 0 ? (
          <InfoSection
            title={`Stammgruppen-Zuordnungen (${groups.length})`}
            icon={
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.groups.icon}
                tone={MOTO_CONCEPTS.groups.tone}
                size={18}
              />
            }
            accentColor="green"
          >
            <div className="space-y-2">
              {groups.map((item) => {
                const key = `grp-${item.id}`;
                return (
                  <div
                    key={item.id}
                    className="flex flex-col gap-2 rounded-lg border border-gray-100 bg-white px-3 py-2 sm:flex-row sm:items-center sm:justify-between"
                  >
                    <span className="text-sm font-medium text-gray-900">
                      {item.groupName}
                    </span>
                    <div className="flex items-center gap-2">
                      <CustomSelect
                        ariaLabel={`Ersatz für ${item.groupName}`}
                        value={groupReplacements[item.id] ?? ""}
                        onChange={(next) =>
                          setGroupReplacements((prev) => ({
                            ...prev,
                            [item.id]: next,
                          }))
                        }
                        options={[
                          { value: "", label: "Ohne Ersatz entfernen" },
                          ...availableStaff.map((staff) => ({
                            value: staff.id,
                            label: staff.fullName,
                          })),
                        ]}
                        disabled={loadingStaff || processing[key]}
                      />
                      <button
                        type="button"
                        onClick={() => void handleResolveGroup(item)}
                        disabled={processing[key]}
                        className="border-moto-red/20 text-moto-red-strong hover:bg-moto-red-soft rounded-md border px-3 py-1 text-xs font-medium whitespace-nowrap transition-colors disabled:opacity-50"
                      >
                        {processing[key]
                          ? "..."
                          : groupReplacements[item.id]
                            ? "Übertragen"
                            : "Entfernen"}
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          </InfoSection>
        ) : null}

        <Alert type="error" message={errorMessage} />
      </div>
    </FormModal>
  );
}
