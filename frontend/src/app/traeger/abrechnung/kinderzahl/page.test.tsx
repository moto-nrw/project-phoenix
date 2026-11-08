import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  FamilyChildren,
  FamilyChildrenSet,
} from "~/lib/traeger-family-children-api";
import TraegerKinderzahlPage from "./page";

const { fetchFamilyChildren, confirmFamilyChildren } = vi.hoisted(() => ({
  fetchFamilyChildren: vi.fn(),
  confirmFamilyChildren: vi.fn(),
}));
vi.mock("~/lib/traeger-family-children-api", () => ({
  fetchFamilyChildren,
  confirmFamilyChildren,
  removeFamilyChildren: vi.fn(),
}));
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  usePathname: () => "/abrechnung/kinderzahl",
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

const stated: FamilyChildren = {
  recipientFirstName: "Eva",
  recipientLastName: "Beispiel",
  recipientEmail: "eva@example.org",
  children: [
    {
      contractId: "31",
      schoolName: "OGS Birkenfeld",
      childFirstName: "Mia",
      childLastName: "Beispiel",
    },
  ],
  count: {
    id: "5",
    contractId: "31",
    statedChildren: 3,
    statedAt: "2026-09-05T08:00:00Z",
    confirmedChildren: null,
    validFrom: null,
    validUntil: null,
    confirmedAt: null,
  },
  status: "stated",
};
const confirmed: FamilyChildren = {
  recipientFirstName: "Jonas",
  recipientLastName: "Muster",
  recipientEmail: "jonas@example.org",
  children: [
    {
      contractId: "32",
      schoolName: "OGS Ispringen",
      childFirstName: "Lea",
      childLastName: "Muster",
    },
  ],
  count: {
    id: "6",
    contractId: "32",
    statedChildren: null,
    statedAt: null,
    confirmedChildren: 2,
    validFrom: "2026-08-01",
    validUntil: null,
    confirmedAt: "2026-08-02T08:00:00Z",
  },
  status: "confirmed",
};
const data: FamilyChildrenSet = {
  families: [stated, confirmed],
  rules: [{ id: "9", name: "Geschwister Birkenfeld" }],
};

function renderPage(set: FamilyChildrenSet = data) {
  fetchFamilyChildren.mockResolvedValue(set);
  render(
    <ToastProvider>
      <TraegerKinderzahlPage />
    </ToastProvider>,
  );
}

describe("TraegerKinderzahlPage", () => {
  it("lists only the parents' new statement under „Zu prüfen“ and confirms it", async () => {
    renderPage();
    const checks = (
      await screen.findByRole("heading", { name: "Zu prüfen" })
    ).closest("section") as HTMLElement;
    expect(within(checks).getByText("Eva Beispiel")).toBeInTheDocument();
    expect(within(checks).queryByText("Jonas Muster")).not.toBeInTheDocument();
    expect(
      within(checks).getByText(/Eltern geben 3 Kinder an/),
    ).toBeInTheDocument();

    confirmFamilyChildren.mockResolvedValue({
      ...data,
      families: [{ ...stated, status: "confirmed" }, confirmed],
    });
    fireEvent.click(
      within(checks).getByRole("button", {
        name: "Kinderzahl von Eva Beispiel prüfen",
      }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText(/Kinder in der Familie/)).toHaveValue(
      "3",
    );
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Kinderzahl bestätigen" }),
    );
    await vi.waitFor(() =>
      expect(confirmFamilyChildren).toHaveBeenCalledWith({
        contractId: "31",
        children: 3,
        validFrom: "2026-09-01",
        validUntil: null,
      }),
    );
  });

  it("says when no sibling rule counts the number yet", async () => {
    renderPage({ ...data, rules: [] });
    expect(
      await screen.findByText(/Noch keine Geschwisterregel zählt diese Zahl/),
    ).toBeInTheDocument();
  });
});
