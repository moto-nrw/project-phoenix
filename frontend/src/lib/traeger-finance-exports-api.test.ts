import { afterEach, expect, it, vi } from "vitest";
import {
  deleteCostCenter,
  fetchExportProtocol,
} from "./traeger-finance-exports-api";

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

// #4693: the protocol carries the name of the exporting account.
it("maps who exported a file by name", async () => {
  const fetchMock = vi.fn<typeof fetch>();
  vi.stubGlobal("fetch", fetchMock);
  const file = {
    id: 3,
    file_number: 3,
    format: "datev",
    run_kind: "care",
    run_id: 7,
    run_label: "Oktober 2026",
    file_name: "EXTF_3.csv",
    invoice_count: 1,
    total_cents: 100,
    exported_by_account_id: 5,
    exported_by_you: false,
    exported_at: "2026-11-09T10:00:00Z",
    submitted_at: null,
  };
  fetchMock.mockResolvedValue(
    new Response(
      JSON.stringify({
        data: {
          total: 2,
          next_offset: null,
          files: [
            { ...file, exported_by_name: "Eva Müller" },
            { ...file, id: 2, exported_by_account_id: null },
          ],
        },
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ),
  );

  const page = await fetchExportProtocol();

  expect(
    page.items.map((entry) => [entry.exportedByAccount, entry.exportedByName]),
  ).toEqual([
    [true, "Eva Müller"],
    [false, ""],
  ]);
});
