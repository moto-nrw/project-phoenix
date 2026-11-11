import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { deferred } from "~/test/helpers/form-modal-pending";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { SAVED_BUT_STALE_SUFFIX } from "~/lib/hooks/use-refresh-after-write";
import { catalogText } from "~/test/error-catalog-text";
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
  modulesText: "Offene Ganztagsschule",
  partyName: "Erika Muster",
  partyEmail: "erika@example.org",
  effectiveFrom: "2027-08-01",
  createdAt: "2026-09-01T08:00:00Z",
  sentAt: null,
  concluded: null,
  endsOn: null,
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
  modulesText: "Kurzbetreuung und Ganztag",
  acceptanceIssue: null,
  renewal: false,
  mailBlock: null,
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
    expect(list).toHaveTextContent("OGS Am LindenhofBei der Annahme");
    expect(list).toHaveTextContent("OGS NordSpäter verschicken");
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
      {
        ...CANDIDATE,
        requestChildId: "47",
        childFirstName: "Jona",
        acceptanceIssue: "acceptance_missing",
        renewal: true,
      },
    ]);
    renderPage();
    await screen.findAllByText("Lina Muster");
    fireEvent.click(screen.getByRole("button", { name: /Verträge erstellen/ }));
    expect(await screen.findByLabelText("Mia Kern auswählen")).toBeDisabled();
    expect(screen.getByLabelText("Ben Kern auswählen")).toBeDisabled();
    expect(screen.getByLabelText("Ole Kern auswählen")).toBeDisabled();
    expect(screen.getByLabelText("Eva Kern auswählen")).toBeDisabled();
    expect(screen.getByLabelText("Jona Kern auswählen")).toBeDisabled();
    // A renewal names the missing acceptance of the new phase (#6456).
    expect(
      screen.getByText("Zustimmung zur neuen Phase fehlt"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /Die Eltern haben den Bedingungen für den neuen Zeitraum nicht zugestimmt, bevor die Schule angenommen hat\./,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("E-Mail-Adresse der Eltern fehlt"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /Die Schule ergänzt die E-Mail-Adresse in der Anmeldung/,
      ),
    ).toBeInTheDocument();

    // Only the child with the proof comes about by itself.
    expect(
      screen.getAllByText(
        "Der Vertrag entsteht in wenigen Minuten von selbst.",
      ),
    ).toHaveLength(1);
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
    ).toHaveLength(3);
    expect(
      screen.getByText(/sind hier keine Entwürfe nötig/),
    ).toBeInTheDocument();
  });

  it("names the children whose contract waits for a freed mail address and how the school frees it", async () => {
    api.fetchConclusionModes.mockResolvedValue([
      { schoolId: "3", schoolName: "OGS Am Lindenhof", mode: "bei_annahme" },
    ]);
    api.fetchContractCandidates.mockResolvedValue([
      CANDIDATE,
      {
        ...CANDIDATE,
        requestChildId: "47",
        childFirstName: "Jona",
        mailBlock: "address",
      },
      {
        ...CANDIDATE,
        requestChildId: "48",
        childFirstName: "Lea",
        mailBlock: "stopped",
      },
    ]);
    renderPage();
    await screen.findAllByText("Lina Muster");
    fireEvent.click(screen.getByRole("button", { name: /Verträge erstellen/ }));
    expect(await screen.findByLabelText("Jona Kern auswählen")).toBeDisabled();

    // Only the child whose mail goes out is promised a contract in minutes.
    expect(
      screen.getAllByText(
        "Der Vertrag entsteht in wenigen Minuten von selbst.",
      ),
    ).toHaveLength(1);
    expect(screen.getByText("Mail-Adresse gesperrt")).toBeInTheDocument();
    expect(screen.getByText("Mails gestoppt")).toBeInTheDocument();
    // A failed address is freed at the child; the application's address
    // cannot be changed.
    expect(
      screen.getByText(
        "Der Vertrag für Jona Kern kommt erst, wenn die Schule die Sperre der Mail-Adresse aufhebt. Mails an diese Adresse kamen nicht an. Die Schule prüft die Adresse beim Kind unter „Erziehungsberechtigte“ und wählt „Sperre aufheben“.",
      ),
    ).toBeInTheDocument();
    // A stop is the school's own choice, not a wrong address.
    expect(
      screen.getByText(
        "Der Vertrag für Lea Kern kommt erst, wenn die Schule wieder Mails an die Eltern sendet. Die Schule hat die Mails gestoppt. Sie wählt beim Kind unter „Erziehungsberechtigte“ „Mails wieder senden“.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/in der Anmeldung korrigiert/),
    ).not.toBeInTheDocument();
  });

  it("keeps the held mail out of a school that sends later", async () => {
    api.fetchConclusionModes.mockResolvedValue([
      { schoolId: "3", schoolName: "OGS Am Lindenhof", mode: "nachtraeglich" },
    ]);
    api.fetchContractCandidates.mockResolvedValue([
      { ...CANDIDATE, mailBlock: "address" },
    ]);
    renderPage();
    await screen.findAllByText("Lina Muster");
    fireEvent.click(screen.getByRole("button", { name: /Verträge erstellen/ }));
    expect(await screen.findByLabelText("Mia Kern auswählen")).toBeEnabled();
    expect(screen.getByText("Mia Kern")).toBeInTheDocument();
    expect(screen.queryByText("Mail-Adresse gesperrt")).not.toBeInTheDocument();
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
  // #6652: after a confirmed send a failed reload is a load error above the
  // list, not a failed send or a page error that replaces the contracts.
  describe("reload after a write (#6652)", () => {
    it("shows a reload failure after sending while a new filter has no data yet", async () => {
      const send = deferred<{ sent: string[]; skipped: [] }>();
      const filtered = deferred<{
        contracts: CareContractSummary[];
        counts: {};
        hasMore: boolean;
      }>();
      api.sendCareContracts.mockReturnValueOnce(send.promise);
      renderPage();
      await screen.findAllByText("Lina Muster");
      fireEvent.click(
        screen.getAllByLabelText("Vertrag von Lina Muster auswählen")[0]!,
      );
      fireEvent.click(
        await screen.findByRole("button", { name: "1 Entwurf senden" }),
      );
      api.fetchCareContracts
        .mockReturnValueOnce(filtered.promise)
        .mockRejectedValueOnce(new ApiError("down", 503))
        .mockResolvedValue({
          contracts: [SENT],
          counts: { versendet: 1 },
          hasMore: false,
        });
      fireEvent.click(screen.getByRole("button", { name: "Alle Status" }));
      fireEvent.click(
        await screen.findByRole("option", { name: /^Versendet/ }),
      );
      await waitFor(() =>
        expect(api.fetchCareContracts).toHaveBeenCalledTimes(2),
      );
      await act(async () => send.resolve({ sent: ["7"], skipped: [] }));
      const retry = await screen.findByRole("button", { name: "Wiederholen" });
      expect(screen.getByRole("alert")).toHaveTextContent(
        SAVED_BUT_STALE_SUFFIX,
      );
      await act(async () =>
        filtered.resolve({ contracts: [DRAFT], counts: {}, hasMore: false }),
      );
      fireEvent.click(retry);
      expect((await screen.findAllByText("Ben Muster"))[0]).toBeVisible();
      expect(api.sendCareContracts).toHaveBeenCalledTimes(1);
    });

    it("keeps created drafts created when the reload fails", async () => {
      api.createCareContracts.mockResolvedValue({
        created: [DRAFT],
        skipped: [],
      });
      renderPage();
      await screen.findAllByText("Lina Muster");
      fireEvent.click(screen.getByRole("button", { name: "Alle Status" }));
      fireEvent.click(await screen.findByRole("option", { name: /^Entwurf/ }));
      await waitFor(() =>
        expect(api.fetchCareContracts).toHaveBeenLastCalledWith(
          expect.objectContaining({ status: "entwurf" }),
        ),
      );
      await screen.findAllByText("Lina Muster");
      fireEvent.click(
        screen.getByRole("button", { name: /Verträge erstellen/ }),
      );
      const createButton = await screen.findByRole("button", {
        name: "1 Entwurf erstellen",
      });
      api.fetchCareContracts.mockClear();
      api.fetchCareContracts
        .mockRejectedValueOnce(
          new ApiError("down", 503, { code: "general.unavailable" }),
        )
        .mockResolvedValue({
          contracts: [DRAFT, SENT],
          counts: { entwurf: 1, versendet: 1 },
          hasMore: false,
        });

      fireEvent.click(createButton);

      const reloadText = `${catalogText(
        "general.unavailable",
        "die Anzeige der Verträge",
      )} ${SAVED_BUT_STALE_SUFFIX}`;
      expect(await screen.findByText(reloadText)).toBeInTheDocument();
      expect(api.createCareContracts).toHaveBeenCalledTimes(1);
      // Anker: the confirmed rows stay listed.
      expect(screen.getAllByText("Lina Muster")[0]).toBeInTheDocument();
      expect(
        screen.queryByText(catalogText("general.unavailable", "die Verträge")),
      ).not.toBeInTheDocument();
    });

    it("keeps sent drafts sent when the reload fails", async () => {
      api.sendCareContracts.mockResolvedValue({ sent: ["7"], skipped: [] });
      renderPage();
      await screen.findAllByText("Lina Muster");
      fireEvent.click(
        screen.getAllByLabelText("Vertrag von Lina Muster auswählen")[0]!,
      );
      const sendButton = await screen.findByRole("button", {
        name: "1 Entwurf senden",
      });
      api.fetchCareContracts.mockClear();
      api.fetchCareContracts
        .mockRejectedValueOnce(
          new ApiError("down", 503, { code: "general.unavailable" }),
        )
        .mockResolvedValue({
          contracts: [{ ...DRAFT, status: "versendet" }, SENT],
          counts: { versendet: 2 },
          hasMore: false,
        });

      fireEvent.click(sendButton);

      const reloadText = `${catalogText(
        "general.unavailable",
        "die Anzeige der Verträge",
      )} ${SAVED_BUT_STALE_SUFFIX}`;
      expect(await screen.findByText(reloadText)).toBeInTheDocument();
      expect(api.sendCareContracts).toHaveBeenCalledTimes(1);
      // Anker: the confirmed rows stay listed.
      expect(screen.getAllByText("Lina Muster")[0]).toBeInTheDocument();
      expect(screen.getAllByText("Ben Muster")[0]).toBeInTheDocument();
      expect(
        screen.queryByText(catalogText("general.unavailable", "die Verträge")),
      ).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
      await waitFor(() =>
        expect(api.fetchCareContracts).toHaveBeenCalledTimes(2),
      );
      await waitFor(() =>
        expect(screen.queryByText(reloadText)).not.toBeInTheDocument(),
      );
      expect(screen.getAllByText("Lina Muster")[0]).toBeInTheDocument();
      expect(api.sendCareContracts).toHaveBeenCalledTimes(1);
    });
  });
});
