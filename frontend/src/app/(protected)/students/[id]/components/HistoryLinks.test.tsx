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

describe("HistoryLinks", () => {
  it("renders Historien title", () => {
    render(<HistoryLinks />);
    expect(screen.getByText("Historien")).toBeInTheDocument();
  });

  it("renders all three history link buttons", () => {
    render(<HistoryLinks />);

    expect(screen.getByText("Raumverlauf")).toBeInTheDocument();
    expect(screen.getByText("Feedbackhistorie")).toBeInTheDocument();
    expect(screen.getByText("Mensaverlauf")).toBeInTheDocument();
  });

  it("renders subtitles for each button", () => {
    render(<HistoryLinks />);

    expect(screen.getByText("Verlauf der Raumbesuche")).toBeInTheDocument();
    expect(screen.getByText("Feedback und Bewertungen")).toBeInTheDocument();
    expect(screen.getByText("Mahlzeiten und Bestellungen")).toBeInTheDocument();
  });

  it("all buttons are disabled", () => {
    render(<HistoryLinks />);

    const buttons = screen.getAllByRole("button");
    buttons.forEach((button) => {
      expect(button).toBeDisabled();
    });
  });
});
