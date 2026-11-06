import { afterEach, describe, expect, it, vi } from "vitest";
import {
  createCareContracts,
  sendCareContracts,
  resendCareContract,
  fetchCareContracts,
} from "./traeger-care-contracts-api";

const request = vi.hoisted(() => vi.fn());
vi.mock("./traeger-api", () => ({ traegerRequest: request }));
afterEach(() => request.mockReset());

describe("care contract requests", () => {
  it("creates and sends selections above the backend batch limit in bounded requests", async () => {
    const ids = Array.from({ length: 401 }, (_, i) => String(i + 1));
    request.mockImplementation(async (_path, init) => ({
      created: [],
      sent: init?.body?.contract_ids ?? [],
      skipped: [],
    }));
    await createCareContracts(
      ids.map((id) => ({ schoolId: "7", requestChildId: id })),
    );
    expect(
      request.mock.calls.map(([, init]) => init.body.contracts.length),
    ).toEqual([200, 200, 1]);
    expect(
      request.mock.calls
        .flatMap(([, init]) => init.body.contracts)
        .map((key) => String(key.request_child_id)),
    ).toEqual(ids);
    request.mockClear();
    const sent = await sendCareContracts(ids);
    expect(
      request.mock.calls.map(([, init]) => init.body.contract_ids.length),
    ).toEqual([200, 200, 1]);
    expect(sent.sent).toEqual(ids);
  });

  it("retries one contract at the explicit corrected address", async () => {
    request.mockResolvedValue({ sent: [7], skipped: [] });
    expect(await resendCareContract("7", "neu@example.org")).toEqual({
      sent: ["7"],
      skipped: [],
    });
    expect(request).toHaveBeenCalledWith("/api/traeger/contracts/send", {
      method: "POST",
      body: { contract_ids: [7], recipient_email: "neu@example.org" },
    });
  });

  it("sends filters and pagination to the list endpoint", async () => {
    request.mockResolvedValue({
      contracts: [],
      counts: { entwurf: 1100 },
      has_more: true,
    });
    const result = await fetchCareContracts({
      search: "Mia & Ben",
      status: "entwurf",
      schoolId: "7",
      offset: 1000,
    });
    const url = new URL(request.mock.calls[0]![0], "http://localhost");
    expect(Object.fromEntries(url.searchParams)).toEqual({
      search: "Mia & Ben",
      status: "entwurf",
      school_id: "7",
      offset: "1000",
    });
    expect(result.hasMore).toBe(true);
  });
});
