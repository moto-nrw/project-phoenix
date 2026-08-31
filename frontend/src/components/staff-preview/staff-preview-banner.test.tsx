import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { StaffPreviewBanner } from "./staff-preview-banner";

const mockShellAuth = vi.hoisted(() => ({
  value: undefined as
    | {
        isPreview?: boolean;
        previewTargetName?: string;
        previewTargetAccountId?: number;
      }
    | undefined,
}));

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuthSafe: () => mockShellAuth.value,
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ update: vi.fn() }),
}));

describe("StaffPreviewBanner", () => {
  it("renders nothing outside a shell provider", () => {
    mockShellAuth.value = undefined;
    const { container } = render(<StaffPreviewBanner />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing while no preview is active", () => {
    mockShellAuth.value = { isPreview: false };
    const { container } = render(<StaffPreviewBanner />);
    expect(container).toBeEmptyDOMElement();
  });

  it("names the previewed person and offers the exit", () => {
    mockShellAuth.value = {
      isPreview: true,
      previewTargetName: "Erika Beispiel",
      previewTargetAccountId: 42,
    };
    render(<StaffPreviewBanner />);

    expect(screen.getAllByText("Erika Beispiel").length).toBeGreaterThan(0);
    expect(
      screen.getByRole("button", { name: "Vorschau beenden" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("status")).toBeInTheDocument();
  });
});
