import { sessionFetch } from "./session-cache";
import {
  type BackendGroupHandover,
  type BackendSubstitutionOverview,
  type Substitution,
  type TeacherAvailability,
  formatDateForBackend,
  mapSubstitutionResponse,
  mapSubstitutionsResponse,
  mapScheduleSubstitutionOverview,
  type ScheduleSubstitutionOverview,
  prepareSubstitutionForBackend,
  type SubstitutionProxyEnvelope,
  unwrapSubstitutionProxyEnvelope,
} from "./substitution-helpers";
import {
  mapApplyDeviations,
  mapBulkSubstitution,
  prepareApplyDeviationsBody,
  prepareBulkSubstitutionBody,
} from "./timetable-helpers";
import type {
  ApplyDeviationsInput,
  ApplyDeviationsResponse,
  BackendApplyDeviationsResponse,
  BackendBulkSubstitutionResponse,
  BulkSubstitutionInput,
  BulkSubstitutionResponse,
} from "./timetable-types";

class SubstitutionService {
  async fetchScheduleOverview(
    from: string,
    to: string,
  ): Promise<ScheduleSubstitutionOverview> {
    const params = new URLSearchParams({ from, to });
    const response = await sessionFetch(`/api/substitutions?${params}`, {
      credentials: "include",
    });
    if (!response.ok) {
      throw new Error("Vertretungen konnten nicht geladen werden.");
    }
    const envelope =
      (await response.json()) as SubstitutionProxyEnvelope<BackendSubstitutionOverview>;
    if (envelope.data === undefined) {
      throw new Error("Ungültige Antwort für Vertretungen.");
    }
    return mapScheduleSubstitutionOverview(envelope.data);
  }

  async applyScheduleSubstitution(
    instanceId: string,
    input: ApplyDeviationsInput,
  ): Promise<ApplyDeviationsResponse> {
    if (input.cancel) {
      throw new Error("Absagen laufen nicht über eine Vertretung.");
    }
    const response = await sessionFetch("/api/substitutions", {
      method: "POST",
      credentials: "include",
      body: JSON.stringify({
        type: "schedule_substitution",
        schedule_substitution: {
          instance_id: Number(instanceId),
          ...prepareApplyDeviationsBody(input),
        },
      }),
    });
    if (!response.ok) throw await substitutionError(response);
    const envelope =
      (await response.json()) as SubstitutionProxyEnvelope<BackendApplyDeviationsResponse>;
    return mapApplyDeviations(unwrapSubstitutionProxyEnvelope(envelope));
  }

  async applyBulkSubstitution(
    input: BulkSubstitutionInput,
  ): Promise<BulkSubstitutionResponse> {
    const response = await sessionFetch("/api/substitutions", {
      method: "POST",
      credentials: "include",
      body: JSON.stringify({
        type: "schedule_substitution",
        schedule_substitution: {
          whole_days: prepareBulkSubstitutionBody(input),
        },
      }),
    });
    if (!response.ok) throw await substitutionError(response);
    const envelope =
      (await response.json()) as SubstitutionProxyEnvelope<BackendBulkSubstitutionResponse>;
    return mapBulkSubstitution(unwrapSubstitutionProxyEnvelope(envelope));
  }

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

async function substitutionError(response: Response): Promise<Error> {
  const body = (await response.json()) as { error?: string };
  return new Error(body.error ?? "Vertretung konnte nicht gespeichert werden.");
}

export const substitutionService = new SubstitutionService();
