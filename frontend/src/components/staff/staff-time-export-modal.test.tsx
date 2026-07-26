import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { StaffTimeExportModal } from "./staff-time-export-modal";

// Der Dialog baut nur die Query — die Tests pinnen genau das: welche Parameter
// bei welcher Auswahl an die Streaming-Proxy-Route gehen (#1417 2b).
describe("StaffTimeExportModal", () => {
  const originalLocation = globalThis.location;

  beforeEach(() => {
    Object.defineProperty(globalThis, "location", {
      value: { href: "" },
      writable: true,
    });
  });

  afterEach(() => {
    globalThis.location = originalLocation;
  });

  function renderModal(onClose = () => undefined) {
    return render(
      <StaffTimeExportModal
        isOpen={true}
        onClose={onClose}
        year={2026}
        month={6}
      />,
    );
  }

  it("exports the displayed month as CSV with hh:mm by default", () => {
    renderModal();
    fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));

    expect(globalThis.location.href).toBe(
      "/api/staff/time-tracking/export?year=2026&format=csv&month=6&granularity=month&time_format=hhmm",
    );
  });

  it("exports the whole year in decimal Excel when selected", () => {
    renderModal();
    fireEvent.click(screen.getByRole("button", { name: "Gesamtes Jahr 2026" }));
    fireEvent.click(screen.getByRole("button", { name: "Excel" }));
    fireEvent.click(screen.getByRole("button", { name: "Dezimalstunden" }));
    fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));

    expect(globalThis.location.href).toBe(
      "/api/staff/time-tracking/export?year=2026&format=xlsx&granularity=month&time_format=decimal",
    );
  });

  it("drops time_format and disables the toggle for the day granularity", () => {
    renderModal();
    fireEvent.click(screen.getByRole("button", { name: "Einzelne Tage" }));

    expect(
      screen.getByRole("button", { name: "Dezimalstunden" }),
    ).toBeDisabled();
    expect(
      screen.getByText(/Zeitformat gilt nur für Monatssummen/),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));
    expect(globalThis.location.href).toBe(
      "/api/staff/time-tracking/export?year=2026&format=csv&month=6&granularity=day",
    );
  });

  it("closes after triggering the download", () => {
    let closed = false;
    renderModal(() => {
      closed = true;
    });
    fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));
    expect(closed).toBe(true);
  });
});
