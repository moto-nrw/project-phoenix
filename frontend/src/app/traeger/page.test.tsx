import { SWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import {
  fireEvent,
  render as rtlRender,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const { mockFetchSchools, mockStartSchoolSession, mockUseSession } = vi.hoisted(
  () => ({
    mockFetchSchools: vi.fn(),
    mockStartSchoolSession: vi.fn(),
    mockUseSession: vi.fn(),
  }),
);

vi.mock("~/lib/traeger-api", () => ({
  fetchTraegerSchools: () => mockFetchSchools(),
  startTraegerSchoolSession: (schoolId: string) =>
    mockStartSchoolSession(schoolId),
}));

vi.mock("next-auth/react", () => ({ useSession: () => mockUseSession() }));

vi.mock("~/env.client", () => ({
  clientEnv: {
    NEXT_PUBLIC_TENANT_DOMAIN: "moto-app.de",
    NEXT_PUBLIC_TRAEGER_HOSTNAME: "traeger.moto-app.de",
  },
}));

const AM_BACH = {
  id: "2",
  name: "OGS Am Bach",
  slug: "am-bach",
  subdomain: "am-bach",
  city: "Bissendorf",
};

// The page shows failed openings as a toast.
function renderPage() {
  return render(
    <ToastProvider>
      <TraegerSchoolsPage />
    </ToastProvider>,
  );
}

function sessionWith(permissions: string[]) {
  return {
    data: { user: { id: "A", scope: "org", orgId: 7, permissions } },
    status: "authenticated",
  };
}

import TraegerSchoolsPage from "./page";

describe("TraegerSchoolsPage", () => {
  beforeEach(() => {
    mockFetchSchools.mockReset();
    mockStartSchoolSession.mockReset();
    mockUseSession.mockReturnValue(sessionWith(["traeger_overview:read"]));
  });

  it("lists every school of the Träger as a read-only table", async () => {
    mockFetchSchools.mockResolvedValue([
      {
        id: "2",
        name: "OGS Am Bach",
        slug: "am-bach",
        subdomain: "am-bach",
        city: "Bissendorf",
      },
      {
        id: "3",
        name: "OGS Lindenweg",
        slug: "lindenweg",
        subdomain: "lindenweg",
        city: "",
      },
    ]);
    renderPage();

    const table = await screen.findByRole("table");
    expect(within(table).getByText("OGS Am Bach")).toBeInTheDocument();
    expect(within(table).getByText("Bissendorf")).toBeInTheDocument();
    // A school without a city shows the empty-value glyph.
    expect(within(table).getByText("–")).toBeInTheDocument();
    // Die Erklärung steht hinter dem Info-Knopf am Titel (#5146).
    fireEvent.click(
      screen.getByRole("button", { name: "Erklärung zu Schulen" }),
    );
    expect(
      screen.getByRole("dialog", { name: "Erklärung zu Schulen" }),
    ).toHaveTextContent(
      "Alle Schulen Ihres Trägers. Neue Schulen ordnet das moto-Team zu.",
    );
    // Anker: die Tabelle ist geladen; Zeilen sind reine Anzeige.
    expect(within(table).getByText("OGS Lindenweg")).toBeInTheDocument();
    expect(within(table).queryAllByRole("button", { name: /OGS/ })).toEqual([]);
  });

  it("says who adds schools when the Träger has none yet", async () => {
    mockFetchSchools.mockResolvedValue([]);
    renderPage();

    // Table and phone layout share the empty state.
    const empty = await screen.findAllByText(
      "Ihrem Träger sind noch keine Schulen zugeordnet.",
    );
    expect(empty[0]).toBeInTheDocument();
    expect(
      screen.getAllByText(
        "Das moto-Team ordnet die Schulen Ihrem Träger zu.",
      )[0],
    ).toBeInTheDocument();
  });

  it("shows a failed load where the list is missing and retries", async () => {
    mockFetchSchools
      .mockRejectedValueOnce(
        new ApiError("unavailable", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce([
        {
          id: "2",
          name: "OGS Am Bach",
          slug: "am-bach",
          subdomain: "am-bach",
          city: "Bissendorf",
        },
      ]);
    renderPage();

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Schulen"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect((await screen.findAllByText("OGS Am Bach"))[0]).toBeInTheDocument();
    expect(mockFetchSchools).toHaveBeenCalledTimes(2);
  });

  it("opens a school in a new tab for a person with the right to switch (#4654)", async () => {
    mockUseSession.mockReturnValue(
      sessionWith(["traeger_overview:read", "traeger_school:switch"]),
    );
    mockFetchSchools.mockResolvedValue([AM_BACH]);
    mockStartSchoolSession.mockResolvedValue("handoff-token");
    const tab = { opener: {}, location: { href: "" }, close: vi.fn() };
    const open = vi
      .spyOn(globalThis, "open")
      .mockReturnValue(tab as unknown as Window);
    renderPage();

    const table = await screen.findByRole("table");
    fireEvent.click(
      screen.getByRole("button", { name: "Erklärung zu Schulen" }),
    );
    expect(
      screen.getByRole("dialog", { name: "Erklärung zu Schulen" }),
    ).toHaveTextContent(/„Öffnen“ zeigt die Schule in einem neuen Tab/);
    fireEvent.click(
      within(table).getByRole("button", { name: "OGS Am Bach öffnen" }),
    );

    await waitFor(() =>
      expect(tab.location.href).toBe(
        `${globalThis.location.protocol}//am-bach.moto-app.de${globalThis.location.port ? `:${globalThis.location.port}` : ""}/traeger-uebergabe#token=handoff-token`,
      ),
    );
    expect(mockStartSchoolSession).toHaveBeenCalledWith("2");
    expect(open).toHaveBeenCalledWith("", "_blank");
    expect(tab.opener).toBeNull();
    open.mockRestore();
  });

  it("closes the new tab and names the school when it cannot be opened", async () => {
    mockUseSession.mockReturnValue(
      sessionWith(["traeger_overview:read", "traeger_school:switch"]),
    );
    mockFetchSchools.mockResolvedValue([AM_BACH]);
    mockStartSchoolSession.mockRejectedValue(
      new ApiError("not found", 404, { code: "traeger.school_not_found" }),
    );
    const tab = { opener: {}, location: { href: "" }, close: vi.fn() };
    const open = vi
      .spyOn(globalThis, "open")
      .mockReturnValue(tab as unknown as Window);
    renderPage();

    const table = await screen.findByRole("table");
    fireEvent.click(
      within(table).getByRole("button", { name: "OGS Am Bach öffnen" }),
    );

    expect(
      await screen.findByText(
        catalogText("traeger.school_not_found", "die Schule"),
      ),
    ).toBeInTheDocument();
    expect(tab.close).toHaveBeenCalled();
    expect(tab.location.href).toBe("");
    open.mockRestore();
  });
});

function render(ui: React.ReactElement) {
  return rtlRender(
    <SWRConfig value={{ provider: () => new Map() }}>{ui}</SWRConfig>,
  );
}
