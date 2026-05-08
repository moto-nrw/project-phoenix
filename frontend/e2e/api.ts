import {
  expect,
  request as apiRequest,
  type APIRequestContext,
  type APIResponse,
} from "@playwright/test";
import { type TenantSession } from "./auth";
import {
  getBackendBaseURL,
  type CheckinFixture,
  type Device,
  type StudentRef,
} from "./state";

export type SickPresentStudentCard = {
  fullName: string;
};

export type CheckinScan = {
  studentId: number;
  studentName?: string;
  action: string;
  visitId?: number | null;
};

export type CurrentVisit = {
  id: number;
} | null;

export type CheckinFlow = {
  scan(): Promise<CheckinScan>;
  readCurrentVisit(): Promise<CurrentVisit>;
  expectSeededStudent(scan: CheckinScan): void;
  expectPersistedVisitMatches(scan: CheckinScan, visit: CurrentVisit): void;
};

export type IotContract = {
  getConfigWithoutAuth(): Promise<APIResponse>;
  getConfigWithInvalidApiKey(): Promise<APIResponse>;
  getConfigWithMalformedAuthorizationHeader(): Promise<APIResponse>;
  getConfigWithValidDevice(): Promise<APIResponse>;
  postCheckinWithoutPin(): Promise<APIResponse>;
  postCheckinWithWrongPin(): Promise<APIResponse>;
  postCheckinWithValidPinAndUnknownRFID(): Promise<APIResponse>;
};

function fullName(student: Pick<StudentRef, "firstName" | "lastName">): string {
  return `${student.firstName} ${student.lastName}`;
}

async function requireOK(
  response: APIResponse,
  context: string,
): Promise<void> {
  if (response.ok()) {
    return;
  }
  throw new Error(
    `${context} (${response.status()}): ${await response.text()}`,
  );
}

async function resolveAccessTokenFromSession(
  session: TenantSession,
): Promise<string> {
  // Browser auth belongs to auth.setup.ts + storageState. API fixtures
  // derive their backend Bearer token from that already-authenticated browser
  // session so the harness has one auth truth instead of separate UI/API login
  // flows that can drift.
  const frontendCtx = await apiRequest.newContext({
    baseURL: session.appRoot,
    storageState: session.storageStatePath,
  });

  try {
    const res = await frontendCtx.post("/api/auth/token");
    if (!res.ok()) {
      throw new Error(
        `session token lookup failed for ${session.email} (${res.status()}): ${await res.text()}`,
      );
    }

    const body = (await res.json()) as {
      access_token?: string;
    };

    if (!body.access_token) {
      throw new Error(
        `session token lookup returned no access token for ${session.email}`,
      );
    }

    return body.access_token;
  } finally {
    await frontendCtx.dispose();
  }
}

async function readCurrentVisit(
  adminApi: APIRequestContext,
  studentId: number,
): Promise<CurrentVisit> {
  const visitRes = await adminApi.get(
    `/api/students/${studentId}/current-visit`,
    {
      failOnStatusCode: false,
    },
  );
  if (visitRes.status() === 200) {
    const body = (await visitRes.json()) as {
      data?: {
        id: number;
      } | null;
    };
    return body.data ?? null;
  }

  throw new Error(
    `current-visit lookup failed for student ${studentId} (${visitRes.status()}): ${await visitRes.text()}`,
  );
}

export async function assertApiSessionReady(
  session: TenantSession,
): Promise<void> {
  await expect
    .poll(() => resolveAccessTokenFromSession(session), {
      message: `API session for ${session.email} should yield a backend access token`,
    })
    .toMatch(/\S+/);
}

export async function createBackendApiContext(): Promise<APIRequestContext> {
  return apiRequest.newContext({
    baseURL: getBackendBaseURL(),
  });
}

