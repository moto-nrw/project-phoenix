import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { HistoryLinks } from "./HistoryLinks";

vi.mock("~/components/ui/info-card", () => ({
  InfoCard: ({
    title,
    children,
  }: {
    title: string;
    children: React.ReactNode;
  }) => (
    <div data-testid="info-card">
      <h2>{title}</h2>
      {children}
    </div>
  ),
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn() }),
}));

describe("HistoryLinks", () => {
  it("renders Historien title", () => {
    render(<HistoryLinks studentId="42" attendanceLogEnabled={false} />);
    expect(screen.getByText("Historien")).toBeInTheDocument();
  });

  it("renders all three history link buttons", () => {
    render(<HistoryLinks studentId="42" attendanceLogEnabled={true} />);

    expect(screen.getByText("Anwesenheitsprotokoll")).toBeInTheDocument();
    expect(screen.getByText("Feedbackhistorie")).toBeInTheDocument();
    expect(screen.getByText("Mensaverlauf")).toBeInTheDocument();
  });

  it("feedback and mensa buttons remain disabled", () => {
    render(<HistoryLinks studentId="42" attendanceLogEnabled={true} />);

    expect(
      screen.getByText("Feedbackhistorie").closest("button"),
    ).toBeDisabled();
    expect(screen.getByText("Mensaverlauf").closest("button")).toBeDisabled();
  });

  it("raumverlauf button is enabled when attendanceLogEnabled is true", () => {
    render(<HistoryLinks studentId="42" attendanceLogEnabled={true} />);

    const raumButton = screen
      .getByText("Anwesenheitsprotokoll")
      .closest("button");
    expect(raumButton).not.toBeDisabled();
    expect(
      screen.getByText("Anwesenheit und besuchte Räume"),
    ).toBeInTheDocument();
  });

  it("raumverlauf button is disabled when attendanceLogEnabled is false", () => {
    render(<HistoryLinks studentId="42" attendanceLogEnabled={false} />);

    const raumButton = screen
      .getByText("Anwesenheitsprotokoll")
      .closest("button");
    expect(raumButton).toBeDisabled();
    expect(screen.getByText("Für Ihre Schule deaktiviert")).toBeInTheDocument();
  });
});
