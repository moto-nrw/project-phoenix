import { SWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
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

function renderPage() {
  return render(
    <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
      <ToastProvider>
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

    openTab("Je Gruppe");

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

    openTab("Je Betreuungsform");

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

  it("shows the load error without figures", async () => {
    mockFetchOccupancy.mockRejectedValue(
      new ApiError("Request failed (500)", 500),
    );
    renderPage();

    expect(await screen.findByText(/Belegung der Schulen/)).toBeInTheDocument();
    expect(screen.queryByText("14 von 16")).not.toBeInTheDocument();
  });
});
