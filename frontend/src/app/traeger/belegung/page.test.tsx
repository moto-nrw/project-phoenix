import { SWRConfig, useSWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { resetSWRScope } from "~/lib/swr/scope";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { TraegerOccupancy } from "~/lib/traeger-occupancy-api";

const { mockFetchOccupancy, mockDownload, mockDownloadBlob, mockUseSession } =
  vi.hoisted(() => ({
    mockFetchOccupancy: vi.fn(),
    mockDownload: vi.fn(),
    mockDownloadBlob: vi.fn(),
    mockUseSession: vi.fn(),
  }));

vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path,
}));
vi.mock("next-auth/react", () => ({ useSession: mockUseSession }));
vi.mock("next/navigation", () => ({
  usePathname: () => "/belegung",
  useRouter: () => ({ push: vi.fn() }),
}));
vi.mock("~/lib/traeger-occupancy-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/traeger-occupancy-api")>()),
  fetchTraegerOccupancy: mockFetchOccupancy,
  downloadTraegerOccupancy: mockDownload,
}));
vi.mock("~/lib/file-download", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/file-download")>()),
  downloadBlob: mockDownloadBlob,
}));
vi.mock("~/env.client", () => ({
  clientEnv: {
    NEXT_PUBLIC_TENANT_DOMAIN: "moto-app.de",
    NEXT_PUBLIC_TRAEGER_HOSTNAME: "traeger.moto-app.de",
  },
}));

import TraegerOccupancyPage from "./page";

const OCCUPANCY: TraegerOccupancy = {
  day: "2026-09-09",
  asOf: "2026-09-09T10:00:00Z",
  schools: [
    {
      id: "2",
      name: "OGS Am Bach",
      slug: "am-bach",
      subdomain: "am-bach",
      city: "Bissendorf",
      children: 10,
      withoutGroup: 2,
      groups: [
        { id: "11", name: "Bären", children: 5 },
        { id: "12", name: "Eulen", children: 3 },
      ],
      withoutCareForm: 1,
      careForms: [
        { id: "21", name: "OGS bis 16 Uhr", children: 8, capacity: 8 },
        { id: "22", name: "Frühbetreuung", children: 2, capacity: null },
      ],
    },
    {
      id: "3",
      name: "OGS Lindenweg",
      slug: "lindenweg",
      subdomain: "lindenweg",
      city: "Melle",
      children: 6,
      withoutGroup: 0,
      groups: [{ id: "13", name: "Füchse", children: 6 }],
      withoutCareForm: 0,
      careForms: [
        { id: "23", name: "OGS bis 16 Uhr", children: 6, capacity: 10 },
      ],
    },
  ],
};

let revalidateAll: () => Promise<unknown> = async () => undefined;

/** Hands the test a way to refetch every key, as a focus or interval would. */
function Revalidator() {
  const { mutate } = useSWRConfig();
  revalidateAll = () => mutate(() => true);
  return null;
}

function renderPage() {
  return render(
    <SWRConfig
      value={{
        provider: () => new Map(),
        dedupingInterval: 0,
      }}
    >
      <ToastProvider>
        <Revalidator />
        <TraegerOccupancyPage />
      </ToastProvider>
    </SWRConfig>,
  );
}

function openTab(name: string) {
  const tab = screen.getByRole("tab", { name });
  fireEvent.mouseDown(tab, { button: 0 });
  fireEvent.click(tab);
}

function rowOf(table: HTMLElement, text: string): HTMLElement {
  return within(table).getAllByText(text)[0]?.closest("tr") as HTMLElement;
}

