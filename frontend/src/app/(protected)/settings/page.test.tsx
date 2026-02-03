/* eslint-disable @typescript-eslint/no-unsafe-return, @next/next/no-img-element */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import SettingsPage from "./page";

/**
 * Radix UI components rely on pointer events for activation, not just click.
 * happy-dom's fireEvent.click doesn't fire the pointer event chain that Radix expects.
 * This helper simulates the full pointer interaction sequence.
 */
function clickRadixTrigger(element: HTMLElement) {
  fireEvent.mouseDown(element, { button: 0 });
  fireEvent.mouseUp(element, { button: 0 });
  fireEvent.click(element);
}

// Mock next-auth
const mockUseSession = vi.fn();
vi.mock("next-auth/react", () => ({
  useSession: () => mockUseSession(),
}));

// Mock next/navigation
const mockRedirect = vi.fn();
vi.mock("next/navigation", () => ({
  redirect: (url: string) => mockRedirect(url),
}));

// Mock Toast Context
const mockToastSuccess = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: vi.fn(),
    info: vi.fn(),
  }),
}));

// Mock Profile Context
const mockUpdateProfileData = vi.fn();
const mockRefreshProfile = vi.fn();
const mockUseProfile = vi.fn();
vi.mock("~/lib/profile-context", () => ({
  useProfile: () => mockUseProfile(),
}));

// Mock Profile API
const mockUpdateProfile = vi.fn();
const mockUploadAvatar = vi.fn();
vi.mock("~/lib/profile-api", () => ({
  updateProfile: (data: unknown) => mockUpdateProfile(data),
  uploadAvatar: (file: unknown) => mockUploadAvatar(file),
}));

// Mock Image Utils
const mockCompressAvatar = vi.fn();
vi.mock("~/lib/image-utils", () => ({
  compressAvatar: (file: unknown) => mockCompressAvatar(file),
}));

// Mock UI Components
vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({ title }: { title: string }) => (
    <div data-testid="page-header">{title}</div>
  ),
}));

vi.mock("~/components/simple/SimpleAlert", () => ({
  SimpleAlert: ({
    type,
    message,
    onClose,
  }: {
    type: string;
    message: string;
    onClose: () => void;
  }) => (
    <div data-testid="simple-alert" data-type={type}>
      {message}
      <button onClick={onClose}>Close</button>
    </div>
  ),
}));

vi.mock("~/components/ui", () => ({
  PasswordChangeModal: ({
    isOpen,
    onClose,
    onSuccess,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSuccess: () => void;
  }) => (
    <>
      {isOpen && (
        <div data-testid="password-modal">
          <button onClick={onClose}>Close Modal</button>
          <button onClick={onSuccess}>Success</button>
        </div>
      )}
    </>
  ),
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: ({ fullPage }: { fullPage?: boolean }) => (
    <div data-testid="loading" data-full-page={fullPage}>
      Loading...
    </div>
  ),
}));

vi.mock("~/lib/navigation-icons", () => ({
  navigationIcons: {
    profile:
      "M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z",
    security:
      "M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z",
  },
}));

// Mock Next.js Image component
vi.mock("next/image", () => ({
  default: ({
    src,
    alt,
    fill,
    priority,
    unoptimized,
    ...props
  }: {
    src: string;
    alt: string;
    fill?: boolean;
    priority?: boolean;
    unoptimized?: boolean;
    [key: string]: unknown;
  }) => (
    <img
      src={src}
      alt={alt}
      {...props}
      data-fill={fill ? "true" : undefined}
      data-priority={priority ? "true" : undefined}
      data-unoptimized={unoptimized ? "true" : undefined}
    />
  ),
}));

