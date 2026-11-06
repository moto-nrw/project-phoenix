import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  CareContractSummary,
  ContractCandidate,
} from "~/lib/traeger-care-contracts-api";

const { api, mockPush } = vi.hoisted(() => ({
  api: {
    fetchCareContracts: vi.fn(),
    fetchContractCandidates: vi.fn(),
    fetchConclusionModes: vi.fn(),
    fetchTraegerSchools: vi.fn(),
    createCareContracts: vi.fn(),
    sendCareContracts: vi.fn(),
  },
  mockPush: vi.fn(),
}));

vi.mock("~/lib/traeger-care-contracts-api", () => api);
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: {
        id: "A",
        permissions: ["traeger_contracts:manage", "traeger_overview:read"],
      },
    },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/traeger-api", () => ({
  fetchTraegerSchools: api.fetchTraegerSchools,
}));
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
  usePathname: () => "/vertraege",
  useSearchParams: () => new URLSearchParams(),
}));

import TraegerVertraegePage from "./page";

const DRAFT: CareContractSummary = {
  id: "7",
  status: "entwurf",
  schoolId: "3",
  schoolName: "OGS Am Lindenhof",
  childFirstName: "Lina",
  childLastName: "Muster",
  moduleNames: ["Offene Ganztagsschule"],
  partyName: "Erika Muster",
  partyEmail: "erika@example.org",
  effectiveFrom: "2027-08-01",
  createdAt: "2026-09-01T08:00:00Z",
  sentAt: null,
  concluded: null,
};

const SENT: CareContractSummary = {
  ...DRAFT,
  id: "8",
  status: "versendet",
  childFirstName: "Ben",
  sentAt: "2026-09-02T08:00:00Z",
};

const CANDIDATE: ContractCandidate = {
  schoolId: "3",
  requestChildId: "41",
  schoolName: "OGS Am Lindenhof",
  childFirstName: "Mia",
  childLastName: "Kern",
  guardianName: "Paula Kern",
  guardianEmail: "paula@example.org",
  phaseName: "2027/28",
  services: [
    {
      weekday: "mon",
      careOfferingId: "5",
      moduleName: "Kurzbetreuung",
      endsAt: "14:00",
      lastDay: null,
    },
    {
      weekday: "tue",
      careOfferingId: "6",
      moduleName: "Ganztag",
      endsAt: "16:30",
      lastDay: null,
    },
  ],
  effectiveFrom: "2027-08-01",
  templateIssue: null,
  acceptanceIssue: null,
};

function renderPage() {
  render(
    <ToastProvider>
      <TraegerVertraegePage />
    </ToastProvider>,
  );
}

