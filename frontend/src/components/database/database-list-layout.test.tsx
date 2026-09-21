import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { DatabaseListLayout } from "./database-list-layout";

describe("DatabaseListLayout", () => {
  beforeEach(() => {
    vi.mocked(useIsMobile).mockReturnValue(false);
  });

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

  // #3330: Am Computer setzt useFillHeight die Höhe und die Liste scrollt in
  // der Karte. Den Browser-Nachbau prüft e2e/layout/database-layouts.spec.ts.
  it("takes the card out of the grow rule on a computer", () => {
    render(
      <DatabaseListLayout>
        <div>Inhalt</div>
      </DatabaseListLayout>,
    );

    const card = screen
      .getByTestId("database-list-layout")
      .querySelector(":scope > .moto-content-surface");
    expect(card).toHaveClass("moto-scroll-surface", "min-h-0", "flex-1");
  });

  it("lets the page scroll on a phone instead of fixing the card's height", () => {
    vi.mocked(useIsMobile).mockReturnValue(true);

    render(
      <DatabaseListLayout>
        <div>Inhalt</div>
      </DatabaseListLayout>,
    );

    const root = screen.getByTestId("database-list-layout");
    expect(root.style.height).toBe("");
    const card = root.querySelector(":scope > .moto-content-surface");
    expect(card).not.toBeNull();
    expect(card).not.toHaveClass("moto-scroll-surface");
  });
});
