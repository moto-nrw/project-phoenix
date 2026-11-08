import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { BillingTerms } from "~/lib/traeger-billing-terms-api";
import { BillingTermsCard, parseDueDays } from "./billing-terms-card";

const { saveServiceTerms } = vi.hoisted(() => ({
  saveServiceTerms: vi.fn(),
}));
vi.mock("~/lib/traeger-billing-terms-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  saveServiceTerms,
}));

const data: BillingTerms = {
  services: [
    { service: "care", bundleWith: [] },
    { service: "meal", bundleWith: ["care"] },
    { service: "holiday", bundleWith: [] },
  ],
  terms: [
    {
      service: "care",
      paymentMethod: "direct_debit",
      dueDays: 10,
      bundleWith: "",
      custom: true,
      updatedAt: "2026-09-01T10:00:00Z",
    },
    {
      service: "meal",
      paymentMethod: "transfer",
      dueDays: 14,
      bundleWith: "care",
      custom: false,
      updatedAt: null,
    },
    {
      service: "holiday",
      paymentMethod: "transfer",
      dueDays: 14,
      bundleWith: "",
      custom: false,
      updatedAt: null,
    },
  ],
};

function renderCard() {
  const onChanged = vi.fn(() => Promise.resolve());
  render(
    <ToastProvider>
      <BillingTermsCard data={data} onChanged={onChanged} />
    </ToastProvider>,
  );
  // The table renders a stacked copy for phones; read the desktop table.
  return { onChanged, table: within(screen.getByRole("table")) };
}

describe("BillingTermsCard", () => {
  it("shows each service's payment and invoice", () => {
    const { table } = renderCard();
    expect(table.getByText("Monatsbeitrag Betreuung")).toBeInTheDocument();
    expect(table.getByText("Lastschrift")).toBeInTheDocument();
    expect(table.getByText("10 Tage")).toBeInTheDocument();
    expect(
      table.getByText("Auf der Rechnung der Betreuung"),
    ).toBeInTheDocument();
    expect(table.getByText("Eigene Rechnung")).toBeInTheDocument();
    expect(table.getAllByText("Immer eigene Rechnung")).toHaveLength(1);
  });

  it("refuses a payment term outside 0 to 120 days", async () => {
    const { table } = renderCard();
    fireEvent.click(table.getByText("Mittagessen"));
    fireEvent.change(screen.getByLabelText(/Zahlungsziel in Tagen/), {
      target: { value: "121" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      await screen.findByText("Bitte eine ganze Zahl von 0 bis 120 eingeben."),
    ).toBeInTheDocument();
    expect(saveServiceTerms).not.toHaveBeenCalled();
  });

  it("saves the meal on its own invoice", async () => {
    saveServiceTerms.mockResolvedValue(data);
    const { onChanged, table } = renderCard();
    fireEvent.click(table.getByText("Mittagessen"));
    fireEvent.click(screen.getByRole("button", { name: "Eigene Rechnung" }));
    fireEvent.change(screen.getByLabelText(/Zahlungsziel in Tagen/), {
      target: { value: "30" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() =>
      expect(saveServiceTerms).toHaveBeenCalledWith("meal", {
        paymentMethod: "transfer",
        dueDays: 30,
        bundleWith: "",
      }),
    );
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });
});

describe("parseDueDays", () => {
  it("accepts whole days from 0 to 120", () => {
    expect(parseDueDays("0")).toBe(0);
    expect(parseDueDays(" 120 ")).toBe(120);
    expect(parseDueDays("121")).toBeNull();
    expect(parseDueDays("1,5")).toBeNull();
    expect(parseDueDays("")).toBeNull();
  });
});
