import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
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

const mockPush = vi.fn();
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mockPush }),
}));

describe("HistoryLinks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders Historien title", () => {
    render(
      <HistoryLinks
        studentId="42"
        attendanceLogEnabled={false}
        feedbackEnabled={false}
      />,
    );
    expect(screen.getByText("Historien")).toBeInTheDocument();
  });

  it("renders all three history link buttons", () => {
    render(
      <HistoryLinks
        studentId="42"
        attendanceLogEnabled={true}
        feedbackEnabled={true}
      />,
    );

    expect(screen.getByText("Anwesenheitsprotokoll")).toBeInTheDocument();
    expect(screen.getByText("Feedbackhistorie")).toBeInTheDocument();
    expect(screen.getByText("Mensaverlauf")).toBeInTheDocument();
  });

  it("mensa button remains disabled", () => {
    render(
      <HistoryLinks
        studentId="42"
        attendanceLogEnabled={true}
        feedbackEnabled={true}
      />,
    );

    expect(screen.getByText("Mensaverlauf").closest("button")).toBeDisabled();
  });

  it("raumverlauf button is enabled when attendanceLogEnabled is true", () => {
    render(
      <HistoryLinks
        studentId="42"
        attendanceLogEnabled={true}
        feedbackEnabled={false}
      />,
    );

    const raumButton = screen
      .getByText("Anwesenheitsprotokoll")
      .closest("button");
    expect(raumButton).not.toBeDisabled();
    expect(
      screen.getByText("Anwesenheit und besuchte Räume"),
    ).toBeInTheDocument();
  });

  it("raumverlauf button is disabled when attendanceLogEnabled is false", () => {
    render(
      <HistoryLinks
        studentId="42"
        attendanceLogEnabled={false}
        feedbackEnabled={false}
      />,
    );

    const raumButton = screen
      .getByText("Anwesenheitsprotokoll")
      .closest("button");
    expect(raumButton).toBeDisabled();
  });

  it("feedback button is enabled when feedbackEnabled is true", () => {
    render(
      <HistoryLinks
        studentId="42"
        attendanceLogEnabled={false}
        feedbackEnabled={true}
      />,
    );

    const feedbackButton = screen
      .getByText("Feedbackhistorie")
      .closest("button");
    expect(feedbackButton).not.toBeDisabled();
    expect(screen.getByText("Feedback und Bewertungen")).toBeInTheDocument();
  });

  it("feedback button is disabled when feedbackEnabled is false", () => {
    render(
      <HistoryLinks
        studentId="42"
        attendanceLogEnabled={false}
        feedbackEnabled={false}
      />,
    );

    const feedbackButton = screen
      .getByText("Feedbackhistorie")
      .closest("button");
    expect(feedbackButton).toBeDisabled();
  });

  it("shows deaktiviert text for disabled features", () => {
    render(
      <HistoryLinks
        studentId="42"
        attendanceLogEnabled={false}
        feedbackEnabled={false}
      />,
    );

    const deactivatedTexts = screen.getAllByText("Für Ihre Schule deaktiviert");
    expect(deactivatedTexts).toHaveLength(2);
  });

  it("navigates to feedback history when feedback button is clicked", () => {
    render(
      <HistoryLinks
        studentId="42"
        attendanceLogEnabled={false}
        feedbackEnabled={true}
      />,
    );

    const feedbackButton = screen
      .getByText("Feedbackhistorie")
      .closest("button");
    fireEvent.click(feedbackButton!);

    expect(mockPush).toHaveBeenCalledWith("/students/42/feedback_history");
  });
});
