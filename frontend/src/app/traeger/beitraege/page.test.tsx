import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { ContributionRule } from "~/lib/traeger-contributions-api";
import { catalogText } from "~/test/error-catalog-text";

const { api } = vi.hoisted(() => ({
  api: {
    fetchContributionRules: vi.fn(),
    createContributionRule: vi.fn(),
    updateContributionRule: vi.fn(),
    deleteContributionRule: vi.fn(),
  },
}));

vi.mock("~/lib/traeger-contributions-api", () => api);

import TraegerBeitraegePage from "./page";

const SIBLING: ContributionRule = {
  id: "3",
  kind: "sibling",
  validFrom: "2026-08-01",
  validUntil: null,
  siblingFromChild: 2,
  discountBasisPoints: 5000,
  discountCents: null,
  note: "",
};

function renderPage() {
  render(
    <ToastProvider>
      <TraegerBeitraegePage />
    </ToastProvider>,
  );
}

describe("TraegerBeitraegePage", () => {
  beforeEach(() => {
    for (const fn of Object.values(api)) fn.mockReset();
    api.fetchContributionRules.mockResolvedValue([SIBLING]);
  });

  it("lists the rules by kind and names the kind without a rule", async () => {
    renderPage();

    expect(
      (await screen.findAllByText("ab 01.08.2026"))[0],
    ).toBeInTheDocument();
    expect(
      screen.getAllByText("ab dem 2. Kind 50 % weniger")[0],
    ).toBeInTheDocument();
    expect(
      screen.getAllByText(
        "Keine Befreiung. Ohne Regel zahlen alle Familien.",
      )[0],
    ).toBeInTheDocument();
  });

  it("adds a sibling rule with a share in basis points", async () => {
    api.createContributionRule.mockResolvedValue(SIBLING);
    renderPage();
    await screen.findAllByText("ab 01.08.2026");

    fireEvent.click(
      screen.getByRole("button", { name: "Geschwisterregel hinzufügen" }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "ab dem 3. Kind" }),
    );
    fireEvent.change(screen.getByLabelText("Anteil in Prozent"), {
      target: { value: "33,5" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(api.createContributionRule).toHaveBeenCalled());
    expect(api.createContributionRule).toHaveBeenCalledWith(
      expect.objectContaining({
        kind: "sibling",
        siblingFromChild: 3,
        discountBasisPoints: 3350,
        discountCents: null,
        validFrom: "2026-09-09",
        validUntil: null,
      }),
    );
    await waitFor(() =>
      expect(api.fetchContributionRules).toHaveBeenCalledTimes(2),
    );
  });

  it("asks for the note of an exemption and shows an overlap at the form", async () => {
    api.createContributionRule.mockRejectedValue(
      new ApiError("overlap", 409, {
        code: "parent_billing.contribution_rule_overlap",
      }),
    );
    renderPage();
    await screen.findAllByText("ab 01.08.2026");

    fireEvent.click(
      screen.getByRole("button", { name: "Befreiung hinzufügen" }),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));
    expect(
      await screen.findByText("Bitte geben Sie den Vermerk an."),
    ).toBeInTheDocument();
    expect(api.createContributionRule).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Vermerk auf dem Beitrag"), {
      target: { value: "Beitragsfrei mit Nachweis" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      await screen.findByText(
        catalogText("parent_billing.contribution_rule_overlap", "die Regel"),
      ),
    ).toBeInTheDocument();
    expect(api.createContributionRule).toHaveBeenCalledWith(
      expect.objectContaining({
        kind: "benefits_exemption",
        note: "Beitragsfrei mit Nachweis",
        siblingFromChild: null,
      }),
    );
  });
});
