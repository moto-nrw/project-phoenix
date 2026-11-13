import {
  act,
  fireEvent,
  render as renderPlain,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { RejectReasonModal } from "./care-contract-notice-modal";
import { ToastProvider } from "~/contexts/ToastContext";
import type { ContractNoticeOptions } from "~/lib/care-contract-notices";
import { unavailableApiError } from "~/lib/api-error";

function render(ui: React.ReactElement) {
  return renderPlain(ui, { wrapper: ToastProvider });
}

const mocks = vi.hoisted(() => ({
  fetchContractNoticeOptions: vi.fn(),
  recordContractNotice: vi.fn(),
  setContractNoticeEnd: vi.fn(),
  rejectContractNoticeReason: vi.fn(),
}));

vi.mock("~/lib/care-contract-notices-api", () => ({
  fetchContractNoticeOptions: mocks.fetchContractNoticeOptions,
  recordContractNotice: mocks.recordContractNotice,
  setContractNoticeEnd: mocks.setContractNoticeEnd,
  rejectContractNoticeReason: mocks.rejectContractNoticeReason,
}));

vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));

import { parseISODate, toISODate } from "~/lib/date-helpers";
vi.mock("~/components/ui/date-picker", () => ({
  DatePicker: ({
    id,
    value,
    onChange,
  }: {
    id: string;
    value?: Date;
    onChange: (value: Date | undefined) => void;
  }) => (
    <input
      id={id}
      type="date"
      value={value ? toISODate(value) : ""}
      onChange={(event) => onChange(parseISODate(event.target.value))}
    />
  ),
}));

import { CareContractNoticeCard } from "./care-contract-notice-card";

const active: ContractNoticeOptions = {
  contractId: "25",
  traegerName: "",
  status: "aktiv",
  childName: "Bente Jansen",
  modules: "Kurzbetreuung",
  schoolName: "OGS Sonnenhof",
  terms: "mit 1 Monat Frist zum Monatsende",
  ordinaryEndsOn: "2026-12-31",
  specialEndsOn: null,
  withdrawalUntil: null,
  earlyStart: true,
  notice: null,
};

