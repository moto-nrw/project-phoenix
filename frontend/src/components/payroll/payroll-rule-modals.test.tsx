import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";

import {
  CreateSurchargeRuleModal,
  CreateWageTypeModal,
} from "./payroll-rule-modals";

const { createWageTypeMock, createSurchargeRuleMock } = vi.hoisted(() => ({
  createWageTypeMock: vi.fn(),
  createSurchargeRuleMock: vi.fn(),
}));

vi.mock("~/lib/payroll-rules-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/payroll-rules-api")>()),
  createWageType: createWageTypeMock,
  createSurchargeRule: createSurchargeRuleMock,
}));

describe("payroll rule dialogs (#4807)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    createWageTypeMock.mockResolvedValue(undefined);
    createSurchargeRuleMock.mockResolvedValue(undefined);
  });

  it("legt eine Lohnart mit Bezeichnung, Nummer und Einheit an", async () => {
    const onCreated = vi.fn();
    const onClose = vi.fn();
    render(
      <CreateWageTypeModal isOpen onClose={onClose} onCreated={onCreated} />,
      { wrapper: ToastProvider },
    );

    fireEvent.change(screen.getByLabelText(/Bezeichnung/), {
      target: { value: " Sonntagszuschlag " },
    });
    fireEvent.change(screen.getByLabelText(/Lohnartnummer/), {
      target: { value: "320" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Betrag in Euro" }));
    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));

    await waitFor(() =>
      expect(createWageTypeMock).toHaveBeenCalledWith({
        name: "Sonntagszuschlag",
        number: "320",
        unit: "betrag",
      }),
    );
    await waitFor(() => expect(onCreated).toHaveBeenCalled());
    expect(onClose).toHaveBeenCalled();
  });

  it("legt keine Lohnart mit fünf Ziffern an", () => {
    render(
      <CreateWageTypeModal isOpen onClose={vi.fn()} onCreated={vi.fn()} />,
      { wrapper: ToastProvider },
    );

    fireEvent.change(screen.getByLabelText(/Bezeichnung/), {
      target: { value: "Zuschlag" },
    });
    fireEvent.change(screen.getByLabelText(/Lohnartnummer/), {
      target: { value: "12345" },
    });

    expect(screen.getByText("1 bis 4 Ziffern.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Anlegen" })).toBeDisabled();
  });

  it("legt eine Nachtregel mit dem vorgegebenen Zeitraum an", async () => {
    render(
      <CreateSurchargeRuleModal
        isOpen
        wageTypes={[
          { id: "7", name: "Nachtzuschlag", number: "310", unit: "stunden" },
          { id: "8", name: "Sonntag", number: "320", unit: "betrag" },
        ]}
        onClose={vi.fn()}
        onCreated={vi.fn()}
      />,
      { wrapper: ToastProvider },
    );

    fireEvent.change(screen.getByLabelText(/^Prozent/), {
      target: { value: "25" },
    });
    fireEvent.click(screen.getByRole("combobox", { name: "Lohnart" }));
    fireEvent.click(
      await screen.findByRole("option", { name: "Nachtzuschlag" }),
    );
    expect(screen.queryByRole("option", { name: "Sonntag" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));

    await waitFor(() =>
      expect(createSurchargeRuleMock).toHaveBeenCalledWith({
        kind: "nacht",
        windowStartMinute: 1200,
        windowEndMinute: 360,
        rateType: "prozent",
        rateHundredths: 2500,
        wageTypeId: "7",
      }),
    );
  });
});
