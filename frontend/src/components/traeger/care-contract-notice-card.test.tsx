import { render as renderPlain, screen } from "@testing-library/react";
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
});