describe("TraegerOccupancyPage", () => {
  beforeEach(() => {
    // A refusal holds per key for the whole tab; start every test clean.
    resetSWRScope({ revalidate: false });
    mockFetchOccupancy.mockReset();
    mockDownload.mockReset();
    mockDownloadBlob.mockReset();
    mockUseSession.mockReturnValue({
      data: {
        user: {
          id: "A",
          scope: "org",
          orgId: 7,
          permissions: ["traeger_overview:read"],
        },
      },
      status: "authenticated",
    });
  });

  it("sums every school and lists each school with its shares", async () => {
    mockFetchOccupancy.mockResolvedValue(OCCUPANCY);
    renderPage();

    expect(await screen.findByText("14 von 16")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Belegung" })).toBeVisible();
    expect(screen.getByText("16")).toBeInTheDocument();
    expect(screen.getByText("15 von 16")).toBeInTheDocument();
    expect(screen.getByText("09.09.2026, Stand 12:00 Uhr")).toBeInTheDocument();

    const table = screen.getByRole("table");
    const bach = within(table)
      .getByRole("link", { name: "OGS Am Bach" })
      .closest("tr") as HTMLElement;
    expect(within(bach).getByText("10")).toBeInTheDocument();
    expect(within(bach).getByText("8")).toBeInTheDocument();
    expect(within(bach).getByText("9")).toBeInTheDocument();
    expect(
      within(table).getByRole("link", { name: "OGS Am Bach" }),
    ).toHaveAttribute("href", "/traeger/schulen/2");
  });

  it("lists every group with the children without a group", async () => {
    mockFetchOccupancy.mockResolvedValue(OCCUPANCY);
    renderPage();
    await screen.findByText("14 von 16");

    openTab("Gruppen");

    const table = await screen.findByRole("table");
    expect(within(rowOf(table, "Bären")).getByText("5")).toBeInTheDocument();
    expect(
      within(rowOf(table, "Ohne Gruppe")).getByText("2"),
    ).toBeInTheDocument();
    expect(within(rowOf(table, "Füchse")).getByText("6")).toBeInTheDocument();
  });

  it("shows the places of each care form and sums offers of the same name", async () => {
    mockFetchOccupancy.mockResolvedValue(OCCUPANCY);
    renderPage();
    await screen.findByText("14 von 16");

    openTab("Betreuungsformen");

    const [sums, perSchool] = (await screen.findAllByRole(
      "table",
    )) as HTMLElement[];
    const allOgs = rowOf(sums as HTMLElement, "OGS bis 16 Uhr");
    expect(
      within(allOgs).getByText("von 18", { exact: false }),
    ).toBeInTheDocument();
    expect(within(allOgs).getByText("4 frei")).toBeInTheDocument();
    // Sorted by school: the first „OGS bis 16 Uhr“ is the one of Am Bach.
    const bachOgs = rowOf(perSchool as HTMLElement, "OGS bis 16 Uhr");
    expect(within(bachOgs).getByText("Voll")).toBeInTheDocument();
    expect(
      within(perSchool as HTMLElement).getAllByText("Keine Grenze").length,
    ).toBeGreaterThan(0);
    expect(
      within(perSchool as HTMLElement).getByText("Ohne Betreuungsform"),
    ).toBeInTheDocument();
  });

  it("keeps zero places finite in each school and the totals", async () => {
    const school = OCCUPANCY.schools[0]!;
    mockFetchOccupancy.mockResolvedValue({
      ...OCCUPANCY,
      schools: [
        {
          ...school,
          careForms: [
            { id: "zero", name: "Keine Plätze", children: 2, capacity: 0 },
            { id: "empty", name: "Leer", children: 0, capacity: 0 },
            { id: "open", name: "Ohne Grenze", children: 2, capacity: null },
          ],
        },
      ],
    });
    renderPage();
    await screen.findByText("9 von 10");
    openTab("Betreuungsformen");

    for (const table of await screen.findAllByRole("table")) {
      const overbooked = within(rowOf(table, "Keine Plätze"));
      expect(
        overbooked.getByText("von 0", { exact: false }),
      ).toBeInTheDocument();
      expect(overbooked.getByText("Überbelegt")).toBeInTheDocument();
      expect(
        within(rowOf(table, "Leer")).getByText("Voll"),
      ).toBeInTheDocument();
      expect(
        within(rowOf(table, "Ohne Grenze")).getByText("Keine Grenze"),
      ).toBeInTheDocument();

      fireEvent.click(within(table).getByRole("button", { name: /^Plätze/ }));
      const names = within(table)
        .getAllByRole("row")
        .slice(1)
        .map((row) => row.textContent);
      expect(names.findIndex((name) => name?.includes("Leer"))).toBeLessThan(
        names.findIndex((name) => name?.includes("Keine Plätze")),
      );
    }
  });

  it("narrows the sums and lists to the schools of the search", async () => {
    mockFetchOccupancy.mockResolvedValue(OCCUPANCY);
    renderPage();
    await screen.findByText("14 von 16");

    fireEvent.change(
      screen.getAllByPlaceholderText(
        "Schule oder Ort suchen…",
      )[0] as HTMLElement,
      {
        target: { value: "melle" },
      },
    );

    expect((await screen.findAllByText("6 von 6")).length).toBe(2);
    expect(
      screen.queryByRole("link", { name: "OGS Am Bach" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getAllByRole("link", { name: "OGS Lindenweg" }).length,
    ).toBeGreaterThan(0);
  });

  it("downloads the CSV export under the name the backend chose", async () => {
    mockFetchOccupancy.mockResolvedValue(OCCUPANCY);
    const blob = new Blob(["Datum"], { type: "text/csv" });
    mockDownload.mockResolvedValue({
      blob,
      filename: "belegung-2026-09-09.csv",
    });
    renderPage();
    await screen.findByText("14 von 16");

    fireEvent.click(
      screen.getByRole("button", { name: "Belegung herunterladen" }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "CSV" }));

    await vi.waitFor(() =>
      expect(mockDownloadBlob).toHaveBeenCalledWith(
        blob,
        "belegung-2026-09-09.csv",
      ),
    );
  });

  it("hides loaded figures when a refresh is refused", async () => {
    mockFetchOccupancy.mockResolvedValueOnce(OCCUPANCY);
    renderPage();
    await screen.findByText("14 von 16");

    mockFetchOccupancy.mockRejectedValue(
      new ApiError("forbidden", 403, { code: "general.forbidden" }),
    );
    await act(async () => {
      await revalidateAll();
    });

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.queryByText("14 von 16")).not.toBeInTheDocument();
    expect(screen.queryByText("Bären")).not.toBeInTheDocument();
  });

  it.each([401, 403, 404])(
    "drops every trace of the loaded figures when a refresh answers %i",
    async (status) => {
      mockFetchOccupancy.mockResolvedValueOnce(OCCUPANCY);
      renderPage();
      await screen.findByText("14 von 16");
      // A view other than the first, so the refusal also covers the tabs.
      openTab("Gruppen");
      expect((await screen.findAllByText("Bären")).length).toBeGreaterThan(0);

      const assign = vi
        .spyOn(window.location, "assign")
        .mockImplementation(() => undefined);
      mockFetchOccupancy.mockRejectedValue(new ApiError("refused", status));
      await act(async () => {
        await revalidateAll();
      });

      if (status === 401) {
        // An ended session goes to the sign-in instead of an error card.
        await vi.waitFor(() => expect(assign).toHaveBeenCalled());
      } else {
        expect(await screen.findByRole("alert")).toBeInTheDocument();
      }
      // Head of the page: no day, no time of the old answer, no school count.
      expect(
        screen.queryByText("09.09.2026, Stand 12:00 Uhr"),
      ).not.toBeInTheDocument();
      expect(screen.queryByText(/Stand \d/)).not.toBeInTheDocument();
      // Body: no figure and no group name.
      expect(screen.queryByText("14 von 16")).not.toBeInTheDocument();
      expect(screen.queryByText("Bären")).not.toBeInTheDocument();
      expect(screen.queryByText("Füchse")).not.toBeInTheDocument();
      // No export of the old answer, and no search over its schools.
      expect(
        screen.queryByPlaceholderText("Schule oder Ort suchen…"),
      ).not.toBeInTheDocument();
      fireEvent.click(
        screen.getByRole("button", { name: "Belegung herunterladen" }),
      );
      expect(
        screen.queryByRole("menuitem", { name: "CSV" }),
      ).not.toBeInTheDocument();
      assign.mockRestore();
    },
  );

  it("keeps loaded figures when a refresh fails for a moment", async () => {
    mockFetchOccupancy.mockResolvedValueOnce(OCCUPANCY);
    renderPage();
    await screen.findByText("14 von 16");

    mockFetchOccupancy.mockRejectedValue(
      new ApiError("Request failed (500)", 500),
    );
    await act(async () => {
      await revalidateAll();
    });

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.getByText("14 von 16")).toBeInTheDocument();
  });

  it("shows the load error without figures", async () => {
    mockFetchOccupancy.mockRejectedValue(
      new ApiError("Request failed (500)", 500),
    );
    renderPage();

    expect(await screen.findByText(/Belegung der Schulen/)).toBeInTheDocument();
    expect(screen.queryByText("14 von 16")).not.toBeInTheDocument();
  });
});