describe("SettingsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    // Default session mock
    mockUseSession.mockReturnValue({
      data: {
        user: {
          id: "1",
          name: "Test User",
          email: "test@example.com",
          token: "test-token",
          isAdmin: false,
        },
      },
      status: "authenticated",
    });

    // Default profile mock
    mockUseProfile.mockReturnValue({
      profile: {
        id: "1",
        firstName: "John",
        lastName: "Doe",
        email: "john.doe@example.com",
        avatar: null,
      },
      updateProfileData: mockUpdateProfileData,
      refreshProfile: mockRefreshProfile,
      loading: false,
    });

    // Reset window.innerWidth to desktop
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: 1024,
    });
  });

  describe("Loading State", () => {
    it("should show loading component when session is loading", () => {
      mockUseSession.mockReturnValue({
        data: null,
        status: "loading",
      });

      render(<SettingsPage />);

      expect(screen.getByTestId("loading")).toBeInTheDocument();
      expect(screen.getByTestId("loading")).toHaveAttribute(
        "data-full-page",
        "false",
      );
    });
  });

  describe("Authentication", () => {
    it("should redirect to home when user is not authenticated", () => {
      mockUseSession.mockReturnValue({
        data: null,
        status: "unauthenticated",
      });

      render(<SettingsPage />);

      expect(mockRedirect).toHaveBeenCalledWith("/");
    });
  });

  describe("Profile Tab - Desktop", () => {
    it("should render profile tab by default on desktop", async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("Vorname")).toBeInTheDocument();
        expect(screen.getByLabelText("Nachname")).toBeInTheDocument();
        expect(screen.getByLabelText("E-Mail")).toBeInTheDocument();
      });
    });

    it("should display profile data in form fields", async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        const firstNameInput = screen.getByLabelText("Vorname");
        const lastNameInput = screen.getByLabelText("Nachname");
        const emailInput = screen.getByLabelText("E-Mail");

        expect((firstNameInput as HTMLInputElement).value).toBe("John");
        expect((lastNameInput as HTMLInputElement).value).toBe("Doe");
        expect((emailInput as HTMLInputElement).value).toBe(
          "john.doe@example.com",
        );
        expect(emailInput).toBeDisabled();
      });
    });

    it("should display user initials when no avatar is present", async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        const initials = screen.getAllByText("JD");
        expect(initials.length).toBeGreaterThan(0);
      });
    });

    it("should display avatar image when profile has avatar", async () => {
      mockUseProfile.mockReturnValue({
        profile: {
          id: "1",
          firstName: "John",
          lastName: "Doe",
          email: "john.doe@example.com",
          avatar: "https://example.com/avatar.jpg",
        },
        updateProfileData: mockUpdateProfileData,
        refreshProfile: mockRefreshProfile,
        loading: false,
      });

      render(<SettingsPage />);

      await waitFor(() => {
        const avatarImages = screen.getAllByAltText("Profile");
        expect(avatarImages.length).toBeGreaterThan(0);
        expect(avatarImages[0]).toHaveAttribute(
          "src",
          "https://example.com/avatar.jpg",
        );
      });
    });

    it("should enable edit mode when clicking Bearbeiten button", async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        const editButton = screen.getByRole("button", { name: /bearbeiten/i });
        fireEvent.click(editButton);
      });

      await waitFor(() => {
        const firstNameInput = screen.getByLabelText("Vorname");
        expect(firstNameInput).not.toBeDisabled();
        expect(
          screen.getByRole("button", { name: /abbrechen/i }),
        ).toBeInTheDocument();
        expect(
          screen.getByRole("button", { name: /speichern/i }),
        ).toBeInTheDocument();
      });
    });

    it("should update form fields when editing", async () => {
      render(<SettingsPage />);

      const editButton = screen.getByRole("button", { name: /bearbeiten/i });
      fireEvent.click(editButton);

      await waitFor(() => {
        const firstNameInput = screen.getByLabelText("Vorname");
        fireEvent.change(firstNameInput, { target: { value: "Jane" } });
        expect((firstNameInput as HTMLInputElement).value).toBe("Jane");
      });
    });

    it("should cancel edit and reset form data", async () => {
      render(<SettingsPage />);

      const editButton = screen.getByRole("button", { name: /bearbeiten/i });
      fireEvent.click(editButton);

      await waitFor(() => {
        const firstNameInput = screen.getByLabelText("Vorname");
        fireEvent.change(firstNameInput, { target: { value: "Jane" } });
        expect((firstNameInput as HTMLInputElement).value).toBe("Jane");
      });

      const cancelButton = screen.getByRole("button", { name: /abbrechen/i });
      fireEvent.click(cancelButton);

      await waitFor(() => {
        const firstNameInput = screen.getByLabelText("Vorname");
        expect((firstNameInput as HTMLInputElement).value).toBe("John");
        expect(firstNameInput).toBeDisabled();
      });
    });

    it("should save profile changes successfully", async () => {
      mockUpdateProfile.mockResolvedValue({});
      mockRefreshProfile.mockResolvedValue({});

      render(<SettingsPage />);

      const editButton = screen.getByRole("button", { name: /bearbeiten/i });
      fireEvent.click(editButton);

      await waitFor(() => {
        const firstNameInput = screen.getByLabelText("Vorname");
        fireEvent.change(firstNameInput, { target: { value: "Jane" } });
      });

      const saveButton = screen.getByRole("button", { name: /^speichern$/i });
      fireEvent.click(saveButton);

      await waitFor(() => {
        expect(mockUpdateProfile).toHaveBeenCalledWith({
          firstName: "Jane",
          lastName: "Doe",
        });
        expect(mockUpdateProfileData).toHaveBeenCalledWith({
          firstName: "Jane",
          lastName: "Doe",
        });
        expect(mockRefreshProfile).toHaveBeenCalledWith(true);
        expect(mockToastSuccess).toHaveBeenCalledWith(
          "Profil erfolgreich aktualisiert",
        );
      });
    });

    it("should show error alert when profile save fails", async () => {
      mockUpdateProfile.mockRejectedValue(new Error("Save failed"));

      render(<SettingsPage />);

      const editButton = screen.getByRole("button", { name: /bearbeiten/i });
      fireEvent.click(editButton);

      await waitFor(() => {
        const firstNameInput = screen.getByLabelText("Vorname");
        fireEvent.change(firstNameInput, { target: { value: "Jane" } });
      });

      const saveButton = screen.getByRole("button", { name: /^speichern$/i });
      fireEvent.click(saveButton);

      await waitFor(() => {
        const alert = screen.getByTestId("simple-alert");
        expect(alert).toBeInTheDocument();
        expect(alert).toHaveAttribute("data-type", "error");
        expect(alert).toHaveTextContent("Fehler beim Speichern des Profils");
      });
    });

    it("should handle avatar upload successfully", async () => {
      const mockFile = new File(["test"], "test.jpg", { type: "image/jpeg" });
      const mockCompressedFile = new File(["compressed"], "test.jpg", {
        type: "image/jpeg",
      });

      mockCompressAvatar.mockResolvedValue(mockCompressedFile);
      mockUploadAvatar.mockResolvedValue({});
      mockRefreshProfile.mockResolvedValue({});

      render(<SettingsPage />);

      // Wait for component to be ready
      await waitFor(() => {
        expect(screen.getByLabelText("Vorname")).toBeInTheDocument();
      });

      // Get the actual file input element (not the label)
      const fileInput = document.querySelector("#avatar-upload")!;
      expect(fileInput).toBeInTheDocument();

      // Create a more realistic file list
      Object.defineProperty(fileInput, "files", {
        value: [mockFile],
        writable: false,
      });

      // Trigger change event
      fireEvent.change(fileInput);

      await waitFor(
        () => {
          expect(mockCompressAvatar).toHaveBeenCalledWith(mockFile);
        },
        { timeout: 3000 },
      );

      await waitFor(
        () => {
          expect(mockUploadAvatar).toHaveBeenCalledWith(mockCompressedFile);
          expect(mockRefreshProfile).toHaveBeenCalledWith(true);
          expect(mockToastSuccess).toHaveBeenCalledWith(
            "Profilbild erfolgreich aktualisiert",
          );
        },
        { timeout: 3000 },
      );
    });

    it("should show error alert when avatar upload fails", async () => {
      const mockFile = new File(["test"], "test.jpg", { type: "image/jpeg" });
      mockCompressAvatar.mockRejectedValue(new Error("Compression failed"));

      render(<SettingsPage />);

      const fileInput = document.querySelector("#avatar-upload")!;
      expect(fileInput).toBeInTheDocument();

      Object.defineProperty(fileInput, "files", {
        value: [mockFile],
        writable: false,
      });
      fireEvent.change(fileInput);

      await waitFor(
        () => {
          const alert = screen.getByTestId("simple-alert");
          expect(alert).toBeInTheDocument();
          expect(alert).toHaveAttribute("data-type", "error");
          expect(alert).toHaveTextContent(
            "Fehler beim Hochladen des Profilbilds",
          );
        },
        { timeout: 3000 },
      );
    });
  });

  describe("Security Tab - Desktop", () => {
    it("should switch to security tab when clicked", async () => {
      render(<SettingsPage />);

      const securityTab = screen.getByRole("tab", {
        name: /^.*sicherheit$/i,
      });
      clickRadixTrigger(securityTab);

      await waitFor(
        () => {
          expect(
            screen.getByText(
              "Aktualisieren Sie Ihr Passwort regelmäßig für zusätzliche Sicherheit.",
            ),
          ).toBeInTheDocument();
        },
        { timeout: 2000 },
      );
    });

    it("should open password change modal", async () => {
      render(<SettingsPage />);

      const securityTab = screen.getByRole("tab", { name: /sicherheit/i });
      clickRadixTrigger(securityTab);

      await waitFor(() => {
        const changePasswordButton = screen.getByRole("button", {
          name: /passwort ändern/i,
        });
        fireEvent.click(changePasswordButton);
      });

      await waitFor(() => {
        expect(screen.getByTestId("password-modal")).toBeInTheDocument();
      });
    });

    it("should close password modal and show success toast on success", async () => {
      render(<SettingsPage />);

      const securityTab = screen.getByRole("tab", { name: /sicherheit/i });
      clickRadixTrigger(securityTab);

      await waitFor(() => {
        const changePasswordButton = screen.getByRole("button", {
          name: /passwort ändern/i,
        });
        fireEvent.click(changePasswordButton);
      });

      await waitFor(() => {
        const successButton = screen.getByRole("button", { name: "Success" });
        fireEvent.click(successButton);
      });

      await waitFor(() => {
        expect(screen.queryByTestId("password-modal")).not.toBeInTheDocument();
        expect(mockToastSuccess).toHaveBeenCalledWith(
          "Passwort erfolgreich geändert",
        );
      });
    });

    it("should close password modal when close button clicked", async () => {
      render(<SettingsPage />);

      const securityTab = screen.getByRole("tab", { name: /sicherheit/i });
      clickRadixTrigger(securityTab);

      await waitFor(() => {
        const changePasswordButton = screen.getByRole("button", {
          name: /passwort ändern/i,
        });
        fireEvent.click(changePasswordButton);
      });

      await waitFor(() => {
        const closeButton = screen.getByRole("button", { name: "Close Modal" });
        fireEvent.click(closeButton);
      });

      await waitFor(() => {
        expect(screen.queryByTestId("password-modal")).not.toBeInTheDocument();
      });
    });
  });

  describe("Tab Navigation - Desktop", () => {
    it("should render tab navigation on desktop", async () => {
      render(<SettingsPage />);

      await waitFor(
        () => {
          expect(screen.getByText("Profil")).toBeInTheDocument();
          expect(screen.getByText("Sicherheit")).toBeInTheDocument();
        },
        { timeout: 2000 },
      );
    });

    it("should highlight active tab", async () => {
      render(<SettingsPage />);

      await waitFor(
        () => {
          const profileTab = screen.getByRole("tab", { name: /profil/i });
          expect(profileTab).toHaveAttribute("data-state", "active");
        },
        { timeout: 2000 },
      );
    });
  });

  describe("Mobile Responsiveness", () => {
    beforeEach(() => {
      // Set mobile viewport
      Object.defineProperty(window, "innerWidth", {
        writable: true,
        configurable: true,
        value: 375,
      });
    });

    it("should show mobile list view initially", async () => {
      render(<SettingsPage />);

      // Trigger resize
      fireEvent(window, new Event("resize"));

      await waitFor(() => {
        expect(screen.getByTestId("page-header")).toBeInTheDocument();
        expect(screen.getByText("John Doe")).toBeInTheDocument();
        expect(screen.getByText("Profil, Profilbild")).toBeInTheDocument();
      });
    });

    it("should navigate to profile detail on mobile", async () => {
      render(<SettingsPage />);

      fireEvent(window, new Event("resize"));

      await waitFor(() => {
        const profileCard = screen.getByText("John Doe").closest("button");
        if (profileCard) fireEvent.click(profileCard);
      });

      await waitFor(() => {
        expect(screen.getByLabelText("Zurück")).toBeInTheDocument();
        expect(screen.getByLabelText("Vorname")).toBeInTheDocument();
      });
    });

    it("should navigate back to list on mobile", async () => {
      render(<SettingsPage />);

      fireEvent(window, new Event("resize"));

      await waitFor(() => {
        const profileCard = screen.getByText("John Doe").closest("button");
        if (profileCard) fireEvent.click(profileCard);
      });

      await waitFor(() => {
        const backButton = screen.getByLabelText("Zurück");
        fireEvent.click(backButton);
      });

      await waitFor(() => {
        expect(screen.getByTestId("page-header")).toBeInTheDocument();
      });
    });

    it("should navigate to security tab on mobile", async () => {
      render(<SettingsPage />);

      fireEvent(window, new Event("resize"));

      await waitFor(() => {
        expect(screen.getByText("Sicherheit")).toBeInTheDocument();
      });

      const securityButton = screen.getByText("Sicherheit").closest("button");
      if (securityButton) fireEvent.click(securityButton);

      await waitFor(
        () => {
          expect(screen.getByLabelText("Zurück")).toBeInTheDocument();
          expect(
            screen.getByText(
              "Aktualisieren Sie Ihr Passwort regelmäßig für zusätzliche Sicherheit.",
            ),
          ).toBeInTheDocument();
        },
        { timeout: 2000 },
      );
    });
  });

  describe("Admin User", () => {
    it("should show admin tabs for admin users", async () => {
      mockUseSession.mockReturnValue({
        data: {
          user: {
            id: "1",
            name: "Admin User",
            email: "admin@example.com",
            token: "admin-token",
            isAdmin: true,
          },
        },
        status: "authenticated",
      });

      render(<SettingsPage />);

      await waitFor(
        () => {
          expect(screen.getByText("Profil")).toBeInTheDocument();
          expect(screen.getByText("Sicherheit")).toBeInTheDocument();
        },
        { timeout: 2000 },
      );
    });
  });

  describe("Alert Handling", () => {
    it("should close alert when close button clicked", async () => {
      mockUpdateProfile.mockRejectedValue(new Error("Save failed"));

      render(<SettingsPage />);

      const editButton = screen.getByRole("button", { name: /bearbeiten/i });
      fireEvent.click(editButton);

      await waitFor(() => {
        const firstNameInput = screen.getByLabelText("Vorname");
        fireEvent.change(firstNameInput, { target: { value: "Jane" } });
      });

      const saveButton = screen.getByRole("button", { name: /^speichern$/i });
      fireEvent.click(saveButton);

      await waitFor(() => {
        const alert = screen.getByTestId("simple-alert");
        expect(alert).toBeInTheDocument();
      });

      const closeButton = screen.getByRole("button", { name: "Close" });
      fireEvent.click(closeButton);

      await waitFor(() => {
        expect(screen.queryByTestId("simple-alert")).not.toBeInTheDocument();
      });
    });
  });
});
