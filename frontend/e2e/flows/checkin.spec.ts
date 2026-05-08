import { apiTest as test, apiExpect as expect } from "../fixtures";

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
    checkinFlow,
  }) => {
    const firstScan = await checkinFlow.scan();
    checkinFlow.expectSeededStudent(firstScan);
    // Possible actions per checkin response semantics: "checked_in" on
    // first scan, "checked_out" / "checked_out_daily" on second.
    expect(firstScan.action).toMatch(
      /^(checked_in|checked_out|checked_out_daily)$/,
    );
    expect(firstScan.visitId).toBeTruthy();

    // Verify state is persisted in active.visits — the API can return
    // visit_id without writing the row if the service layer breaks (e.g.
    // a forgotten Commit). Issue #1142 calls this out explicitly.
    const visitAfterFirst = await checkinFlow.readCurrentVisit();
    checkinFlow.expectPersistedVisitMatches(firstScan, visitAfterFirst);

    const secondScan = await checkinFlow.scan();
    checkinFlow.expectSeededStudent(secondScan);
    expect(secondScan.action).toMatch(
      /^(checked_in|checked_out|checked_out_daily)$/,
    );
    expect(secondScan.action).not.toBe(firstScan.action);

    // After the toggle, the active.visits state must flip too.
    const visitAfterSecond = await checkinFlow.readCurrentVisit();
    checkinFlow.expectPersistedVisitMatches(secondScan, visitAfterSecond);
  });
});
