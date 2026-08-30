import { sessionFetch } from "./session-cache";
import {
  type BackendGroupHandover,
  type BackendSubstitutionOverview,
  type Substitution,
  type TeacherAvailability,
  formatDateForBackend,
  mapSubstitutionResponse,
  mapSubstitutionsResponse,
  prepareSubstitutionForBackend,
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
    const body = (await response.json()) as BackendSubstitutionOverview;
    return mapSubstitutionsResponse(body.group_handovers);
  }

  async fetchAvailableTeachers(): Promise<TeacherAvailability[]> {
    const response = await sessionFetch("/api/substitutions", {
      credentials: "include",
    });
    if (!response.ok) {
      throw new Error("Fachkräfte konnten nicht geladen werden.");
    }
    const body = (await response.json()) as BackendSubstitutionOverview;
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
    const body = (await response.json()) as BackendGroupHandover;
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
