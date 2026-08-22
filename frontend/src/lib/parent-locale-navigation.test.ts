import { describe, it, expect, vi, afterEach } from "vitest";

import { reloadForLocaleChange } from "./parent-locale-navigation";

// The reload lives behind this seam so the locale switcher can be tested
// without jsdom tearing the document down mid-test.
describe("reloadForLocaleChange", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("reloads the current document", () => {
    const reload = vi.fn();
    vi.stubGlobal("window", { location: { reload } });

    reloadForLocaleChange();

    expect(reload).toHaveBeenCalledOnce();
  });
});
