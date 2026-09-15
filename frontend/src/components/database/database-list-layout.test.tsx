import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DatabaseListLayout } from "./database-list-layout";

describe("DatabaseListLayout", () => {
  it("renders the list inside one full-width card", () => {
    render(
      <DatabaseListLayout>
        <ul>
          <li>Mia Fischer</li>
        </ul>
      </DatabaseListLayout>,
    );

    const root = screen.getByTestId("database-list-layout");
    expect(root.style.height).toContain("100dvh");
    expect(screen.getByText("Mia Fischer")).toBeInTheDocument();
    expect(root.querySelector(".moto-content-surface")).not.toBeNull();
  });

  it("applies a custom className to the root", () => {
    render(
      <DatabaseListLayout className="custom-list">
        <div>Inhalt</div>
      </DatabaseListLayout>,
    );

    expect(screen.getByTestId("database-list-layout")).toHaveClass(
      "custom-list",
    );
  });
});
