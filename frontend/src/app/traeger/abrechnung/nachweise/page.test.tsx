import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  CareEvidence,
  CareEvidenceSet,
} from "~/lib/traeger-care-reductions-api";
import TraegerNachweisePage from "./page";

const { fetchCareEvidences } = vi.hoisted(() => ({
  fetchCareEvidences: vi.fn(),
}));
vi.mock("~/lib/traeger-care-reductions-api", () => ({
  fetchCareEvidences,
  deleteCareEvidence: vi.fn(),
  saveCareEvidence: vi.fn(),
}));
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  usePathname: () => "/abrechnung/nachweise",
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

const evidence = (
  id: string,
  from: string,
  until: string | null,
): CareEvidence => ({
  id,
  contractId: "1",
  kind: "single_parent",
  ruleId: null,
  evidence: `Erklärung ${id}`,
  checkedOn: "2026-01-01",
  validFrom: from,
  validUntil: until,
});
const data: CareEvidenceSet = {
  levels: [],
  contracts: [
    {
      contractId: "1",
      schoolId: "11",
      schoolName: "Schule",
      childFirstName: "Mia",
      childLastName: "Beispiel",
      evidences: [
        evidence("old", "2025-01-01", "2025-12-31"),
        evidence("now", "2026-01-01", "2026-12-31"),
        evidence("next", "2027-01-01", "2027-12-31"),
        evidence("later", "2028-01-01", null),
      ],
    },
  ],
};

function renderPage() {
  fetchCareEvidences.mockResolvedValue(data);
  render(
    <ToastProvider>
      <TraegerNachweisePage />
    </ToastProvider>,
  );
}

describe("TraegerNachweisePage", () => {
  it("shows every period and makes expired and future evidence editable", async () => {
    renderPage();
    for (const id of ["old", "now", "next", "later"]) {
      expect(
        (await screen.findAllByText(new RegExp(`Erklärung ${id}`)))[0],
      ).toBeInTheDocument();
    }
    fireEvent.click(
      screen.getAllByRole("button", { name: "Nachweise: Mia Beispiel" })[0]!,
    );
    fireEvent.click(
      screen.getByRole("menuitem", {
        name: "Alleinerziehend ändern",
        description: /01\.01\.2025/,
      }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByDisplayValue("Erklärung old"),
    ).toBeInTheDocument();
  });

  it("offers a separate new evidence action despite an existing current period", async () => {
    renderPage();
    await screen.findAllByText(/Erklärung now/);
    fireEvent.click(
      screen.getAllByRole("button", { name: "Nachweise: Mia Beispiel" })[0]!,
    );
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Alleinerziehend eintragen" }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByRole("button", { name: "Nachweis speichern" }),
    ).toBeInTheDocument();
    expect(within(dialog).getByLabelText(/Geprüfter Nachweis/)).toHaveValue("");
  });
});
