import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { OfferingPreview, RosterDeviation } from "~/lib/roster-deviation";

const { mockPreview, mockConnect, mockToastSuccess } = vi.hoisted(() => ({
  mockPreview: vi.fn(),
  mockConnect: vi.fn(),
  mockToastSuccess: vi.fn(),
}));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    getOfferingPreview: mockPreview,
    connectTemplateToOffering: mockConnect,
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
  offering: { id: "3", name: "Randstunde" },
  gradeLevels: [],
  schoolClasses: ["1b"],
  unbookedCount: 2,
  missingCount: 1,
  candidates: [
    { id: "3", name: "Randstunde" },
    { id: "4", name: "Ganztag" },
  ],
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
});
