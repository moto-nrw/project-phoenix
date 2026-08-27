import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useEmergencyHealthInfoEnabled } from "~/lib/tenant-context";
import EmergencyPage from "./page";

const mockUseSession = vi.fn();
const mockExportEmergencySnapshot = vi.fn();

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

// Nur die beiden Symbole der Seite bekommen ein Testkennzeichen; alle
// weiteren Symbole zieht das Seitengerüst über das UI-Kit nach und braucht
// dafür das echte Paket.
vi.mock("lucide-react", async (importOriginal) => ({
  ...(await importOriginal<typeof import("lucide-react")>()),
  Download: (props: Record<string, unknown>) => (
    <svg data-testid="download-icon" {...props} />
  ),
  Printer: (props: Record<string, unknown>) => (
    <svg data-testid="printer-icon" {...props} />
  ),
}));

beforeEach(() => {
  mockUseSession.mockReturnValue({ status: "authenticated" });
  mockExportEmergencySnapshot.mockResolvedValue(undefined);
  vi.mocked(useEmergencyHealthInfoEnabled).mockReturnValue(true);
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

  it("shows an error when export fails", async () => {
    mockExportEmergencySnapshot.mockRejectedValueOnce(new Error("failed"));
    render(<EmergencyPage />);

    fireEvent.click(
      screen.getByRole("button", { name: /Notfallliste drucken/ }),
    );

    expect(
      await screen.findByText(
        "Die Notfallliste konnte nicht erstellt werden. Bitte versuchen Sie es erneut.",
      ),
    ).toBeInTheDocument();
  });

  // The page is the only place staff read before printing, so it has to name
  // the health column — and warn that a missing entry is not an all-clear
  // (#2609).
  it("names the health info and the Nicht-hinterlegt caveat when the column is on", () => {
    render(<EmergencyPage />);

    expect(
      screen.getByText(/und die hinterlegten Gesundheitsinfos/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Das heißt nicht, dass das Kind keine Allergie hat/),
    ).toBeInTheDocument();
  });

  // A school that switched the column off must not be promised health data
  // the PDF does not carry.
  it("omits the health info copy when the school switched the column off", () => {
    vi.mocked(useEmergencyHealthInfoEnabled).mockReturnValue(false);

    render(<EmergencyPage />);

    expect(screen.queryByText(/Gesundheitsinfos/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Nicht hinterlegt/)).not.toBeInTheDocument();
    // The list itself is still offered.
    expect(
      screen.getByRole("button", { name: /Notfallliste drucken/ }),
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