export async function createTenantApiContext(
  session: TenantSession,
): Promise<APIRequestContext> {
  const token = await resolveAccessTokenFromSession(session);
  return apiRequest.newContext({
    baseURL: getBackendBaseURL(),
    extraHTTPHeaders: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
  });
}

export async function createDeviceApiContext(
  device: Pick<Device, "apiKey" | "pin">,
): Promise<APIRequestContext> {
  return apiRequest.newContext({
    baseURL: getBackendBaseURL(),
    extraHTTPHeaders: {
      Authorization: `Bearer ${device.apiKey}`,
      "X-Staff-PIN": device.pin,
      "Content-Type": "application/json",
    },
  });
}

export function createSickPresentStudentCard(
  student: StudentRef,
): SickPresentStudentCard {
  return {
    fullName: fullName(student),
  };
}

export function createCheckinFlow(
  scenario: CheckinFixture,
  deviceApi: APIRequestContext,
  adminApi: APIRequestContext,
): CheckinFlow {
  return {
    async scan(): Promise<CheckinScan> {
      const response = await deviceApi.post("/api/iot/checkin", {
        data: {
          student_rfid: scenario.rfidTag,
          action: "checkin",
          room_id: scenario.room.id,
        },
      });
      await requireOK(response, "device checkin");

      const body = (await response.json()) as {
        data: {
          student_id: number;
          student_name?: string;
          action: string;
          visit_id?: number | null;
        };
      };
      return {
        studentId: body.data.student_id,
        studentName: body.data.student_name,
        action: body.data.action,
        visitId: body.data.visit_id,
      };
    },
    async readCurrentVisit(): Promise<CurrentVisit> {
      return readCurrentVisit(adminApi, scenario.student.id);
    },
    expectSeededStudent(scan: CheckinScan): void {
      expect(scan.studentId).toBe(scenario.student.id);
      expect(scan.studentName).toContain(scenario.student.firstName);
    },
    expectPersistedVisitMatches(scan: CheckinScan, visit: CurrentVisit): void {
      if (scan.action === "checked_in") {
        expect(
          visit,
          "after a checked_in action the student must have a current open visit",
        ).not.toBeNull();
        expect(visit?.id).toBe(scan.visitId);
        return;
      }

      expect(
        visit,
        "after a checked_out action the student must have no current open visit",
      ).toBeNull();
    },
  };
}

export function createIotContract(
  backendApi: APIRequestContext,
  deviceApi: APIRequestContext,
  device: Device,
): IotContract {
  return {
    getConfigWithoutAuth(): Promise<APIResponse> {
      return backendApi.get("/api/iot/config");
    },
    getConfigWithInvalidApiKey(): Promise<APIResponse> {
      return backendApi.get("/api/iot/config", {
        headers: { Authorization: "Bearer dev_not_a_real_key" },
      });
    },
    getConfigWithMalformedAuthorizationHeader(): Promise<APIResponse> {
      return backendApi.get("/api/iot/config", {
        headers: { Authorization: device.apiKey },
      });
    },
    getConfigWithValidDevice(): Promise<APIResponse> {
      return deviceApi.get("/api/iot/config");
    },
    postCheckinWithoutPin(): Promise<APIResponse> {
      return backendApi.post("/api/iot/checkin", {
        headers: {
          Authorization: `Bearer ${device.apiKey}`,
        },
        data: { student_rfid: "TEST-RFID-001" },
      });
    },
    postCheckinWithWrongPin(): Promise<APIResponse> {
      return backendApi.post("/api/iot/checkin", {
        headers: {
          Authorization: `Bearer ${device.apiKey}`,
          "X-Staff-PIN": "0000",
        },
        data: { student_rfid: "TEST-RFID-001" },
      });
    },
    postCheckinWithValidPinAndUnknownRFID(): Promise<APIResponse> {
      return deviceApi.post("/api/iot/checkin", {
        data: { student_rfid: "TEST-DOES-NOT-EXIST" },
      });
    },
  };
}
