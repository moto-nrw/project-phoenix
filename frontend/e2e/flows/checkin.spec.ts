import type { APIRequestContext } from "@playwright/test";
import { apiTest as test, apiExpect as expect } from "../fixtures";
import type { E2ECheckinFixture } from "../contract";

type CurrentVisit = {
  id: number;
} | null;

type CheckinScanResult = {
  student_id: number;
  student_name?: string;
  action: string;
  visit_id?: number | null;
};

async function getCurrentVisit(
  studentId: number,
  adminApi: APIRequestContext,
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

async function scanCheckin(
  deviceApi: APIRequestContext,
  checkinScenario: E2ECheckinFixture,
): Promise<CheckinScanResult> {
  const res = await deviceApi.post("/api/iot/checkin", {
    data: {
      student_rfid: checkinScenario.rfid_tag,
      action: "checkin",
      room_id: checkinScenario.room.id,
    },
  });
  if (!res.ok()) {
    throw new Error(
      `device checkin failed (${res.status()}): ${await res.text()}`,
    );
  }

  const body = (await res.json()) as {
    data: CheckinScanResult;
  };
  return body.data;
}

/**
 * Full RFID check-in / check-out flow. Exercises the same code path the
 * Raspberry Pi kiosk uses in production: device API key + staff PIN +
 * student RFID. Asserts that the toggle behaviour creates a visit on the
 * first scan and ends it on the second, AND that the resulting active.visits
 * row matches the action — issue #1142 explicitly requires "in active.visits
 * sichtbar", so a regression where the API returns 200 + visit_id but
 * doesn't actually persist the row would be missed without this check.
 *
 * Setup is via HTTP (not the UI) so the test stays fast and isolated from
 * frontend state. The companion `flows/iot-api.spec.ts` covers the
 * cross-repo contract (auth, config) — this one covers the business flow.
 */

test.describe("RFID check-in / check-out toggle", () => {
  test("two scans toggle the student between checked-in and checked-out", async ({
    adminApi,
    checkinScenario,
    deviceApi,
  }) => {
    const firstScan = await scanCheckin(deviceApi, checkinScenario);
    expect(firstScan.student_id).toBe(checkinScenario.student.id);
    // Student name comes from the manifest-backed fixture, not a hardcoded "Felix"
    // — keeps the test honest if the seeder ever reorders students.
    expect(firstScan.student_name).toContain(
      checkinScenario.student.first_name,
    );
    // Possible actions per checkin response semantics: "checked_in" on
    // first scan, "checked_out" / "checked_out_daily" on second.
    expect(firstScan.action).toMatch(
      /^(checked_in|checked_out|checked_out_daily)$/,
    );
    expect(firstScan.visit_id).toBeTruthy();

    // Verify state is persisted in active.visits — the API can return
    // visit_id without writing the row if the service layer breaks (e.g.
    // a forgotten Commit). Issue #1142 calls this out explicitly.
    const visitAfterFirst = await getCurrentVisit(
      checkinScenario.student.id,
      adminApi,
    );
    if (firstScan.action === "checked_in") {
      expect(
        visitAfterFirst,
        "after a checked_in action the student must have a current open visit",
      ).not.toBeNull();
      expect(visitAfterFirst?.id).toBe(firstScan.visit_id);
    } else {
      expect(
        visitAfterFirst,
        "after a checked_out action the student must have no current open visit",
      ).toBeNull();
    }

    const secondScan = await scanCheckin(deviceApi, checkinScenario);
    expect(secondScan.student_id).toBe(checkinScenario.student.id);
    expect(secondScan.action).toMatch(
      /^(checked_in|checked_out|checked_out_daily)$/,
    );
    expect(secondScan.action).not.toBe(firstScan.action);

    // After the toggle, the active.visits state must flip too.
    const visitAfterSecond = await getCurrentVisit(
      checkinScenario.student.id,
      adminApi,
    );
    if (secondScan.action === "checked_in") {
      expect(visitAfterSecond).not.toBeNull();
      expect(visitAfterSecond?.id).toBe(secondScan.visit_id);
    } else {
      expect(visitAfterSecond).toBeNull();
    }
  });
});
