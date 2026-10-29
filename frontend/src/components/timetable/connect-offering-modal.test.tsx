import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { OfferingPreview, RosterDeviation } from "~/lib/roster-deviation";

const { mockPreview, mockConnect, mockToastSuccess, mockOfferingSources } =
  vi.hoisted(() => ({
    mockPreview: vi.fn(),
    mockConnect: vi.fn(),
    mockToastSuccess: vi.fn(),
    mockOfferingSources: vi.fn(),
  }));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    getOfferingPreview: mockPreview,
    connectTemplateToOffering: mockConnect,
    getOfferingSources: mockOfferingSources,
  },
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: mockToastSuccess, error: vi.fn() }),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), info: vi.fn(), warn: vi.fn() }),
}));

import { ConnectOfferingModal } from "./connect-offering-modal";

const deviation: RosterDeviation = {
  templateId: "9",
  offerings: [{ id: "3", name: "Randstunde" }],
  gradeLevels: [],
  schoolClasses: ["1b"],
  unbookedCount: 2,
  missingCount: 1,
  candidates: [
    { id: "3", name: "Randstunde" },
    { id: "4", name: "Ganztag" },
  ],
  ambiguous: false,
};

const preview: OfferingPreview = {
  from: "2026-10-28",
  weekdays: [
    {
      weekday: 3,
      added: [{ studentId: "11", name: "Ada Berg" }],
      removed: [
        { studentId: "12", name: "Ben Kurz" },
        { studentId: "13", name: "Cem Ott" },
      ],
      keptCount: 4,
    },
  ],
  segments: [
    { templateId: "8", validFrom: "2026-09-01", validUntil: "2026-10-30" },
    { templateId: "9", validFrom: "2026-10-30" },
  ],
};