describe("TraegerVertraegePage", () => {
  beforeEach(() => {
    for (const fn of Object.values(api)) fn.mockReset();
    mockPush.mockReset();
    api.fetchCareContracts.mockResolvedValue({
      contracts: [DRAFT, SENT],
      counts: { entwurf: 1, versendet: 1 },
      hasMore: false,
    });
    api.fetchTraegerSchools.mockResolvedValue([
      { id: "3", name: "OGS Am Lindenhof" },
    ]);
    api.fetchConclusionModes.mockResolvedValue([
      { schoolId: "3", schoolName: "OGS Am Lindenhof", mode: "nachtraeglich" },
    ]);
    api.fetchContractCandidates.mockResolvedValue([
      CANDIDATE,
      {
        ...CANDIDATE,
        requestChildId: "42",
        childFirstName: "Tom",
        templateIssue: "template_missing",
      },
      {
        ...CANDIDATE,
        requestChildId: "43",
        childFirstName: "Ida",
        templateIssue: "templates_differ",
      },
    ]);
  });

  it("lists the contracts with status and names the children without one", async () => {
    renderPage();

    expect((await screen.findAllByText("Lina Muster"))[0]).toBeInTheDocument();
    expect(screen.getAllByText("Entwurf")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Versendet")[0]).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Verträge erstellen \(3\)/ }),
    ).toBeInTheDocument();
  });

  it("loads older contracts and sends filters to the server", async () => {
    api.fetchCareContracts
      .mockResolvedValueOnce({
        contracts: [SENT],
        counts: { entwurf: 1100, versendet: 1 },
        hasMore: true,
      })
      .mockResolvedValueOnce({
        contracts: [DRAFT],
        counts: { entwurf: 1100, versendet: 1 },
        hasMore: false,
      });
    renderPage();
    fireEvent.click(
      await screen.findByRole("button", { name: "Weitere Verträge laden" }),
    );
    expect((await screen.findAllByText("Lina Muster"))[0]).toBeInTheDocument();
    expect(screen.getAllByText("Ben Muster")[0]).toBeInTheDocument();
    expect(api.fetchCareContracts).toHaveBeenLastCalledWith({
      status: undefined,
      schoolId: "",
      search: "",
      offset: 1,
    });
    fireEvent.change(
      screen.getAllByPlaceholderText("Kind, Eltern oder Angebot suchen")[0]!,
      { target: { value: "Mia" } },
    );
    await waitFor(() =>
      expect(api.fetchCareContracts).toHaveBeenLastCalledWith({
        status: undefined,
        schoolId: "",
        search: "Mia",
        offset: 0,
      }),
    );
  });

  it("creates one draft per child over its modules, only with equal conditions", async () => {
    api.createCareContracts.mockResolvedValue({
      created: [DRAFT],
      skipped: [],
    });
    renderPage();
    await screen.findAllByText("Lina Muster");

    fireEvent.click(screen.getByRole("button", { name: /Verträge erstellen/ }));
    expect(
      await screen.findByText(
        /Für 1 Kind fehlen die Vertragsbedingungen eines Moduls\. Bei 1 Kind haben die gebuchten Module verschiedene Bedingungen\./,
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Tom Kern auswählen")).toBeDisabled();
    expect(screen.getByLabelText("Ida Kern auswählen")).toBeDisabled();
    expect(
      screen.getByText("Bedingungen der Module weichen ab"),
    ).toBeInTheDocument();
    expect(
      screen.getAllByText("Mo bis 14:00 · Di bis 16:30")[0],
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "1 Entwurf erstellen" }),
    );

    await waitFor(() =>
      expect(api.createCareContracts).toHaveBeenCalledWith([CANDIDATE]),
    );
    expect(
      await screen.findByText(/1 Entwurf ist erstellt/),
    ).toBeInTheDocument();
  });

  it("sends the selected drafts", async () => {
    api.sendCareContracts.mockResolvedValue({ sent: ["7"], skipped: [] });
    renderPage();
    await screen.findAllByText("Lina Muster");

    fireEvent.click(
      screen.getAllByLabelText("Vertrag von Lina Muster auswählen")[0]!,
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "1 Entwurf senden" }),
    );

    await waitFor(() =>
      expect(api.sendCareContracts).toHaveBeenCalledWith(["7"]),
    );
    expect(
      await screen.findByText(/1 Vertrag ist unterwegs/),
    ).toBeInTheDocument();
  });

  it("opens a contract from its row", async () => {
    renderPage();
    fireEvent.click((await screen.findAllByText("Offene Ganztagsschule"))[0]!);

    expect(mockPush).toHaveBeenCalledWith("/vertraege/7");
  });

  it("shows how the contracts of each school come about", async () => {
    api.fetchConclusionModes.mockResolvedValue([
      { schoolId: "3", schoolName: "OGS Am Lindenhof", mode: "bei_annahme" },
      { schoolId: "4", schoolName: "OGS Nord", mode: "nachtraeglich" },
    ]);
    renderPage();

    const list = await screen.findByRole("list", {
      name: "Abschluss je Schule",
    });
    expect(list).toHaveTextContent("OGS Am LindenhofGilt bei der Annahme");
    expect(list).toHaveTextContent("OGS NordSie verschicken ihn");
    expect(
      screen.getByText(/entsteht der Vertrag von selbst, sobald die Schule/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "In den Einstellungen ändern" }),
    ).toHaveAttribute("href", "/einstellungen");
  });

  it("does not offer manual drafts for schools concluding at acceptance and says why each child waits", async () => {
    api.fetchConclusionModes.mockResolvedValue([
      { schoolId: "3", schoolName: "OGS Am Lindenhof", mode: "bei_annahme" },
    ]);
    api.fetchContractCandidates.mockResolvedValue([
      CANDIDATE,
      {
        ...CANDIDATE,
        requestChildId: "44",
        childFirstName: "Ben",
        acceptanceIssue: "acceptance_changed",
      },
      {
        ...CANDIDATE,
        requestChildId: "45",
        childFirstName: "Ole",
        acceptanceIssue: "acceptance_missing",
      },
      {
        ...CANDIDATE,
        requestChildId: "46",
        childFirstName: "Eva",
        guardianEmail: "  ",
      },
    ]);
    renderPage();
    await screen.findAllByText("Lina Muster");
    fireEvent.click(screen.getByRole("button", { name: /Verträge erstellen/ }));
    expect(await screen.findByLabelText("Mia Kern auswählen")).toBeDisabled();
    expect(screen.getByLabelText("Ben Kern auswählen")).toBeDisabled();
    expect(screen.getByLabelText("Ole Kern auswählen")).toBeDisabled();
    expect(screen.getByLabelText("Eva Kern auswählen")).toBeDisabled();
    expect(
      screen.getByText("E-Mail-Adresse der Eltern fehlt"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /Die Schule ergänzt die E-Mail-Adresse in der Anmeldung/,
      ),
    ).toBeInTheDocument();

    // Only the child with the proof comes about by itself.
    expect(screen.getAllByText("Entsteht bei der Annahme")).toHaveLength(1);
    expect(
      screen.getByText("Der Vertrag entsteht in wenigen Minuten von selbst."),
    ).toBeInTheDocument();
    // Changed conditions name the real state and the next step.
    expect(
      screen.getByText("Bedingungen nach der Anmeldung geändert"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /Sie haben die Bedingungen nach der Anmeldung geändert\. Die Eltern haben ihnen nicht zugestimmt\./,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Zustimmung zum Vertrag fehlt"),
    ).toBeInTheDocument();
    expect(
      screen.getAllByText(
        /Zum Senden stellen Sie die Schule auf „Später verschicken“\./,
      ),
    ).toHaveLength(2);
    expect(
      screen.getByText(/sind hier keine Entwürfe nötig/),
    ).toBeInTheDocument();
  });

  it("names an active contract with the day it was concluded", async () => {
    api.fetchCareContracts.mockResolvedValue({
      contracts: [
        {
          ...SENT,
          id: "9",
          status: "aktiv",
          childFirstName: "Ole",
          concluded: { method: "mit_anmeldung", at: "2026-09-03T08:00:00Z" },
        },
      ],
      counts: { aktiv: 1 },
      hasMore: false,
    });
    renderPage();

    expect((await screen.findAllByText("Aktiv"))[0]).toBeInTheDocument();
    expect(screen.getByText("seit 03.09.2026")).toBeInTheDocument();
    expect(screen.getByText("aktiv")).toBeInTheDocument();
  });
});
