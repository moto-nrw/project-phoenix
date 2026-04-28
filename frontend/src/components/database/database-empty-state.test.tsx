import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DatabaseEmptyState } from "./database-empty-state";

describe("DatabaseEmptyState", () => {
  it("renders the provided icon, title, and description", () => {
    render(
      <DatabaseEmptyState
        icon={<svg data-testid="empty-icon" />}
        title="Keine Geräte vorhanden"
        description="Es wurden noch keine Geräte registriert."
      />,
    );

    expect(screen.getByTestId("empty-icon")).toBeInTheDocument();
    expect(screen.getByText("Keine Geräte vorhanden")).toBeInTheDocument();
    expect(
      screen.getByText("Es wurden noch keine Geräte registriert."),
    ).toBeInTheDocument();
  });
});
