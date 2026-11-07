import {
  act,
  fireEvent,
  render as renderPlain,
  screen,
  waitFor,
} from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { ContractNoticeOptions } from "~/lib/care-contract-notices";

function render(ui: React.ReactElement) {
  return renderPlain(ui, { wrapper: ToastProvider });
}

const mocks = vi.hoisted(() => ({
  fetchMyContractNoticeOptions: vi.fn(),
  declareMyContractNotice: vi.fn(),
}));

vi.mock("~/lib/parent-api", () => ({
  fetchMyContractNoticeOptions: mocks.fetchMyContractNoticeOptions,
  declareMyContractNotice: mocks.declareMyContractNotice,
}));

import { setTestClock } from "~/test/clock";
import { ParentContractNoticePage } from "./parent-contract-notice-page";

const active: ContractNoticeOptions = {
  contractId: "27",
  traegerName: "Ganztag Nord e. V.",
  status: "aktiv",
  childName: "Mariam Kumar",
  modules: "OGS Ganztag",
  schoolName: "OGS Sonnenhof",
  terms: "mit 1 Monat Frist zum Monatsende",
  ordinaryEndsOn: "2026-12-31",
  specialEndsOn: null,
  withdrawalUntil: "2026-11-19",
  earlyStart: false,
  notice: null,
};

describe("ParentContractNoticePage (#4682, § 312k BGB)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("terminates with „Jetzt kündigen“ and shows the confirmation", async () => {
    mocks.fetchMyContractNoticeOptions.mockResolvedValue(active);
    mocks.declareMyContractNotice.mockResolvedValue({
      ...active,
      status: "gekuendigt",
      notice: {
        kind: "kuendigung",
        terminationKind: "ordentlich",
        channel: "eltern_app",
        declaredAt: "2026-11-07T09:30:00Z",
        receivedOn: "2026-11-07",
        endsOn: "2026-12-31",
        recipient: "priya.kumar@email.de",
        confirmationText: "wir bestätigen Ihre Kündigung.",
      },
    });

    render(<ParentContractNoticePage organizationId="2" contractId="27" />);

    expect(
      await screen.findByRole("radio", {
        name: /Ordentlich kündigen zum 31\.12\.2026/,
      }),
    ).toBeChecked();
    expect(
      screen.getByText(
        "Ihr Vertrag endet am 31.12.2026. Bis dahin findet die Betreuung wie gebucht statt.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Jetzt kündigen" }));

    await waitFor(() =>
      expect(mocks.declareMyContractNotice).toHaveBeenCalledWith("2", "27", {
        kind: "kuendigung",
        terminationKind: "ordentlich",
        expectedEndsOn: "2026-12-31",
      }),
    );
    expect(
      await screen.findByText(
        "Ihre Kündigung ist eingegangen. Der Vertrag endet am 31.12.2026.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("wir bestätigen Ihre Kündigung."),
    ).toBeInTheDocument();
  });

  it("offers the withdrawal only within its period", async () => {
    mocks.fetchMyContractNoticeOptions.mockResolvedValue(active);
    render(<ParentContractNoticePage organizationId="2" contractId="27" />);
    fireEvent.click(await screen.findByRole("radio", { name: /Widerrufen/ }));
    expect(
      screen.getByRole("button", { name: "Jetzt widerrufen" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Jetzt widerrufen" }));
    await waitFor(() =>
      expect(mocks.declareMyContractNotice).toHaveBeenCalledWith("2", "27", {
        kind: "widerruf",
      }),
    );
  });

  it("has no withdrawal after the period", async () => {
    mocks.fetchMyContractNoticeOptions.mockResolvedValue({
      ...active,
      withdrawalUntil: null,
    });
    render(<ParentContractNoticePage organizationId="2" contractId="27" />);
    expect(
      await screen.findByRole("radio", { name: /Ordentlich kündigen zum/ }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("radio", { name: /Widerrufen/ }),
    ).not.toBeInTheDocument();
  });

  // § 312k Abs. 2 BGB: the parents name the kind of their termination, and
  // the reason of an extraordinary one. Its end is open until the Träger
  // checked the reason; the confirmation repeats kind, reason and the
  // receipt with its time (§ 312k Abs. 4 BGB).
  it("terminates extraordinarily only with a reason and shows it confirmed", async () => {
    mocks.fetchMyContractNoticeOptions.mockResolvedValue(active);
    mocks.declareMyContractNotice.mockResolvedValue({
      ...active,
      status: "gekuendigt",
      notice: {
        kind: "kuendigung",
        terminationKind: "ausserordentlich",
        reason: "Wir ziehen nach Hamburg.",
        channel: "eltern_app",
        declaredAt: "2026-11-07T09:30:00Z",
        receivedOn: "2026-11-07",
        endsOn: null,
        recipient: "priya.kumar@email.de",
        confirmationText: "wir bestätigen den Eingang Ihrer Kündigung.",
      },
    });
    render(<ParentContractNoticePage organizationId="2" contractId="27" />);

    fireEvent.click(
      await screen.findByRole("radio", { name: /Außerordentlich kündigen/ }),
    );
    expect(
      screen.getByText(
        "Der Träger prüft Ihren Grund. Danach legt er fest, wann der Vertrag endet. Bis dahin findet die Betreuung wie gebucht statt.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Jetzt kündigen" }));
    expect(
      await screen.findByText("Bitte schreiben Sie Ihren Grund."),
    ).toBeInTheDocument();
    expect(mocks.declareMyContractNotice).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText(/Ihr Grund/), {
      target: { value: "  Wir ziehen nach Hamburg. " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Jetzt kündigen" }));
    await waitFor(() =>
      expect(mocks.declareMyContractNotice).toHaveBeenCalledWith("2", "27", {
        kind: "kuendigung",
        terminationKind: "ausserordentlich",
        reason: "Wir ziehen nach Hamburg.",
      }),
    );
    expect(
      await screen.findByText(
        "Ihre Kündigung ist eingegangen. Der Träger prüft Ihren Grund und legt dann das Vertragsende fest.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Außerordentlich")).toBeInTheDocument();
    expect(screen.getByText("Wir ziehen nach Hamburg.")).toBeInTheDocument();
    expect(
      screen.getByText("Legt der Träger nach der Prüfung fest"),
    ).toBeInTheDocument();
    expect(screen.getByText("07.11.2026 um 10:30 Uhr")).toBeInTheDocument();
  });
  it("refreshes at Berlin midnight and requires a new choice when dates change", async () => {
    setTestClock("2026-11-30T22:59:30Z");
    vi.useFakeTimers();
    mocks.fetchMyContractNoticeOptions
      .mockResolvedValueOnce(active)
      .mockResolvedValue({
        ...active,
        ordinaryEndsOn: "2027-01-31",
        withdrawalUntil: null,
      });
    await act(async () => {
      render(<ParentContractNoticePage organizationId="2" contractId="27" />);
    });
    expect(screen.getByRole("radio", { name: /31.12.2026/ })).toBeChecked();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    expect(mocks.fetchMyContractNoticeOptions).toHaveBeenCalledTimes(2);
    expect(
      screen.getByText(
        "Die Termine haben sich geändert. Bitte wählen Sie erneut.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /31.01.2027/ })).not.toBeChecked();
    expect(
      screen.getByRole("button", { name: "Jetzt kündigen" }),
    ).toBeDisabled();
    expect(mocks.declareMyContractNotice).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("radio", { name: /31.01.2027/ }));
    expect(
      screen.getByRole("button", { name: "Jetzt kündigen" }),
    ).toBeEnabled();
  });
});
