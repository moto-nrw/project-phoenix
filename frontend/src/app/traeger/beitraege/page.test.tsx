import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type {
  ContributionRule,
  CostBearer,
} from "~/lib/traeger-contributions-api";
import { catalogText } from "~/test/error-catalog-text";

const { api } = vi.hoisted(() => ({
  api: {
    fetchContributionRules: vi.fn(),
    createContributionRule: vi.fn(),
    updateContributionRule: vi.fn(),
    deleteContributionRule: vi.fn(),
    fetchCostBearers: vi.fn(),
    saveCostBearer: vi.fn(),
    deleteCostBearer: vi.fn(),
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

const YOUTH_OFFICE: CostBearer = {
  id: "4",
  name: "Fachdienst Jugend des Landkreises Musterstadt",
  address: "Am Kreishaus 1\n12345 Musterstadt",
  fileReference: "FJ 51-2027",
  active: true,
  inUse: true,
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
    api.fetchCostBearers.mockResolvedValue([]);
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
        "Keine Befreiungsregel. Alle Familien zahlen, auch bei Leistungsbezug.",
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

    expect(
      screen.getAllByText(
        "Die Regel gilt für alle Familien mit Leistungsbezug. Welche Familie befreit ist, wählen Sie beim Bestätigen der Anmeldung. Befreite Familien zahlen nichts.",
      )[0],
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Befreiungsregel hinzufügen" }),
    );
    expect(
      await screen.findByRole("heading", {
        name: "Befreiungsregel hinzufügen",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Die Regel gilt für alle Familien mit Leistungsbezug. Welche Familie befreit ist, wählen Sie beim Bestätigen der Anmeldung.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByLabelText("Vermerk auf dem Beitrag"),
    ).toHaveAccessibleDescription(
      "Der Vermerk steht beim Beitrag jeder befreiten Familie.",
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
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

  it("adds a cost bearer and asks for its name", async () => {
    api.saveCostBearer.mockResolvedValue(YOUTH_OFFICE);
    renderPage();
    expect(
      (
        await screen.findAllByText(
          "Kein Kostenträger. Beim Bestätigen bezahlt die Familie oder ist befreit.",
        )
      )[0],
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Kostenträger hinzufügen" }),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));
    expect(
      await screen.findByText("Bitte geben Sie den Namen ein."),
    ).toBeInTheDocument();
    expect(api.saveCostBearer).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText(/^Name/), {
      target: { value: YOUTH_OFFICE.name },
    });
    fireEvent.change(
      screen.getByLabelText("Ihr Aktenzeichen beim Kostenträger (freiwillig)"),
      { target: { value: "FJ 51-2027" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(api.saveCostBearer).toHaveBeenCalledWith(null, {
        name: YOUTH_OFFICE.name,
        address: "",
        fileReference: "FJ 51-2027",
        active: true,
      }),
    );
    await waitFor(() => expect(api.fetchCostBearers).toHaveBeenCalledTimes(2));
  });

  it("only switches off a cost bearer a contribution names", async () => {
    api.fetchCostBearers.mockResolvedValue([YOUTH_OFFICE]);
    api.saveCostBearer.mockResolvedValue({ ...YOUTH_OFFICE, active: false });
    renderPage();
    const name = (await screen.findAllByText(YOUTH_OFFICE.name))[0];
    expect(name).toHaveClass("wrap-anywhere");

    fireEvent.click(
      screen.getAllByRole("button", {
        name: `Aktionen für ${YOUTH_OFFICE.name}`,
      })[0]!,
    );
    expect(
      await screen.findByRole("menuitem", { name: "Ausschalten" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "Ändern" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "Löschen" })).toBeNull();
    fireEvent.click(screen.getByRole("menuitem", { name: "Ausschalten" }));

    await waitFor(() =>
      expect(api.saveCostBearer).toHaveBeenCalledWith("4", {
        ...YOUTH_OFFICE,
        active: false,
      }),
    );
  });

  it("changes a cost bearer in its row, one row at a time, without a dialog", async () => {
    const social: CostBearer = {
      id: "5",
      name: "Sozialamt Musterstadt",
      address: "",
      fileReference: "",
      active: true,
      inUse: false,
    };
    api.fetchCostBearers.mockResolvedValue([YOUTH_OFFICE, social]);
    api.saveCostBearer.mockResolvedValue(YOUTH_OFFICE);
    renderPage();
    await screen.findAllByText(YOUTH_OFFICE.name);

    const openChange = async (name: string) => {
      fireEvent.click(
        screen.getAllByRole("button", { name: `Aktionen für ${name}` })[0]!,
      );
      fireEvent.click(await screen.findByRole("menuitem", { name: "Ändern" }));
    };

    await openChange(YOUTH_OFFICE.name);
    const form = await screen.findByRole("form", {
      name: `${YOUTH_OFFICE.name} ändern`,
    });
    expect(form.closest("li")).not.toBeNull();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByDisplayValue("FJ 51-2027")).toBeInTheDocument();
    expect(screen.getByText("Sozialamt Musterstadt")).toBeInTheDocument();

    // Opening another row closes the first one.
    await openChange(social.name);
    expect(
      await screen.findByRole("form", { name: "Sozialamt Musterstadt ändern" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("form", { name: `${YOUTH_OFFICE.name} ändern` }),
    ).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(
      screen.queryByRole("form", { name: "Sozialamt Musterstadt ändern" }),
    ).toBeNull();
    expect(screen.getByText("Sozialamt Musterstadt")).toBeInTheDocument();

    await openChange(YOUTH_OFFICE.name);
    const name = await screen.findByDisplayValue(YOUTH_OFFICE.name);
    fireEvent.change(name, { target: { value: " " } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      await screen.findByText("Bitte geben Sie den Namen ein."),
    ).toBeInTheDocument();
    expect(name).toHaveAttribute("aria-invalid", "true");
    expect(api.saveCostBearer).not.toHaveBeenCalled();

    fireEvent.change(name, { target: { value: "Jugendamt Musterstadt" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() =>
      expect(api.saveCostBearer).toHaveBeenCalledWith(
        "4",
        expect.objectContaining({
          name: "Jugendamt Musterstadt",
          address: YOUTH_OFFICE.address,
          fileReference: "FJ 51-2027",
          active: true,
        }),
      ),
    );
    await waitFor(() => expect(api.fetchCostBearers).toHaveBeenCalledTimes(2));
    expect(
      screen.queryByRole("form", { name: "Jugendamt Musterstadt ändern" }),
    ).toBeNull();
  });
});