describe("CareContractNoticeCard (#4682)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows what an active contract allows today and offers the entry", async () => {
    mocks.fetchContractNoticeOptions.mockResolvedValue(active);
    render(
      <CareContractNoticeCard
        contractId="25"
        status="aktiv"
        concludedAt="2026-11-05T03:42:00Z"
        onChanged={vi.fn()}
      />,
    );
    expect(await screen.findByText("31.12.2026")).toBeInTheDocument();
    expect(screen.getByText("Nicht mehr möglich")).toBeInTheDocument();
    expect(screen.getByText("Von den Eltern verlangt")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Kündigung eintragen" }),
    ).toBeInTheDocument();
  });

  it("shows a declaration with its confirmation and no entry", async () => {
    mocks.fetchContractNoticeOptions.mockResolvedValue({
      ...active,
      status: "gekuendigt",
      notice: {
        kind: "kuendigung",
        terminationKind: "ordentlich",
        channel: "eltern_app",
        declaredAt: "2026-11-07T09:30:00Z",
        receivedOn: "2026-11-07",
        endsOn: "2026-12-31",
        recipient: "birgit.jansen@email.de",
        confirmationText: "wir bestätigen Ihre Kündigung.",
      },
    });
    render(
      <CareContractNoticeCard
        contractId="25"
        status="gekuendigt"
        concludedAt="2026-11-05T03:42:00Z"
        onChanged={vi.fn()}
      />,
    );
    expect(
      await screen.findByText("wir bestätigen Ihre Kündigung."),
    ).toBeInTheDocument();
    expect(screen.getByText("birgit.jansen@email.de")).toBeInTheDocument();
    expect(screen.getByText("Gekündigt zum")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Kündigung eintragen" }),
    ).not.toBeInTheDocument();
  });
  it("offers timely withdrawal and special termination for a backdated receipt", async () => {
    mocks.fetchContractNoticeOptions.mockImplementation(
      (_id: string, receivedOn?: string) =>
        Promise.resolve(
          receivedOn === "2026-09-01"
            ? {
                ...active,
                ordinaryEndsOn: "2027-07-31",
                specialEndsOn: "2027-01-31",
                withdrawalUntil: "2026-09-02",
              }
            : active,
        ),
    );
    render(
      <CareContractNoticeCard
        contractId="25"
        status="aktiv"
        concludedAt="2026-08-19T09:00:00Z"
        onChanged={vi.fn()}
      />,
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "Kündigung eintragen" }),
    );
    // The warning names where the text of the parents' e-mail is set; a
    // new tab keeps the dialog.
    const mailText = await screen.findByRole("link", {
      name: "Text der E-Mail ansehen, neuer Tab",
    });
    expect(mailText).toHaveAttribute(
      "href",
      "/einstellungen?bereich=vertraege",
    );
    expect(mailText).toHaveAttribute("target", "_blank");
    fireEvent.change(await screen.findByLabelText(/Eingegangen am/), {
      target: { value: "2026-09-01" },
    });
    expect(
      await screen.findByText(/Sonderkündigung zum Halbjahr:/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("radio", { name: /Sonderkündigung/ }));
    fireEvent.click(screen.getByRole("button", { name: /^Widerruf$/ }));
    expect(
      await screen.findByText(/Rechtzeitig abgesendet: bis 02.09.2026/),
    ).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText(/Eingegangen am/), {
      target: { value: "2026-09-09" },
    });
    await waitFor(() =>
      expect(
        screen.getByText(
          "Ein Widerruf ist nicht mehr möglich. Die Eltern können kündigen.",
        ),
      ).toBeInTheDocument(),
    );
    expect(
      screen.getByRole("radio", { name: /Nächster Termin/ }),
    ).toBeChecked();
  });
  it("blocks submission while dates load and ignores superseded responses", async () => {
    let resolveEarlier!: (options: ContractNoticeOptions) => void;
    mocks.fetchContractNoticeOptions.mockImplementation(
      (_id: string, day?: string) =>
        day === "2026-09-01"
          ? new Promise<ContractNoticeOptions>((resolve) => {
              resolveEarlier = resolve;
            })
          : Promise.resolve(active),
    );
    render(
      <CareContractNoticeCard
        contractId="25"
        status="aktiv"
        concludedAt="2026-08-19T09:00:00Z"
        onChanged={vi.fn()}
      />,
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "Kündigung eintragen" }),
    );
    fireEvent.change(await screen.findByLabelText(/Eingegangen am/), {
      target: { value: "2026-09-01" },
    });
    expect(
      document.querySelector('button[form="care-contract-notice-form"]'),
    ).toBeDisabled();
    expect(mocks.recordContractNotice).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText(/Eingegangen am/), {
      target: { value: "2026-09-09" },
    });
    await screen.findByText(
      "Ein Widerruf ist nicht mehr möglich. Die Eltern können kündigen.",
    );
    await act(async () =>
      resolveEarlier({ ...active, withdrawalUntil: "2026-09-02" }),
    );
    expect(
      screen.getByText(
        "Ein Widerruf ist nicht mehr möglich. Die Eltern können kündigen.",
      ),
    ).toBeInTheDocument();
    expect(mocks.fetchContractNoticeOptions).toHaveBeenCalledWith(
      "25",
      "2026-09-01",
    );
  });
  it("submits an office declaration only once while saving", async () => {
    mocks.fetchContractNoticeOptions.mockResolvedValue(active);
    mocks.recordContractNotice.mockReturnValue(new Promise(() => {}));
    render(
      <CareContractNoticeCard
        contractId="25"
        status="aktiv"
        concludedAt="2026-08-19T09:00:00Z"
        onChanged={vi.fn()}
      />,
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "Kündigung eintragen" }),
    );
    await screen.findByText(/Nächster Termin:/);
    const submit = document.querySelector(
      'button[form="care-contract-notice-form"]',
    )!;
    fireEvent.click(submit);
    fireEvent.click(submit);
    expect(submit).toBeDisabled();
    expect(mocks.recordContractNotice).toHaveBeenCalledTimes(1);
    expect(mocks.recordContractNotice).toHaveBeenCalledWith(
      "25",
      expect.objectContaining({ expectedEndsOn: "2026-12-31" }),
    );
  });
  // A lost connection may still have recorded the notice and sent its
  // mail: the dialog reads the contract before a second try could repeat it.
  it("shows a notice recorded despite a lost connection instead of a retry", async () => {
    const recorded = {
      ...active,
      status: "gekuendigt" as const,
      notice: {
        kind: "kuendigung" as const,
        terminationKind: "ordentlich" as const,
        channel: "post" as const,
        declaredAt: "2026-09-09T09:30:00Z",
        receivedOn: "2026-09-09",
        endsOn: "2026-12-31",
        recipient: "birgit.jansen@email.de",
        confirmationText: "wir bestätigen Ihre Kündigung.",
      },
    };
    mocks.fetchContractNoticeOptions
      .mockResolvedValueOnce(active)
      .mockResolvedValueOnce(active)
      .mockResolvedValue(recorded);
    mocks.recordContractNotice.mockRejectedValue(
      unavailableApiError(new TypeError("Failed to fetch")),
    );
    const onChanged = vi.fn();
    render(
      <CareContractNoticeCard
        contractId="25"
        status="aktiv"
        concludedAt="2026-08-19T09:00:00Z"
        onChanged={onChanged}
      />,
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "Kündigung eintragen" }),
    );
    await screen.findByText(/Nächster Termin:/);
    fireEvent.click(
      document.querySelector('button[form="care-contract-notice-form"]')!,
    );

    expect(
      await screen.findByText(
        "Die Kündigung ist eingetragen. Die Eltern bekommen eine Bestätigung per E-Mail.",
      ),
    ).toBeInTheDocument();
    expect(mocks.recordContractNotice).toHaveBeenCalledTimes(1);
  });

  it("records a timely dispatch separately from a late receipt", async () => {
    mocks.fetchContractNoticeOptions.mockResolvedValue({
      ...active,
      withdrawalUntil: "2026-09-02",
    });
    mocks.recordContractNotice.mockResolvedValue({
      ...active,
      status: "widerrufen",
    });
    render(
      <CareContractNoticeCard
        contractId="25"
        status="aktiv"
        concludedAt="2026-08-19T09:00:00Z"
        onChanged={vi.fn()}
      />,
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "Kündigung eintragen" }),
    );
    fireEvent.click(await screen.findByRole("button", { name: /^Widerruf$/ }));
    const submit = document.querySelector(
      'button[form="care-contract-notice-form"]',
    )!;
    expect(submit).toHaveAttribute("aria-disabled", "true");
    fireEvent.change(screen.getByLabelText(/Abgesendet am/), {
      target: { value: "2026-09-01" },
    });
    fireEvent.click(submit);
    await waitFor(() =>
      expect(mocks.recordContractNotice).toHaveBeenCalledWith("25", {
        kind: "widerruf",
        terminationKind: "ordentlich",
        receivedOn: "2026-09-09",
        dispatchedOn: "2026-09-01",
      }),
    );
  });

  // An extraordinary termination shows its kind and reason with an open
  // end; the office sets the end after its check, once (#4682).
  it("lets the office set the end of an extraordinary termination", async () => {
    const pending: ContractNoticeOptions = {
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
        recipient: "birgit.jansen@email.de",
        confirmationText: "wir bestätigen den Eingang.",
      },
    };
    mocks.fetchContractNoticeOptions.mockResolvedValue(pending);
    const ended = {
      ...pending,
      notice: { ...pending.notice!, endsOn: "2026-12-31" },
    };
    mocks.setContractNoticeEnd.mockResolvedValue(ended);
    const onChanged = vi.fn();
    render(
      <CareContractNoticeCard
        contractId="25"
        status="gekuendigt"
        concludedAt="2026-11-05T03:42:00Z"
        onChanged={onChanged}
      />,
    );
    expect(await screen.findByText("Ende noch offen")).toBeInTheDocument();
    expect(
      screen.getByText("Außerordentlich, aus wichtigem Grund"),
    ).toBeInTheDocument();
    expect(screen.getByText("Wir ziehen nach Hamburg.")).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Vertragsende festlegen" }),
    );
    fireEvent.change(await screen.findByLabelText(/Letzter Tag/), {
      target: { value: "2026-12-31" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Vertragsende eintragen" }),
    );
    await waitFor(() =>
      expect(mocks.setContractNoticeEnd).toHaveBeenCalledWith(
        "25",
        "2026-12-31",
      ),
    );
    expect(await screen.findByText("31.12.2026")).toBeInTheDocument();
    expect(onChanged).toHaveBeenCalled();
    expect(
      screen.queryByRole("button", { name: "Vertragsende festlegen" }),
    ).not.toBeInTheDocument();
  });

  it("lets the office refuse the reason with the end it was shown, and shows the notice sent", async () => {
    const pending: ContractNoticeOptions = {
      ...active,
      status: "gekuendigt",
      reinterpretedEndsOn: "2026-12-31",
      notice: {
        kind: "kuendigung",
        terminationKind: "ausserordentlich",
        reason: "Wir ziehen nach Hamburg.",
        channel: "eltern_app",
        declaredAt: "2026-11-07T09:30:00Z",
        receivedOn: "2026-11-07",
        endsOn: null,
        recipient: "birgit.jansen@email.de",
        confirmationText: "wir bestätigen den Eingang.",
      },
    };
    mocks.fetchContractNoticeOptions.mockResolvedValue(pending);
    mocks.rejectContractNoticeReason.mockResolvedValue({
      ...pending,
      reinterpretedEndsOn: null,
      notice: {
        ...pending.notice!,
        endsOn: "2026-12-31",
        endDecision: "umgedeutet",
        endDecidedAt: "2026-11-09T10:00:00Z",
        endNoticeText: "Ihren Grund erkennen wir nicht an.",
        endNoticeSent: true,
      },
    });
    const onChanged = vi.fn();
    render(
      <CareContractNoticeCard
        contractId="25"
        status="gekuendigt"
        concludedAt="2026-11-05T03:42:00Z"
        onChanged={onChanged}
      />,
    );
    expect(
      await screen.findByText(
        "Dann gilt sie als ordentliche Kündigung zum 31.12.2026.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Grund nicht anerkennen" }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent("Neues Vertragsende31.12.2026");
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Grund nicht anerkennen" }),
    );
    await waitFor(() =>
      expect(mocks.rejectContractNoticeReason).toHaveBeenCalledWith(
        "25",
        "2026-12-31",
      ),
    );
    expect(mocks.rejectContractNoticeReason).toHaveBeenCalledTimes(1);
    expect(
      await screen.findByText("Grund nicht anerkannt"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("figure", {
        name: "Mitteilung zum Vertragsende, wie sie verschickt wurde",
      }),
    ).toHaveTextContent("Ihren Grund erkennen wir nicht an.");
    expect(onChanged).toHaveBeenCalled();
    expect(
      screen.queryByRole("button", { name: "Vertragsende festlegen" }),
    ).not.toBeInTheDocument();
  });
});

