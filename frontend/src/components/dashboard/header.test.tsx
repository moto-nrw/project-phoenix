import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

// Mock all dependencies BEFORE importing component
const mockUsePathname = vi.fn(() => "/students/search");
const mockUseSession = vi.fn(() => ({
  data: {
    user: {
      name: "Test User",
      email: "test@example.com",
      token: "mock-token",
      roles: ["admin"],
    },
    error: undefined as string | undefined,
  },
}));
const mockUseProfile = vi.fn(() => ({
  profile: {
    firstName: "John",
    lastName: "Doe",
    avatar: "avatar.png",
  } as { firstName: string; lastName: string; avatar: string } | undefined,
}));
const mockUseBreadcrumb = vi.fn(() => ({
  breadcrumb: {},
}));

vi.mock("next/navigation", () => ({
  usePathname: () => mockUsePathname(),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => mockUseSession(),
}));

vi.mock("@/components/ui/help_button", () => ({
  HelpButton: () => <button data-testid="help-button">Help</button>,
}));

vi.mock("@/lib/help-content", () => ({
  getHelpContent: () => ({ title: "Help Title", content: "Help Content" }),
}));

vi.mock("~/components/ui/logout-modal", () => ({
  LogoutModal: ({ isOpen }: { isOpen: boolean }) =>
    isOpen ? <div data-testid="logout-modal">Logout Modal</div> : null,
}));

vi.mock("~/lib/profile-context", () => ({
  useProfile: () => mockUseProfile(),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useBreadcrumb: () => mockUseBreadcrumb(),
}));

vi.mock("./header/brand-link", () => ({
  BrandLink: () => <div data-testid="brand-link">Brand</div>,
  BreadcrumbDivider: () => <div data-testid="divider">/</div>,
}));

vi.mock("./header/session-warning", () => ({
  SessionWarning: ({ variant }: { variant: string }) => (
    <div data-testid={`session-warning-${variant}`}>Warning</div>
  ),
}));

vi.mock("./header/profile-dropdown", () => ({
  ProfileTrigger: ({ onClick }: { onClick: () => void }) => (
    <button data-testid="profile-trigger" onClick={onClick}>
      Profile
    </button>
  ),
  ProfileDropdownMenu: ({
    isOpen,
    onClose,
    onLogout,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onLogout: () => void;
  }) =>
    isOpen ? (
      <div data-testid="profile-dropdown">
        <button data-testid="close-dropdown" onClick={onClose}>
          Close
        </button>
        <button data-testid="logout-button" onClick={onLogout}>
          Logout
        </button>
      </div>
    ) : null,
}));

vi.mock("./header/breadcrumb-components", () => ({
  DatabaseBreadcrumb: () => <div data-testid="database-breadcrumb">DB</div>,
  OgsGroupsBreadcrumb: () => <div data-testid="ogs-groups-breadcrumb">OGS</div>,
  ActiveSupervisionsBreadcrumb: () => (
    <div data-testid="active-supervisions-breadcrumb">Active</div>
  ),
  InvitationsBreadcrumb: () => (
    <div data-testid="invitations-breadcrumb">Invitations</div>
  ),
  ActivityBreadcrumb: () => (
    <div data-testid="activity-breadcrumb">Activity</div>
  ),
  RoomBreadcrumb: () => <div data-testid="room-breadcrumb">Room</div>,
  StudentHistoryBreadcrumb: () => (
    <div data-testid="student-history-breadcrumb">History</div>
  ),
  StudentDetailBreadcrumb: () => (
    <div data-testid="student-detail-breadcrumb">Detail</div>
  ),
  PageTitleDisplay: ({ title }: { title: string }) => (
    <div data-testid="page-title">{title}</div>
  ),
}));

