import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DatabaseCreateAction } from "./database-create-action";

function stubMatchMedia(matches: boolean) {
  vi.stubGlobal(
    "matchMedia",
    vi.fn((query: string) => ({
      matches,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  );
}

describe("DatabaseCreateAction", () => {
  beforeEach(() => {
    stubMatchMedia(true);
    document.documentElement.style.removeProperty("--moto-floating-fab-offset");
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    document.documentElement.style.removeProperty("--moto-floating-fab-offset");
  });

  it("publishes stack clearance while its mobile FAB is visible", () => {
    const { container, unmount } = render(
      <div data-testid="header-action-slot">
        <DatabaseCreateAction
          label="Raum"
          ariaLabel="Raum erstellen"
          onClick={() => undefined}
        />
      </div>,
    );

    expect(
      screen.getAllByRole("button", { name: "Raum erstellen" }),
    ).toHaveLength(2);
    expect(window.matchMedia).toHaveBeenCalledWith("(max-width: 767px)");
    expect(
      document.documentElement.style.getPropertyValue(
        "--moto-floating-fab-offset",
      ),
    ).toBe("4.5rem");

    // The blurred TenantPage header is a containing block for `fixed`
    // descendants. The FAB must be portaled outside its action slot so its
    // viewport offsets remain stable while the header scrolls away.
    expect(container.querySelector("[data-icon-only]")).not.toBeInTheDocument();
    expect(document.body.querySelector("[data-icon-only]")).toHaveClass(
      "fixed",
      "right-4",
      "bottom-24",
    );

    unmount();
    expect(
      document.documentElement.style.getPropertyValue(
        "--moto-floating-fab-offset",
      ),
    ).toBe("");
  });

  it("keeps the desktop action without rendering a mobile FAB when requested", () => {
    render(
      <DatabaseCreateAction
        label="Raum"
        ariaLabel="Raum erstellen"
        showMobileFab={false}
        onClick={() => undefined}
      />,
    );

    expect(screen.getByRole("button", { name: "Raum erstellen" })).toHaveClass(
      "md:flex",
    );
    expect(document.body.querySelector("[data-icon-only]")).toBeNull();
    expect(
      document.documentElement.style.getPropertyValue(
        "--moto-floating-fab-offset",
      ),
    ).toBe("");
  });
});
