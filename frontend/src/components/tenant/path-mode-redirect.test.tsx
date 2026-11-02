import { afterEach, describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";
import { PathModeRedirect } from "./path-mode-redirect";

const originalLocation = window.location;

afterEach(() => {
  Object.defineProperty(window, "location", {
    writable: true,
    value: originalLocation,
  });
});

describe("PathModeRedirect (#5130)", () => {
  // A display link carries its token in the fragment, which only the
  // browser knows; a server redirect in a streamed answer dropped it.
  it("keeps the fragment of a display link", () => {
    const replace = vi.fn();
    Object.defineProperty(window, "location", {
      writable: true,
      value: { hash: "#token-123", replace },
    });

    render(<PathModeRedirect href="https://school-a.moto-app.de/display" />);

    expect(replace).toHaveBeenCalledWith(
      "https://school-a.moto-app.de/display#token-123",
    );
  });
});