describe("ConnectOfferingModal (#5103)", () => {
  beforeEach(() => {
    mockPreview.mockReset().mockResolvedValue(preview);
    mockConnect.mockReset().mockResolvedValue(undefined);
    mockToastSuccess.mockReset();
    mockOfferingSources.mockReset().mockResolvedValue([]);
  });

  it("previews per weekday who joins and who leaves, then connects every period", async () => {
    const onConnected = vi.fn();
    const onClose = vi.fn();
    render(
      <ConnectOfferingModal
        isOpen
        templateName="RDST 1b"
        deviation={deviation}
        onClose={onClose}
        onConnected={onConnected}
      />,
    );

    expect(mockPreview).toHaveBeenCalledWith("9", {
      offeringIds: ["3"],
      gradeLevels: [],
      schoolClasses: ["1b"],
    });
    expect(await screen.findByText("Mittwoch")).toBeInTheDocument();
    expect(screen.getByText("Kommt dazu (1): Ada Berg")).toBeInTheDocument();
    expect(
      screen.getByText("Fällt weg (2): Ben Kurz, Cem Ott"),
    ).toBeInTheDocument();
    expect(screen.getByText("Bleibt (4)")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Ab 28.10.2026 kommen nur noch die für „Randstunde“ angemeldeten Kinder. An- und Abmeldungen werden danach automatisch übernommen.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Nur Kinder der Klasse 1b")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Gilt für alle Termine ab 28.10.2026, auch für den Abschnitt ab 30.10.2026.",
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Verbinden" }));

    await waitFor(() =>
      expect(mockConnect).toHaveBeenCalledWith(
        "9",
        { offeringIds: ["3"], gradeLevels: [], schoolClasses: ["1b"] },
        "2026-10-28",
      ),
    );
    await waitFor(() => expect(onConnected).toHaveBeenCalled());
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Regeltermin ist mit „Randstunde“ verbunden.",
    );
  });

  it("keeps the dialog open with the error when connecting fails", async () => {
    mockConnect.mockRejectedValue(
      new ApiError("server", 500, { code: "general.server" }),
    );
    const onConnected = vi.fn();
    render(
      <ConnectOfferingModal
        isOpen
        templateName="RDST 1b"
        deviation={deviation}
        onClose={vi.fn()}
        onConnected={onConnected}
      />,
    );
    await screen.findByText("Mittwoch");

    fireEvent.click(screen.getByRole("button", { name: "Verbinden" }));

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(onConnected).not.toHaveBeenCalled();
  });

  it("preselects no offering when several fit equally and previews the chosen one (#5138)", async () => {
    render(
      <ConnectOfferingModal
        isOpen
        templateName="RDST 3a"
        deviation={{
          ...deviation,
          offerings: [{ id: "4", name: "Ganztag" }],
          schoolClasses: ["3a"],
          candidates: [
            { id: "4", name: "Ganztag" },
            { id: "3", name: "Randstunde" },
          ],
          ambiguous: true,
        }}
        onClose={vi.fn()}
        onConnected={vi.fn()}
      />,
    );

    expect(
      screen.getByText(
        "Mehrere Angebote passen zur Kinderliste. Wählen Sie die Angebote, deren angemeldete Kinder kommen sollen.",
      ),
    ).toBeInTheDocument();
    const select = screen.getByRole("button", { name: "Angebote" });
    expect(select).toHaveTextContent("Angebote wählen");
    expect(screen.getByRole("button", { name: "Verbinden" })).toBeDisabled();
    expect(screen.queryByText("Das ändert sich")).not.toBeInTheDocument();
    expect(mockPreview).not.toHaveBeenCalled();

    fireEvent.click(select);
    fireEvent.click(
      await screen.findByRole("checkbox", { name: "Randstunde" }),
    );

    await waitFor(() =>
      expect(mockPreview).toHaveBeenCalledWith("9", {
        offeringIds: ["3"],
        gradeLevels: [],
        schoolClasses: ["3a"],
      }),
    );
    expect(await screen.findByText("Mittwoch")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Ab 28.10.2026 kommen nur noch die für „Randstunde“ angemeldeten Kinder. An- und Abmeldungen werden danach automatisch übernommen.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Verbinden" }));
    await waitFor(() =>
      expect(mockConnect).toHaveBeenCalledWith(
        "9",
        { offeringIds: ["3"], gradeLevels: [], schoolClasses: ["3a"] },
        "2026-10-28",
      ),
    );
  });

  it("connects Randstunde and Ganztag together and keeps the Ganztag children (#5139)", async () => {
    mockOfferingSources.mockResolvedValue([
      { id: "3", name: "Randstunde", phaseId: "1", phaseName: "2026/27" },
      { id: "4", name: "Ganztag", phaseId: "1", phaseName: "2026/27" },
      { id: "5", name: "Ferien", phaseId: "1", phaseName: "2026/27" },
      { id: "6", name: "Nachmeldung", phaseId: "2", phaseName: "Nachmeldung" },
    ]);
    render(
      <ConnectOfferingModal
        isOpen
        templateName="RDST 1a"
        periodId="12"
        deviation={{
          ...deviation,
          offerings: [
            { id: "3", name: "Randstunde" },
            { id: "4", name: "Ganztag" },
          ],
          schoolClasses: ["1a"],
          unbookedCount: 3,
          missingCount: 0,
          candidates: [
            { id: "3", name: "Randstunde" },
            { id: "4", name: "Ganztag" },
          ],
        }}
        onClose={vi.fn()}
        onConnected={vi.fn()}
      />,
    );

    expect(mockPreview).toHaveBeenCalledWith("9", {
      offeringIds: ["3", "4"],
      gradeLevels: [],
      schoolClasses: ["1a"],
    });
    expect(mockOfferingSources).toHaveBeenCalledWith("12");
    expect(
      await screen.findByText(
        "Ab 28.10.2026 kommen nur noch die für „Randstunde“ oder „Ganztag“ angemeldeten Kinder. An- und Abmeldungen werden danach automatisch übernommen.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Angebote" })).toHaveTextContent(
      "Randstunde, Ganztag",
    );

    // Every offering of the period can be added; another phase cannot.
    fireEvent.click(screen.getByRole("button", { name: "Angebote" }));
    expect(
      await screen.findByRole("checkbox", { name: "Ferien" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("checkbox", { name: "Nachmeldung" }),
    ).toBeDisabled();
    expect(
      screen.getByText(
        "Angebote anderer Anmeldephasen sind ausgegraut, solange ein Angebot gewählt ist.",
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Verbinden" }));

    await waitFor(() =>
      expect(mockConnect).toHaveBeenCalledWith(
        "9",
        { offeringIds: ["3", "4"], gradeLevels: [], schoolClasses: ["1a"] },
        "2026-10-28",
      ),
    );
    await waitFor(() =>
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "Regeltermin ist mit „Randstunde“ und „Ganztag“ verbunden.",
      ),
    );
  });
});
