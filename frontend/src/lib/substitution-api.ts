import { ApiError, apiErrorFromResponse } from "./api-error";
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
  mapRunningSupervision,
  mapSubstitutionOverview,
  mapScheduleSubstitutionOverview,
  type ScheduleSubstitutionOverview,
  type SubstitutionOverview,
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
  async fetchOverview(): Promise<SubstitutionOverview> {
    const response = await sessionFetch("/api/substitutions", {
      credentials: "include",
    });
    if (!response.ok) {
      throw await apiErrorFromResponse(
        response,
        "substitution overview failed",
      );
    }
    const envelope =
      (await response.json()) as SubstitutionProxyEnvelope<BackendSubstitutionOverview>;
    return mapSubstitutionOverview(unwrapSubstitutionProxyEnvelope(envelope));
  }

  async fetchScheduleOverview(
    from: string,
    to: string,
  ): Promise<ScheduleSubstitutionOverview> {
    const params = new URLSearchParams({ from, to });
    const response = await sessionFetch(`/api/substitutions?${params}`, {
      credentials: "include",
    });
    if (!response.ok) {
      throw await apiErrorFromResponse(
        response,
        "schedule substitution overview failed",
      );
    }
    const envelope =
      (await response.json()) as SubstitutionProxyEnvelope<BackendSubstitutionOverview>;
    if (envelope.data === undefined) {
      throw invalidResponseError("schedule substitution overview");
    }
    return mapScheduleSubstitutionOverview(envelope.data);
  }

  async applyScheduleSubstitution(
    instanceId: string,
    input: ApplyDeviationsInput,
  ): Promise<ApplyDeviationsResponse> {
    if (input.cancel) {
      // Programmierfehler, kein Nutzerfehler: Absagen haben einen eigenen
      // Weg. Der Absturztext des Anzeigewegs ist dafür die richtige Folge.
      throw new Error("cancellations do not go through a substitution");
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
      throw await apiErrorFromResponse(response, "group handovers failed");
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
      throw await apiErrorFromResponse(response, "available staff failed");
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
      throw await apiErrorFromResponse(response, "running supervision failed");
    }
    const envelope =
      (await response.json()) as SubstitutionProxyEnvelope<BackendSubstitutionOverview>;
    const rows = unwrapSubstitutionProxyEnvelope(envelope).running_supervisions;
    if (!Array.isArray(rows) || rows.length !== 1 || !rows[0]) {
      throw invalidResponseError("running supervision");
    }
    return mapRunningSupervision(rows[0]);
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
          active_group_id: activeGroupId,
          target_staff_id: targetStaffId,
        },
      }),
    });
    if (!response.ok) {
      throw await apiErrorFromResponse(
        response,
        "additional supervision failed",
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
      throw await apiErrorFromResponse(response, "group handover failed");
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
        id,
      }),
    });
    if (!response.ok) {
      throw await apiErrorFromResponse(response, "end group handover failed");
    }
  }
}

async function substitutionError(response: Response): Promise<ApiError> {
  return apiErrorFromResponse(response, "schedule substitution failed");
}

/** A 2xx answer without the expected shape counts as a server failure. */
function invalidResponseError(what: string): ApiError {
  return new ApiError(`invalid ${what} response`, 502, {
    code: "general.server",
  });
}

export const substitutionService = new SubstitutionService();
