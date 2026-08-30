import { sessionFetch } from "./session-cache";
import {
  type BackendGroupHandover,
  type BackendAdditionalSupervisionResult,
  type RunningSupervision,
  type BackendSubstitutionOverview,
  type Substitution,
  type TeacherAvailability,
  formatDateForBackend,
  mapSubstitutionResponse,
  mapSubstitutionsResponse,
  prepareSubstitutionForBackend,
  type SubstitutionProxyEnvelope,
  unwrapSubstitutionProxyEnvelope,
} from "./substitution-helpers";

class SubstitutionService {
  async fetchSubstitutions(date?: Date): Promise<Substitution[]> {
    const params = new URLSearchParams();
    if (date) params.set("date", formatDateForBackend(date));
    const response = await sessionFetch(
      `/api/substitutions${params.size > 0 ? `?${params.toString()}` : ""}`,
      { credentials: "include" },
    );
    if (!response.ok) {
      throw new Error(`Gruppenübergaben konnten nicht geladen werden.`);
    }
    const envelope =
      (await response.json()) as SubstitutionProxyEnvelope<BackendSubstitutionOverview>;
    const body = unwrapSubstitutionProxyEnvelope(envelope);
    return mapSubstitutionsResponse(body.group_handovers);
  }

  async fetchAvailableTeachers(): Promise<TeacherAvailability[]> {
    const response = await sessionFetch("/api/substitutions", {
      credentials: "include",
    });
    if (!response.ok) {
      throw new Error("Fachkräfte konnten nicht geladen werden.");
    }
    const envelope =
      (await response.json()) as SubstitutionProxyEnvelope<BackendSubstitutionOverview>;
    const body = unwrapSubstitutionProxyEnvelope(envelope);
    return body.targets.map((staff) => {
      const [firstName = "", ...lastNameParts] = staff.full_name.split(" ");
      return {
        id: staff.id.toString(),
        firstName,
        lastName: lastNameParts.join(" "),
        inSubstitution: false,
        substitutionCount: 0,
      };
    });
  }

  async fetchRunningSupervision(
    activeGroupId: string,
  ): Promise<RunningSupervision> {
    const params = new URLSearchParams({ active_group_id: activeGroupId });
    const response = await sessionFetch(`/api/substitutions?${params}`, {
      credentials: "include",
    });
    if (!response.ok) {
      throw new Error(
        "Die Betreuung konnte nicht geladen werden. Bitte versuchen Sie es noch einmal.",
      );
    }
    const envelope =
      (await response.json()) as SubstitutionProxyEnvelope<BackendSubstitutionOverview>;
    const rows = unwrapSubstitutionProxyEnvelope(envelope).running_supervisions;
    if (!Array.isArray(rows) || rows.length !== 1 || !rows[0]) {
      throw new Error("Ungültige Antwort für die Betreuung.");
    }
    const row = rows[0];
    return {
      id: row.id.toString(),
      name: row.name,
      roomName: row.room_name,
      supervisors: row.supervisors.map((staff) => ({
        id: staff.id.toString(),
        fullName: staff.full_name,
      })),
      availableTargets: row.available_targets.map((staff) => ({
        id: staff.id.toString(),
        fullName: staff.full_name,
      })),
      isCurrentUserSupervising: row.is_current_user_supervising,
      canAssign: row.can_assign,
    };
  }

  async addSupervisor(
    activeGroupId: string,
    targetStaffId: string,
  ): Promise<{ id: string; targetName: string }> {
    const response = await sessionFetch("/api/substitutions", {
      method: "POST",
      credentials: "include",
      body: JSON.stringify({
        type: "additional_supervision",
        additional_supervision: {
          active_group_id: Number.parseInt(activeGroupId, 10),
          target_staff_id: Number.parseInt(targetStaffId, 10),
        },
      }),
    });
    if (!response.ok) {
      const body = (await response.json()) as { error?: string };
      throw new Error(
        body.error ??
          "Der Betreuer konnte nicht hinzugefügt werden. Bitte versuchen Sie es noch einmal.",
      );
    }
    const envelope =
      (await response.json()) as SubstitutionProxyEnvelope<BackendAdditionalSupervisionResult>;
    const body = unwrapSubstitutionProxyEnvelope(envelope);
    return { id: body.id.toString(), targetName: body.target.full_name };
  }

  async createSubstitution(
    groupId: string,
    substituteStaffId: string,
    startDate: string,
    endDate: string,
  ): Promise<Substitution> {
    const response = await sessionFetch("/api/substitutions", {
      method: "POST",
      credentials: "include",
      body: JSON.stringify(
        prepareSubstitutionForBackend(
          groupId,
          substituteStaffId,
          startDate,
          endDate,
        ),
      ),
    });
    if (!response.ok) {
      const body = (await response.json()) as { error?: string };
      throw new Error(
        body.error ?? "Gruppenübergabe konnte nicht erstellt werden.",
      );
    }
    const envelope =
      (await response.json()) as SubstitutionProxyEnvelope<BackendGroupHandover>;
    const body = unwrapSubstitutionProxyEnvelope(envelope);
    return mapSubstitutionResponse(body);
  }

  async deleteSubstitution(id: string): Promise<void> {
    const response = await sessionFetch("/api/substitutions/end", {
      method: "POST",
      credentials: "include",
      body: JSON.stringify({
        type: "group_handover",
        id: Number.parseInt(id, 10),
      }),
    });
    if (!response.ok) {
      const body = (await response.json()) as { error?: string };
      throw new Error(
        body.error ?? "Gruppenübergabe konnte nicht beendet werden.",
      );
    }
  }
}

export const substitutionService = new SubstitutionService();