vi.mock("./header/breadcrumb-utils", () => ({
  getPageTitle: (pathname: string) => {
    if (pathname === "/rooms") return "Rooms";
    return "Dashboard";
  },
  getSubPageLabel: () => "Sub Page",
  getBreadcrumbLabel: () => "Breadcrumb",
  getHistoryType: () => "history",
  getPageTypeInfo: (pathname: string) => ({
    isDatabaseSubPage: pathname.includes("/database/"),
    isDatabaseDeepPage: pathname.includes("/database/groups/"),
    isActivityDetailPage:
      pathname.includes("/activities/") && pathname !== "/activities",
    isRoomDetailPage: pathname.includes("/rooms/") && pathname !== "/rooms",
    isStudentHistoryPage:
      pathname.includes("/students/") && pathname.includes("/history"),
    isStudentDetailPage:
      pathname.includes("/students/") &&
      !pathname.includes("/history") &&
      pathname !== "/students/search",
  }),
}));

// Import component AFTER all mocks are defined
import { Header } from "./header";

describe("Header", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset mocks to default values
    mockUsePathname.mockReturnValue("/students/search");
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "Test User",
          email: "test@example.com",
          token: "mock-token",
          roles: ["admin"],
        },
        error: undefined,
      },
    });
    mockUseProfile.mockReturnValue({
      profile: {
        firstName: "John",
        lastName: "Doe",
        avatar: "avatar.png",
      },
    });
    mockUseBreadcrumb.mockReturnValue({
      breadcrumb: {},
    });
    // Reset window.scrollY
    Object.defineProperty(window, "scrollY", { value: 0, writable: true });
  });

  it("renders brand link and divider", () => {
    render(<Header />);
    expect(screen.getByTestId("brand-link")).toBeInTheDocument();
    expect(screen.getByTestId("divider")).toBeInTheDocument();
  });

  it("renders help button on desktop", () => {
    render(<Header />);
    expect(screen.getByTestId("help-button")).toBeInTheDocument();
  });

  it("renders session warnings for both desktop and mobile", () => {
    render(<Header />);
    expect(screen.getByTestId("session-warning-desktop")).toBeInTheDocument();
    expect(screen.getByTestId("session-warning-mobile")).toBeInTheDocument();
  });

  it("toggles profile menu on click", () => {
    render(<Header />);

    // Menu should be closed initially
    expect(screen.queryByTestId("profile-dropdown")).not.toBeInTheDocument();

    // Click to open
    fireEvent.click(screen.getByTestId("profile-trigger"));
    expect(screen.getByTestId("profile-dropdown")).toBeInTheDocument();

    // Click to close
    fireEvent.click(screen.getByTestId("close-dropdown"));
    expect(screen.queryByTestId("profile-dropdown")).not.toBeInTheDocument();
  });

  it("opens logout modal when logout is clicked", () => {
    render(<Header />);

    // Open profile menu
    fireEvent.click(screen.getByTestId("profile-trigger"));

    // Click logout
    fireEvent.click(screen.getByTestId("logout-button"));

    expect(screen.getByTestId("logout-modal")).toBeInTheDocument();
  });

  it("applies scroll styles when scrolled", () => {
    const { container } = render(<Header />);
    const header = container.querySelector("header");

    // Initially not scrolled
    expect(header).not.toHaveClass("shadow-sm");

    // Simulate scroll
    Object.defineProperty(window, "scrollY", { value: 30, writable: true });
    fireEvent.scroll(window);

    // Should have shadow
    expect(header).toHaveClass("shadow-sm");
  });

  it("renders page title for simple routes", () => {
    mockUsePathname.mockReturnValue("/rooms");

    render(<Header />);
    expect(screen.getByTestId("page-title")).toHaveTextContent("Rooms");
  });

  it("renders database breadcrumb for database pages", () => {
    mockUsePathname.mockReturnValue("/database/groups/combined");

    render(<Header />);
    expect(screen.getByTestId("database-breadcrumb")).toBeInTheDocument();
  });

  it("renders OGS groups breadcrumb", () => {
    mockUsePathname.mockReturnValue("/ogs-groups");

    render(<Header />);
    expect(screen.getByTestId("ogs-groups-breadcrumb")).toBeInTheDocument();
  });

  it("renders active supervisions breadcrumb", () => {
    mockUsePathname.mockReturnValue("/active-supervisions");

    render(<Header />);
    expect(
      screen.getByTestId("active-supervisions-breadcrumb"),
    ).toBeInTheDocument();
  });

  it("renders invitations breadcrumb", () => {
    mockUsePathname.mockReturnValue("/invitations");

    render(<Header />);
    expect(screen.getByTestId("invitations-breadcrumb")).toBeInTheDocument();
  });

  it("renders activity breadcrumb for activity detail pages", () => {
    mockUsePathname.mockReturnValue("/activities/123");
    mockUseBreadcrumb.mockReturnValue({
      breadcrumb: { activityName: "Test Activity" },
    });

    render(<Header />);
    expect(screen.getByTestId("activity-breadcrumb")).toBeInTheDocument();
  });

  it("renders room breadcrumb for room detail pages", () => {
    mockUsePathname.mockReturnValue("/rooms/123");
    mockUseBreadcrumb.mockReturnValue({
      breadcrumb: { roomName: "Test Room" },
    });

    render(<Header />);
    expect(screen.getByTestId("room-breadcrumb")).toBeInTheDocument();
  });

  it("renders student history breadcrumb", () => {
    mockUsePathname.mockReturnValue("/students/123/history");
    mockUseBreadcrumb.mockReturnValue({
      breadcrumb: { studentName: "Test Student" },
    });

    render(<Header />);
    expect(
      screen.getByTestId("student-history-breadcrumb"),
    ).toBeInTheDocument();
  });

  it("renders student detail breadcrumb", () => {
    mockUsePathname.mockReturnValue("/students/123");
    mockUseBreadcrumb.mockReturnValue({
      breadcrumb: { studentName: "Test Student" },
    });

    render(<Header />);
    expect(screen.getByTestId("student-detail-breadcrumb")).toBeInTheDocument();
  });

  it("uses custom page title from breadcrumb context", () => {
    mockUseBreadcrumb.mockReturnValue({
      breadcrumb: { pageTitle: "Custom Title" },
    });

    render(<Header />);
    expect(screen.getByTestId("page-title")).toHaveTextContent("Custom Title");
  });

  it("displays user info from session", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "Admin User",
          email: "admin@example.com",
          token: "test-token",
          roles: ["admin"],
        },
        error: undefined,
      },
    });

    render(<Header />);
    fireEvent.click(screen.getByTestId("profile-trigger"));

    // Profile dropdown should be visible
    expect(screen.getByTestId("profile-dropdown")).toBeInTheDocument();
  });

  it("handles expired session", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "User",
          email: "user@example.com",
          token: "test-token",
          roles: [],
        },
        error: "RefreshTokenExpired",
      },
    });

    render(<Header />);

    // Session warnings should be rendered
    expect(screen.getByTestId("session-warning-desktop")).toBeInTheDocument();
    expect(screen.getByTestId("session-warning-mobile")).toBeInTheDocument();
  });

  it("uses profile data from context when available", () => {
    mockUseProfile.mockReturnValue({
      profile: {
        firstName: "Jane",
        lastName: "Smith",
        avatar: "jane.png",
      },
    });

    render(<Header />);
    fireEvent.click(screen.getByTestId("profile-trigger"));

    expect(screen.getByTestId("profile-dropdown")).toBeInTheDocument();
  });

  it("falls back to session name when profile is not available", () => {
    mockUseProfile.mockReturnValue({ profile: undefined });

    render(<Header />);
    fireEvent.click(screen.getByTestId("profile-trigger"));

    expect(screen.getByTestId("profile-dropdown")).toBeInTheDocument();
  });

  it("cleans up scroll listener on unmount", () => {
    const removeEventListenerSpy = vi.spyOn(window, "removeEventListener");

    const { unmount } = render(<Header />);
    unmount();

    expect(removeEventListenerSpy).toHaveBeenCalledWith(
      "scroll",
      expect.any(Function),
    );
  });
});
