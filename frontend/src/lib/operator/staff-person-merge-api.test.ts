import { beforeEach, describe, expect, it, vi } from "vitest";
const { fetchOperator } = vi.hoisted(() => ({ fetchOperator: vi.fn() }));
vi.mock("./api-helpers", () => ({ operatorFetch: fetchOperator }));
import {
  executeStaffPersonMerges,
  previewStaffPersonMerges,
} from "./staff-person-merge-api";

beforeEach(() => vi.clearAllMocks());
describe("staff person repair API", () => {
  it("preserves bigint identity values through the read and confirmation request", async () => {
    fetchOperator.mockResolvedValueOnce([
      {
        keeper: {
          person: {
            id: "9007199254740993",
            first_name: "Anna",
            last_name: "Beispiel",
            account_id: "9007199254740995",
            created_at: "2026-04-14T12:00:00Z",
          },
          employment: {
            id: "9007199254740997",
            created_at: "2026-04-14T12:00:00Z",
            deleted_at: null,
          },
        },
        donor: {
          person: {
            id: "9007199254740999",
            first_name: "Anna",
            last_name: "Beispiel",
            created_at: "2026-09-01T12:00:00Z",
          },
          employment: {
            id: "9007199254741001",
            created_at: "2026-09-01T12:00:00Z",
            deleted_at: "2026-09-02T12:00:00Z",
          },
        },
        revision: "reviewed-case",
      },
    ]);
    const cases = await previewStaffPersonMerges("6");
    expect(cases[0]?.keeper.accountId).toBe("9007199254740995");
    expect(cases[0]?.donor.staffId).toBe("9007199254741001");
    fetchOperator.mockResolvedValueOnce(undefined);
    await executeStaffPersonMerges("6", cases, true);
    expect(fetchOperator).toHaveBeenLastCalledWith(
      "/api/operator/provisioning/staff-person-merges/6",
      {
        method: "POST",
        body: {
          school_confirmed: true,
          cases: [
            {
              donor_id: "9007199254740999",
              keeper_id: "9007199254740993",
              revision: "reviewed-case",
            },
          ],
        },
      },
    );
  });
});
