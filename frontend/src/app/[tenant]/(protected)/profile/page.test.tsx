/* eslint-disable @typescript-eslint/no-unsafe-return, @next/next/no-img-element */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import ProfilePage from "./page";

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
const mockToastError = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
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
vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: ({ title }: { title: string }) => (
    <div data-testid="page-header">{title}</div>
  ),
}));

vi.mock("~/components/ui/password-change-modal", () => ({
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
          <button type="button" onClick={onClose}>
            Close Modal
          </button>
          <button type="button" onClick={onSuccess}>
            Success
          </button>
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

vi.mock("~/components/settings/passkey-settings-section", () => ({
  PasskeySettingsSection: () => <div data-testid="passkey-settings" />,
}));

vi.mock("~/components/settings/trusted-devices-section", () => ({
  TrustedDevicesSection: () => <div data-testid="trusted-devices" />,
}));

vi.mock("~/components/settings/notification-preferences-section", () => ({
  NotificationPreferencesSection: () => (
    <div data-testid="notification-preferences" />
  ),
}));

vi.mock("~/components/settings/birthday-visibility-section", () => ({
  BirthdayVisibilitySection: () => <div data-testid="birthday-visibility" />,
}));

vi.mock("~/components/settings/push-notification-section", () => ({
  PushNotificationSection: () => <div data-testid="push-notifications" />,
}));

// Mock Button component
vi.mock("~/components/ui/button", () => ({
  Button: ({
    children,
    isLoading,
    loadingText,
    variant: _variant,
    size: _size,
    ...props
  }: {
    children: React.ReactNode;
    isLoading?: boolean;
    loadingText?: string;
    variant?: string;
    size?: string;
    [key: string]: unknown;
  }) => (
    <button type="button" {...props} disabled={!!props.disabled || isLoading}>
      {isLoading ? loadingText : children}
    </button>
  ),
}));

// Mock Input component
vi.mock("~/components/ui/input", () => ({
  Input: ({
    label,
    error,
    name,
    ...props
  }: {
    label?: string;
    error?: string;
    name?: string;
    [key: string]: unknown;
  }) => (
    <div>
      {label && <label htmlFor={name}>{label}</label>}
      <input id={name} name={name} {...props} />
      {error && <span>{error}</span>}
    </div>
  ),
}));

// Mock lucide-react
vi.mock("lucide-react", () => ({
  Camera: (props: Record<string, unknown>) => (
    <svg data-testid="camera-icon" {...props} />
  ),
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

describe("ProfilePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();

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
      update: vi.fn().mockResolvedValue(undefined),
    });

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
  });

  describe("Loading State", () => {
    it("should show loading component when session is loading", () => {
      mockUseSession.mockReturnValue({
        data: null,
        status: "loading",
      });

      render(<ProfilePage />);

      expect(screen.getByTestId("loading")).toBeInTheDocument();
    });
  });

  describe("Authentication", () => {
    it("should redirect to home when user is not authenticated", () => {
      mockUseSession.mockReturnValue({
        data: null,
        status: "unauthenticated",
      });

      render(<ProfilePage />);

      expect(mockRedirect).toHaveBeenCalledWith("/");
    });
  });

  describe("Profile Form", () => {
    it("should render profile form fields", async () => {
      render(<ProfilePage />);

      await waitFor(() => {
        expect(screen.getByLabelText("Vorname")).toBeInTheDocument();
        expect(screen.getByLabelText("Nachname")).toBeInTheDocument();
        expect(screen.getByLabelText("E-Mail")).toBeInTheDocument();
      });
    });

    it("should display profile data in form fields", async () => {
      render(<ProfilePage />);

      await waitFor(() => {
        expect(screen.getByLabelText("Vorname")).toHaveValue("John");
        expect(screen.getByLabelText("Nachname")).toHaveValue("Doe");
        expect(screen.getByLabelText("E-Mail")).toHaveValue(
          "john.doe@example.com",
        );
        expect(screen.getByLabelText("E-Mail")).toBeDisabled();
      });
    });

    it("should display user initials when no avatar is present", async () => {
      render(<ProfilePage />);

      await waitFor(() => {
        expect(screen.getByText("JD")).toBeInTheDocument();
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

      render(<ProfilePage />);

      await waitFor(() => {
        const avatarImage = screen.getByAltText("Profile");
        expect(avatarImage).toHaveAttribute(
          "src",
          "https://example.com/avatar.jpg",
        );
      });
    });

    it("should enable edit mode when clicking Bearbeiten button", async () => {
      render(<ProfilePage />);

      fireEvent.click(screen.getByRole("button", { name: /bearbeiten/i }));

      await waitFor(() => {
        expect(screen.getByLabelText("Vorname")).not.toBeDisabled();
        expect(
          screen.getByRole("button", { name: /abbrechen/i }),
        ).toBeInTheDocument();
        expect(
          screen.getByRole("button", { name: /speichern/i }),
        ).toBeInTheDocument();
      });
    });

    it("should cancel edit and reset form data", async () => {
      render(<ProfilePage />);

      fireEvent.click(screen.getByRole("button", { name: /bearbeiten/i }));

      const firstNameInput = screen.getByLabelText("Vorname");
      fireEvent.change(firstNameInput, { target: { value: "Jane" } });
      expect(firstNameInput).toHaveValue("Jane");

      fireEvent.click(screen.getByRole("button", { name: /abbrechen/i }));

      await waitFor(() => {
        expect(screen.getByLabelText("Vorname")).toHaveValue("John");
        expect(screen.getByLabelText("Vorname")).toBeDisabled();
      });
    });

    it("should save profile changes successfully", async () => {
      mockUpdateProfile.mockResolvedValue({});
      mockRefreshProfile.mockResolvedValue({});

      render(<ProfilePage />);

      fireEvent.click(screen.getByRole("button", { name: /bearbeiten/i }));

      const firstNameInput = screen.getByLabelText("Vorname");
      fireEvent.change(firstNameInput, { target: { value: "Jane" } });

      fireEvent.click(screen.getByRole("button", { name: /^speichern$/i }));

      await waitFor(() => {
        expect(mockUpdateProfile).toHaveBeenCalledWith({
          firstName: "Jane",
          lastName: "Doe",
        });
        expect(mockToastSuccess).toHaveBeenCalledWith(
          "Profil erfolgreich aktualisiert",
        );
      });
    });

    it("should show error toast when profile save fails", async () => {
      mockUpdateProfile.mockRejectedValue(new Error("Save failed"));

      render(<ProfilePage />);

      fireEvent.click(screen.getByRole("button", { name: /bearbeiten/i }));

      const firstNameInput = screen.getByLabelText("Vorname");
      fireEvent.change(firstNameInput, { target: { value: "Jane" } });

      fireEvent.click(screen.getByRole("button", { name: /^speichern$/i }));

      await waitFor(() => {
        expect(mockToastError).toHaveBeenCalledWith(
          "Fehler beim Speichern des Profils",
        );
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

      render(<ProfilePage />);

      await waitFor(() => {
        expect(screen.getByLabelText("Vorname")).toBeInTheDocument();
      });

      const fileInput = document.querySelector("#avatar-upload")!;
      Object.defineProperty(fileInput, "files", {
        value: [mockFile],
        writable: false,
      });
      fireEvent.change(fileInput);

      await waitFor(
        () => {
          expect(mockCompressAvatar).toHaveBeenCalledWith(mockFile);
          expect(mockUploadAvatar).toHaveBeenCalledWith(mockCompressedFile);
          expect(mockToastSuccess).toHaveBeenCalledWith(
            "Profilbild erfolgreich aktualisiert",
          );
        },
        { timeout: 3000 },
      );
    });
  });

  describe("Password Change", () => {
    it("should open password change modal", async () => {
      render(<ProfilePage />);

      fireEvent.click(screen.getByRole("button", { name: /passwort ändern/i }));

      await waitFor(() => {
        expect(screen.getByTestId("password-modal")).toBeInTheDocument();
      });
    });

    it("should close password modal and show success toast on success", async () => {
      render(<ProfilePage />);

      fireEvent.click(screen.getByRole("button", { name: /passwort ändern/i }));

      fireEvent.click(await screen.findByRole("button", { name: "Success" }));

      await waitFor(() => {
        expect(screen.queryByTestId("password-modal")).not.toBeInTheDocument();
        expect(mockToastSuccess).toHaveBeenCalledWith(
          "Passwort erfolgreich geändert",
        );
      });
    });
  });
});
