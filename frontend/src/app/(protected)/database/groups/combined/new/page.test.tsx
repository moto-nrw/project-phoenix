import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import NewCombinedGroupPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn() }),
}));

vi.mock("@/lib/api", () => ({
  combinedGroupService: {
    createCombinedGroup: vi.fn(),
  },
}));

vi.mock("@/components/dashboard", () => ({
  PageHeader: ({ title }: { title: string }) => (
    <div data-testid="page-header">{title}</div>
  ),
}));

vi.mock("@/components/groups", () => ({
  CombinedGroupForm: ({
    formTitle,
    submitLabel,
  }: {
    formTitle: string;
    submitLabel: string;
  }) => (
    <div data-testid="combined-group-form">
      <span>{formTitle}</span>
      <button>{submitLabel}</button>
    </div>
  ),
}));

describe("NewCombinedGroupPage", () => {
  it("renders page header with correct title", () => {
    render(<NewCombinedGroupPage />);

    expect(screen.getByTestId("page-header")).toHaveTextContent(
      "Neue Gruppenkombination",
    );
  });

  it("renders combined group form", () => {
    render(<NewCombinedGroupPage />);

    expect(screen.getByTestId("combined-group-form")).toBeInTheDocument();
    expect(
      screen.getByText("Gruppenkombination erstellen"),
    ).toBeInTheDocument();
    expect(screen.getByText("Erstellen")).toBeInTheDocument();
  });
});
