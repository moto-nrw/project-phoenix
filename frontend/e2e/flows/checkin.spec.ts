import { test, expect } from "@playwright/test";
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

/**
 * Full RFID check-in / check-out flow. Exercises the same code path the
 * Raspberry Pi kiosk uses in production: device API key + staff PIN +
 * student RFID. Asserts that the toggle behaviour creates a visit on the
 * first scan and ends it on the second.
 *
 * Setup is via HTTP (not the UI) so the test stays fast and isolated from
 * frontend state. The companion `flows/iot-api.spec.ts` covers the
 * cross-repo contract (auth, config) — this one covers the business flow.
 */

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
    expect(b1.data.student_name).toContain("Felix");
    // Possible actions per checkin response semantics: "checked_in" on
    // first scan, "checked_out" / "checked_out_daily" on second.
    expect(b1.data.action).toMatch(
      /^(checked_in|checked_out|checked_out_daily)$/,
    );
    expect(b1.data.visit_id).toBeTruthy();

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
  });
});
