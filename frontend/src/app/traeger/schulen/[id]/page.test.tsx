import { SWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";

const { mockFetchSchool, mockStartSession, mockUseSession } = vi.hoisted(
  () => ({
    mockFetchSchool: vi.fn(),
    mockStartSession: vi.fn(),
    mockUseSession: vi.fn(),
  }),
);

vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path,
}));

vi.mock("next-auth/react", () => ({ useSession: mockUseSession }));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "2" }),
  usePathname: () => "/schulen/2",
  useRouter: () => ({ push: vi.fn() }),
}));
vi.mock("~/lib/traeger-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/traeger-api")>()),
  fetchTraegerSchool: mockFetchSchool,
  startTraegerSchoolSession: mockStartSession,
}));
vi.mock("~/env.client", () => ({
  clientEnv: {
    NEXT_PUBLIC_TENANT_DOMAIN: "moto-app.de",
    NEXT_PUBLIC_TRAEGER_HOSTNAME: "traeger.moto-app.de",
  },
}));

import TraegerSchoolPage from "./page";

const SCHOOL = {
  id: "2",
  name: "OGS Am Bach",
  slug: "am-bach",
  subdomain: "am-bach",
  city: "Bissendorf",
  address: "Lindenweg 3",
  zip: "49143",
  phone: "05402 1234",
  email: "ogs@example.com",
  figures: {
    childrenTotal: 120,
    childrenPresent: 51,
    childrenSick: 4,
    staffTotal: 12,
    staffPresent: 6,
    careRatio: 8.5,
    asOf: "2026-09-09T10:00:00Z",
  },
};

function sessionWith(permissions: string[]) {
  return {
    data: { user: { id: "A", scope: "org", orgId: 7, permissions } },
    status: "authenticated",
  };
}

function renderPage() {
  return render(
    <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
      <ToastProvider>
        <TraegerSchoolPage />
      </ToastProvider>
    </SWRConfig>,
  );
}

describe("TraegerSchoolPage", () => {
  beforeEach(() => {
    mockFetchSchool.mockReset();
    mockStartSession.mockReset();
    mockUseSession.mockReturnValue(sessionWith(["traeger_overview:read"]));
  });

  it("shows every figure with its explaining sentence", async () => {
    mockFetchSchool.mockResolvedValue(SCHOOL);
    renderPage();

    expect(await screen.findByText("1 : 8,5")).toBeInTheDocument();
    expect(mockFetchSchool).toHaveBeenCalledWith("2");
    expect(screen.getByRole("heading", { name: "OGS Am Bach" })).toBeVisible();
    expect(screen.getByText("Stand 12:00 Uhr")).toBeVisible();
    for (const [label, value, sentence] of [
      ["Kinder gesamt", "120", "Alle betreuten Kinder dieser Schule."],
      ["Jetzt anwesend", "51", "Kinder, die gerade in der Schule sind."],
      ["Heute krank", "4", "Kinder mit Krankmeldung für heute."],
      [
        "Betreuungskräfte anwesend",
        "6",
        "Angemeldet in der Zeiterfassung. Homeoffice zählt nicht.",
      ],
    ] as const) {
      expect(screen.getByText(label)).toBeInTheDocument();
      expect(screen.getByText(value)).toBeInTheDocument();
      expect(screen.getByText(sentence)).toBeInTheDocument();
    }
  });

  it("says why there is no ratio without present staff", async () => {
    mockFetchSchool.mockResolvedValue({
      ...SCHOOL,
      figures: { ...SCHOOL.figures, staffPresent: 0, careRatio: null },
    });
    renderPage();

    expect(
      await screen.findByText(
        "Gerade ist keine Betreuungskraft in der Zeiterfassung angemeldet.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("–")).toBeInTheDocument();
  });

  it("shows the master data read-only in its tab", async () => {
    mockFetchSchool.mockResolvedValue(SCHOOL);
    renderPage();

    fireEvent.click(await screen.findByRole("tab", { name: "Stammdaten" }));
    expect(await screen.findByText(/Lindenweg 3/)).toHaveTextContent(
      "Lindenweg 349143 Bissendorf",
    );
    expect(screen.getByText("05402 1234")).toBeInTheDocument();
    expect(screen.getByText("ogs@example.com")).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(
      screen.getByText("Nur zum Lesen. Änderungen übernimmt das moto-Team."),
    ).toBeInTheDocument();
  });

  it("offers the Schul-App only with the right", async () => {
    mockFetchSchool.mockResolvedValue(SCHOOL);
    renderPage();
    await screen.findByText("1 : 8,5");
    expect(
      screen.queryByRole("button", {
        name: "Schul-App öffnen: OGS Am Bach, neuer Tab",
      }),
    ).not.toBeInTheDocument();
  });

  it("opens the Schul-App with the right", async () => {
    mockUseSession.mockReturnValue(
      sessionWith(["traeger_overview:read", "traeger_school:switch"]),
    );
    mockFetchSchool.mockResolvedValue(SCHOOL);
    mockStartSession.mockResolvedValue("token");
    const tab = { opener: {}, location: { href: "" }, close: vi.fn() };
    const openSpy = vi
      .spyOn(globalThis, "open")
      .mockReturnValue(tab as unknown as Window);
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Schul-App öffnen: OGS Am Bach, neuer Tab",
      }),
    );
    expect(openSpy).toHaveBeenCalledWith("", "_blank");
    expect(mockStartSession).toHaveBeenCalledWith("2");
    openSpy.mockRestore();
  });

  it("shows a refused school instead of figures", async () => {
    mockFetchSchool.mockRejectedValue(
      new ApiError("forbidden", 403, { code: "general.forbidden" }),
    );
    renderPage();

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.queryByText("Kinder gesamt")).not.toBeInTheDocument();
  });
});
