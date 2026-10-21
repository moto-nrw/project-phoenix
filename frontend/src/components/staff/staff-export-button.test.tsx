import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { StaffExportButton } from "./staff-export-button";

// The export form starts the download by navigating to the proxy URL. The stub
// keeps happy-dom from navigating and records the target instead.
const location = { href: "" };

beforeEach(() => {
  location.href = "";
  vi.stubGlobal("location", location);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("StaffExportButton", () => {
  it("exports the time tracking of the given staff member", () => {
    render(<StaffExportButton staffId="7" yearStart={new Date(2026, 0, 1)} />);

    fireEvent.click(screen.getByRole("button", { name: "Menü öffnen" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Exportieren" }));
    fireEvent.click(screen.getByRole("button", { name: "Excel" }));

    expect(location.href).toMatch(
      /^\/api\/staff\/7\/time-tracking\/export\?from=\d{4}-\d{2}-\d{2}&to=\d{4}-\d{2}-\d{2}&format=xlsx$/,
    );
    // The form closes after it started the download.
    expect(screen.queryByText("Zeitraum exportieren")).not.toBeInTheDocument();
  });
});
