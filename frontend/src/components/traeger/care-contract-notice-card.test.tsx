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
  fetchContractNoticeOptions: vi.fn(),
  recordContractNotice: vi.fn(),
}));

vi.mock("~/lib/care-contract-notices-api", () => ({
  fetchContractNoticeOptions: mocks.fetchContractNoticeOptions,
  recordContractNotice: mocks.recordContractNotice,
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
    expect(submit).toBeDisabled();
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
});
