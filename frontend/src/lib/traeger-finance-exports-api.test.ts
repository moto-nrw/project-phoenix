import { afterEach, expect, it, vi } from "vitest";
import { deleteCostCenter } from "./traeger-finance-exports-api";

afterEach(() => vi.unstubAllGlobals());

it("reloads booking accounts after the BFF deletes a cost center with 204", async () => {
  const fetchMock = vi.fn<typeof fetch>();
  vi.stubGlobal("fetch", fetchMock);
  fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
  fetchMock.mockResolvedValueOnce(
    new Response(
      JSON.stringify({
        data: {
          standard_account: "4400",
          cost_center_required: false,
          cost_centers: [
            {
              id: 8,
              scope: "school",
              scope_id: 2,
              valid_from: "2026-09-01",
              code: "K8",
            },
          ],
        },
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ),
  );

  const accounts = await deleteCostCenter("7");

  expect(fetchMock).toHaveBeenNthCalledWith(
    1,
    "/api/traeger/finance-exports/booking-accounts/cost-centers/7",
    expect.objectContaining({ method: "DELETE" }),
  );
  expect(fetchMock).toHaveBeenNthCalledWith(
    2,
    "/api/traeger/finance-exports/booking-accounts",
    expect.anything(),
  );
  expect(accounts.standardAccount).toBe("4400");
  expect(accounts.costCenters.map((center) => center.id)).toEqual(["8"]);
});
