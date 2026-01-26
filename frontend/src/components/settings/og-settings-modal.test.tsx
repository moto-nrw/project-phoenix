import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { OGSettingsModal } from "./og-settings-modal";

// Mock Modal component
vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    title,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
  }) => (isOpen ? <div data-testid="modal" data-title={title}>{children}</div> : null),
}));

// Mock OGSettingsPanel
vi.mock("./og-settings-panel", () => ({
  OGSettingsPanel: ({
    ogId,
    showHistory,
  }: {
    ogId: string;
    showHistory?: boolean;
  }) => (
    <div
      data-testid="og-settings-panel"
      data-ogid={ogId}
      data-show-history={showHistory}
    />
  ),
}));

describe("OGSettingsModal", () => {
  it("renders nothing when closed", () => {
    render(
      <OGSettingsModal
        isOpen={false}
        onClose={vi.fn()}
        ogId="5"
        ogName="OG West"
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders modal with correct title when open", () => {
    render(
      <OGSettingsModal
        isOpen={true}
        onClose={vi.fn()}
        ogId="5"
        ogName="OG West"
      />,
    );

    const modal = screen.getByTestId("modal");
    expect(modal).toBeInTheDocument();
    expect(modal).toHaveAttribute("data-title", "Einstellungen: OG West");
  });

  it("passes ogId and showHistory to OGSettingsPanel", () => {
    render(
      <OGSettingsModal
        isOpen={true}
        onClose={vi.fn()}
        ogId="42"
        ogName="OG Ost"
      />,
    );

    const panel = screen.getByTestId("og-settings-panel");
    expect(panel).toHaveAttribute("data-ogid", "42");
    expect(panel).toHaveAttribute("data-show-history", "true");
  });
});
