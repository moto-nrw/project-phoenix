import {
  fireEvent,
  render as rtlRender,
  screen,
  waitFor,
} from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import EmergencyPage from "./page";

const mockUseSession = vi.fn();
const mockExportEmergencySnapshot = vi.fn();

function render(ui: ReactElement) {
  return rtlRender(ui, { wrapper: ToastProvider });
}

vi.mock("next-auth/react", () => ({
  useSession: () => mockUseSession(),
}));

vi.mock("~/lib/emergency-export-api", () => ({
  exportEmergencySnapshot: (mode: string) => mockExportEmergencySnapshot(mode),
}));

vi.mock("~/components/ui/moto-concept-icon", () => ({
  MotoConceptIcon: ({ concept }: { concept: string }) => (
    <svg data-testid="concept-icon" data-concept={concept} />
  ),
}));

vi.mock("~/components/ui/button", () => ({
  Button: ({
    children,
    isLoading,
    loadingText,
    ...props
  }: {
    children: React.ReactNode;
    isLoading?: boolean;
    loadingText?: string;
    [key: string]: unknown;
  }) => (
    <button
      type="button"
      {...props}
      disabled={Boolean(props.disabled) || isLoading}
    >
      {isLoading ? loadingText : children}
    </button>
  ),
}));

vi.mock("lucide-react", async (importOriginal) => {
  const actual = await importOriginal<typeof import("lucide-react")>();
  return {
    ...actual,
    Download: (props: Record<string, unknown>) => (
      <svg data-testid="download-icon" {...props} />
    ),
    Printer: (props: Record<string, unknown>) => (
      <svg data-testid="printer-icon" {...props} />
    ),
  };
});

beforeEach(() => {
  mockUseSession.mockReturnValue({ status: "authenticated" });
  mockExportEmergencySnapshot.mockReset();
  mockExportEmergencySnapshot.mockResolvedValue(undefined);
});

describe("EmergencyPage", () => {
  it("renders the emergency list actions", () => {
    render(<EmergencyPage />);

    expect(
      screen.getByRole("heading", { name: "Notfallliste" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Notfallliste drucken/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /PDF herunterladen/ }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("concept-icon")).toHaveAttribute(
      "data-concept",
      "emergency",
    );
  });

  it("prints the emergency snapshot", async () => {
    render(<EmergencyPage />);

    fireEvent.click(
      screen.getByRole("button", { name: /Notfallliste drucken/ }),
    );

    await waitFor(() => {
      expect(mockExportEmergencySnapshot).toHaveBeenCalledWith("print");
    });
  });

  it("downloads the emergency snapshot", async () => {
    render(<EmergencyPage />);

    fireEvent.click(screen.getByRole("button", { name: /PDF herunterladen/ }));

    await waitFor(() => {
      expect(mockExportEmergencySnapshot).toHaveBeenCalledWith("download");
    });
  });

  it("shows the catalog text with retry in a toast when export fails", async () => {
    mockExportEmergencySnapshot.mockRejectedValueOnce(
      new ApiError("boom", 500, { code: "general.server" }),
    );
    render(<EmergencyPage />);

    fireEvent.click(
      screen.getByRole("button", { name: /Notfallliste drucken/ }),
    );

    expect(
      await screen.findByText(
        catalogText("general.server", "die Notfallliste"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => {
      expect(mockExportEmergencySnapshot).toHaveBeenCalledTimes(2);
    });
    expect(mockExportEmergencySnapshot).toHaveBeenLastCalledWith("print");
  });

  // The page is the only place staff read before printing, so it has to name
  // the health column — and warn that a missing entry is not an all-clear
  // (#2609).
  it("names the health info and the Nicht-hinterlegt caveat when the column is on", () => {
    render(<EmergencyPage />);

    expect(
      screen.getByText(/die\s+hinterlegten Gesundheitsinfos/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Das heißt nicht, dass das Kind keine Allergie hat/),
    ).toBeInTheDocument();
  });

  it("shows loading while auth resolves", () => {
    mockUseSession.mockReturnValue({ status: "loading" });

    render(<EmergencyPage />);

    expect(
      screen.getByLabelText("Notfallliste wird geladen…"),
    ).toBeInTheDocument();
  });
});
