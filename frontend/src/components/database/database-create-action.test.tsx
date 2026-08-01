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
    const { unmount } = render(
      <DatabaseCreateAction
        label="Raum"
        ariaLabel="Raum erstellen"
        onClick={() => undefined}
      />,
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

    unmount();
    expect(
      document.documentElement.style.getPropertyValue(
        "--moto-floating-fab-offset",
      ),
    ).toBe("");
  });
});
