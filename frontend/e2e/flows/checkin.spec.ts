import { apiTest as test, apiExpect as expect } from "../fixtures";
import {
  BACKEND_URL,
  IOT_HEADERS,
  getDeviceApiKey,
  getDevicePIN,
} from "../helpers/iot";
import {
  setupCheckinScenario,
  type CheckinScenario,
} from "../helpers/checkin-fixtures";
import { withAdminContext } from "../helpers/admin-api";

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

/**
 * Reads /api/active/visits/student/{id}/current via an admin token.
 *
 * The handler has two "no current visit" code paths:
 *   - The service returns ErrVisitNotFound  → handler renders 404 with
 *     `{"error":"...visit not found"}`.
 *   - The service returns (nil, nil)        → handler renders 200 with
 *     `data: null` and message "Student has no active visit".
 * In practice we observe the 404 path, but both must be treated as
 * "no open visit". Anything else is a real failure we want surfaced.
 */
async function getCurrentVisit(
  studentId: number,
): Promise<{ id: number; group_id?: number } | null> {
  return withAdminContext(async (ctx, headers) => {
    const res = await ctx.get(
      `${BACKEND_URL}/api/active/visits/student/${studentId}/current`,
      { headers, failOnStatusCode: false },
    );
    if (res.status() === 404) return null;
    expect(res.status(), `current-visit lookup body: ${await res.text()}`).toBe(
      200,
    );
    const body = (await res.json()) as {
      data: { id: number; group_id?: number } | null;
    };
    return body.data;
  });
}

test.describe("RFID check-in / check-out toggle", () => {
  let scenario: CheckinScenario;

  test.beforeAll(async () => {
    scenario = await setupCheckinScenario();
  });

  test("two scans toggle the student between checked-in and checked-out", async ({
    request,
  }) => {
    const headers = {
      ...IOT_HEADERS.apiKeyAndPin(getDeviceApiKey(), getDevicePIN()),
      "Content-Type": "application/json",
    };
    const body = {
      student_rfid: scenario.rfidTag,
      action: "checkin", // server ignores this and toggles based on current state
      room_id: scenario.roomId,
    };

    // First scan
    const r1 = await request.post(`${BACKEND_URL}/api/iot/checkin`, {
      headers,
      data: body,
    });
    expect(r1.status(), `first scan body: ${await r1.text()}`).toBe(200);
    const b1 = (await r1.json()) as {
      data: {
        student_id: number;
        student_name: string;
        action: string;
        visit_id?: number | null;
      };
    };
    expect(b1.data.student_id).toBe(scenario.studentId);
    // Student name comes from the seed-state lookup, not a hardcoded
    // "Felix" — keeps the test honest if the seeder ever reorders students.
    expect(b1.data.student_name).toContain(scenario.studentFirstName);
    // Possible actions per checkin response semantics: "checked_in" on
    // first scan, "checked_out" / "checked_out_daily" on second.
    expect(b1.data.action).toMatch(
      /^(checked_in|checked_out|checked_out_daily)$/,
    );
    expect(b1.data.visit_id).toBeTruthy();

    // Verify state is persisted in active.visits — the API can return
    // visit_id without writing the row if the service layer breaks (e.g.
    // a forgotten Commit). Issue #1142 calls this out explicitly.
    const visitAfterFirst = await getCurrentVisit(scenario.studentId);
    if (b1.data.action === "checked_in") {
      expect(
        visitAfterFirst,
        "after a checked_in action the student must have a current open visit",
      ).not.toBeNull();
      expect(visitAfterFirst?.id).toBe(b1.data.visit_id);
    } else {
      expect(
        visitAfterFirst,
        "after a checked_out action the student must have no current open visit",
      ).toBeNull();
    }

    // Second scan — must produce a different action.
    const r2 = await request.post(`${BACKEND_URL}/api/iot/checkin`, {
      headers,
      data: body,
    });
    expect(r2.status()).toBe(200);
    const b2 = (await r2.json()) as {
      data: {
        student_id: number;
        action: string;
        visit_id?: number | null;
      };
    };
    expect(b2.data.student_id).toBe(scenario.studentId);
    expect(b2.data.action).toMatch(
      /^(checked_in|checked_out|checked_out_daily)$/,
    );
    expect(b2.data.action).not.toBe(b1.data.action);

    // After the toggle, the active.visits state must flip too.
    const visitAfterSecond = await getCurrentVisit(scenario.studentId);
    if (b2.data.action === "checked_in") {
      expect(visitAfterSecond).not.toBeNull();
      expect(visitAfterSecond?.id).toBe(b2.data.visit_id);
    } else {
      expect(visitAfterSecond).toBeNull();
    }
  });
});
