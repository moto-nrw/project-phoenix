import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { setTestClock } from "~/test/clock";
import { ExportRangeForm } from "./export-range-form";

// The form starts the download by navigating to the proxy URL. The stub keeps
// happy-dom from navigating and records the target instead.
const location = { href: "" };

beforeEach(() => {
  location.href = "";
  vi.stubGlobal("location", location);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("ExportRangeForm", () => {
  it("downloads the last 30 days as CSV by default and closes", () => {
    setTestClock("2026-03-15T12:00:00+01:00");
    const onDone = vi.fn();
    render(
      <ExportRangeForm
        exportUrl="/api/time-tracking/export"
        anchor={new Date(2026, 0, 1)}
        onDone={onDone}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "CSV" }));

    expect(location.href).toBe(
      "/api/time-tracking/export?from=2026-02-14&to=2026-03-15&format=csv",
    );
    expect(onDone).toHaveBeenCalledOnce();
  });

  it("requests the xlsx format for Excel with the chosen range", () => {
    render(
      <ExportRangeForm
        exportUrl="/api/staff/7/time-tracking/export"
        anchor={new Date(2026, 0, 1)}
        initialRange={{ from: new Date(2026, 4, 4), to: new Date(2026, 4, 8) }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Excel" }));

    expect(location.href).toBe(
      "/api/staff/7/time-tracking/export?from=2026-05-04&to=2026-05-08&format=xlsx",
    );
  });

  it("starts no download while the range is incomplete", () => {
    render(
      <ExportRangeForm
        exportUrl="/api/time-tracking/export"
        anchor={new Date(2026, 0, 1)}
        initialRange={{ from: new Date(2026, 4, 4), to: undefined }}
      />,
    );

    const csv = screen.getByRole("button", { name: "CSV" });
    expect(csv).toBeDisabled();
    fireEvent.click(csv);

    expect(location.href).toBe("");
  });
});