describe("RejectReasonModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });
  it("locks dismissal while saving and displays a failed decision inline", async () => {
    let refuse!: (error: Error) => void;
    mocks.rejectContractNoticeReason.mockReturnValue(
      new Promise((_resolve, reject) => {
        refuse = reject;
      }),
    );
    const onClose = vi.fn();
    const onRejected = vi.fn();
    render(
      <RejectReasonModal
        isOpen
        onClose={onClose}
        contractId="25"
        reason="Wir ziehen um."
        receivedOn="2026-11-07"
        endsOn="2026-12-31"
        onRejected={onRejected}
      />,
    );
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Grund nicht anerkennen" }),
    );
    const saving = await within(dialog).findByRole("button", {
      name: "Wird eingetragen …",
    });
    expect(saving).toBeDisabled();
    expect(
      within(dialog).getByRole("button", { name: "Abbrechen" }),
    ).toBeDisabled();
    fireEvent.click(saving);
    fireEvent.click(within(dialog).getByRole("button", { name: "Abbrechen" }));
    fireEvent.keyDown(document, { key: "Escape" });
    expect(mocks.rejectContractNoticeReason).toHaveBeenCalledTimes(1);
    expect(onClose).not.toHaveBeenCalled();
    await act(async () => {
      refuse(new Error("request failed"));
    });
    const error = await within(dialog).findByText(
      /Entscheidung konnte nicht bearbeitet werden/,
    );
    expect(error.closest('[role="alert"]')).toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: "Grund nicht anerkennen" }),
    ).toBeEnabled();
    expect(onRejected).not.toHaveBeenCalled();
  });

  it.each([
    [
      "missing_student",
      "Der Vertrag ist mit keinem Kind aus den „Kinderdaten“ der Schule verknüpft.",
    ],
    [
      "no_permitted_address",
      "Keine Adresse des Vertrags gehört noch einer Bezugsperson, die für das Kind handeln darf.",
    ],
    [
      undefined,
      "moto hat keine Adresse gefunden, an die die Mitteilung gehen darf.",
    ],
  ] as const)("explains the unsent notice: %s", async (reason, explanation) => {
    mocks.fetchContractNoticeOptions.mockResolvedValue({
      ...active,
      status: "gekuendigt",
      notice: {
        kind: "kuendigung",
        terminationKind: "ausserordentlich",
        reason: "Wir ziehen nach Hamburg.",
        channel: "eltern_app",
        declaredAt: "2026-11-07T09:30:00Z",
        receivedOn: "2026-11-07",
        endsOn: "2026-12-31",
        recipient: "birgit.jansen@email.de",
        confirmationText: "wir bestätigen den Eingang.",
        endDecision: "festgelegt",
        endDecidedAt: "2026-11-09T10:00:00Z",
        endNoticeText: "Der Vertrag endet am 31.12.2026.",
        endNoticeSent: false,
        endNoticeNotSentReason: reason,
      },
    } satisfies ContractNoticeOptions);
    render(
      <CareContractNoticeCard
        contractId="25"
        status="gekuendigt"
        concludedAt="2026-11-05T03:42:00Z"
        onChanged={vi.fn()}
      />,
    );
    expect(
      await screen.findByText(
        "Die Mitteilung zum Vertragsende ging nicht raus",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        `${explanation} Schicken Sie den Eltern die Mitteilung unten per Brief.`,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("figure", {
        name: "Mitteilung zum Vertragsende (nicht verschickt)",
      }),
    ).toHaveTextContent("Der Vertrag endet am 31.12.2026.");
    expect(
      screen.queryByRole("figure", {
        name: "Mitteilung zum Vertragsende, wie sie verschickt wurde",
      }),
    ).not.toBeInTheDocument();
    if (reason !== "no_permitted_address") {
      expect(
        screen.queryByText(/Keine Adresse des Vertrags gehört/),
      ).not.toBeInTheDocument();
    }
  });
});
