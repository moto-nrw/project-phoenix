import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  mockToastSuccess,
  mockToastError,
  mockGetFeed,
  mockRotateFeed,
  mockGetStaffFeed,
  mockRotateStaffFeed,
} = vi.hoisted(() => ({
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
  mockGetFeed: vi.fn(),
  mockRotateFeed: vi.fn(),
  mockGetStaffFeed: vi.fn(),
  mockRotateStaffFeed: vi.fn(),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mockToastSuccess, error: mockToastError }),
}));

vi.mock("~/lib/personal-calendar-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/personal-calendar-api")
  >("~/lib/personal-calendar-api");
  return {
    ...actual,
    getParentCalendarFeed: mockGetFeed,
    rotateParentCalendarFeed: mockRotateFeed,
    getStaffCalendarFeed: mockGetStaffFeed,
    rotateStaffCalendarFeed: mockRotateStaffFeed,
  };
});

import {
  CalendarSubscribePanel,
  StaffCalendarSubscribePanel,
} from "./calendar-subscribe-panel";

describe("CalendarSubscribePanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("loads and shows the subscription URLs on demand", async () => {
    mockGetFeed.mockResolvedValue({
      url: "https://parents.test/api/calendar-feed/abc",
      webcal_url: "webcal://parents.test/api/calendar-feed/abc",
    });

    render(<CalendarSubscribePanel />);

    // The URL is not fetched until the parent asks for it.
    expect(mockGetFeed).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: /Abo-Link anzeigen/ }));

    await waitFor(() => expect(mockGetFeed).toHaveBeenCalledOnce());

    // The subscribe (webcal) link and the copyable https link both render.
    const subscribeLink = await screen.findByRole("link", {
      name: /Im Kalender abonnieren/,
    });
    expect(subscribeLink).toHaveAttribute(
      "href",
      "webcal://parents.test/api/calendar-feed/abc",
    );
    expect(
      screen.getByDisplayValue("https://parents.test/api/calendar-feed/abc"),
    ).toBeInTheDocument();
  });

  it("shows a visible success state after copying the link", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });
    mockGetFeed.mockResolvedValue({
      url: "https://parents.test/api/calendar-feed/abc",
      webcal_url: "webcal://parents.test/api/calendar-feed/abc",
    });

    render(<CalendarSubscribePanel />);
    fireEvent.click(screen.getByRole("button", { name: /Abo-Link anzeigen/ }));
    fireEvent.click(await screen.findByRole("button", { name: /^Kopieren$/ }));

    await waitFor(() =>
      expect(screen.getByRole("button", { name: /^Kopiert$/ })).toHaveClass(
        "bg-moto-green",
      ),
    );
    expect(writeText).toHaveBeenCalledWith(
      "https://parents.test/api/calendar-feed/abc",
    );
    expect(mockToastSuccess).toHaveBeenCalledWith("Link kopiert.");
  });

  it("passes the subscription URL to Apple Calendar on macOS", async () => {
    const userAgent = vi
      .spyOn(window.navigator, "userAgent", "get")
      .mockReturnValue(
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/140 Safari/537.36",
      );
    mockGetFeed.mockResolvedValue({
      url: "https://parents.test/api/calendar-feed/abc",
      webcal_url: "webcal://parents.test/api/calendar-feed/abc",
    });

    render(<CalendarSubscribePanel />);
    fireEvent.click(screen.getByRole("button", { name: /Abo-Link anzeigen/ }));

    const subscribeLink = await screen.findByRole("link", {
      name: /Im Kalender abonnieren/,
    });
    await waitFor(() =>
      expect(subscribeLink).toHaveAttribute(
        "href",
        "webcal://parents.test/api/calendar-feed/abc",
      ),
    );
    userAgent.mockRestore();
  });

  it("prompts to regenerate when the link is not re-displayable", async () => {
    // The backend only stores the token hash, so an already-generated feed
    // returns empty URLs (show-once). The panel offers a regenerate instead.
    mockGetFeed.mockResolvedValue({ url: "", webcal_url: "" });
    mockRotateFeed.mockResolvedValue({
      url: "https://parents.test/api/calendar-feed/fresh",
      webcal_url: "webcal://parents.test/api/calendar-feed/fresh",
    });

    render(<CalendarSubscribePanel />);
    fireEvent.click(screen.getByRole("button", { name: /Abo-Link anzeigen/ }));
    await waitFor(() => expect(mockGetFeed).toHaveBeenCalledOnce());

    // No URL is shown; the parent gets a regenerate button instead.
    expect(
      screen.queryByRole("link", { name: /Im Kalender abonnieren/ }),
    ).toBeNull();
    const regenerate = await screen.findByRole("button", {
      name: /Neuen Abo-Link erstellen/,
    });

    fireEvent.click(regenerate);
    await waitFor(() => expect(mockRotateFeed).toHaveBeenCalledOnce());
    await waitFor(() =>
      expect(
        screen.getByDisplayValue(
          "https://parents.test/api/calendar-feed/fresh",
        ),
      ).toBeInTheDocument(),
    );
  });

  it("rotates the link when requested", async () => {
    mockGetFeed.mockResolvedValue({
      url: "https://parents.test/api/calendar-feed/old",
      webcal_url: "webcal://parents.test/api/calendar-feed/old",
    });
    mockRotateFeed.mockResolvedValue({
      url: "https://parents.test/api/calendar-feed/new",
      webcal_url: "webcal://parents.test/api/calendar-feed/new",
    });

    render(<CalendarSubscribePanel />);
    fireEvent.click(screen.getByRole("button", { name: /Abo-Link anzeigen/ }));
    await screen.findByRole("link", { name: /Im Kalender abonnieren/ });

    fireEvent.click(screen.getByRole("button", { name: /Link neu erstellen/ }));
    await waitFor(() => expect(mockRotateFeed).toHaveBeenCalledOnce());
    await waitFor(() =>
      expect(
        screen.getByDisplayValue("https://parents.test/api/calendar-feed/new"),
      ).toBeInTheDocument(),
    );
    expect(mockToastSuccess).toHaveBeenCalled();
  });
});

describe("StaffCalendarSubscribePanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("loads the staff feed and explains that the subscription is read-only", async () => {
    mockGetStaffFeed.mockResolvedValue({
      url: "https://school.test/api/calendar-feed/staff-token",
      webcal_url: "webcal://school.test/api/calendar-feed/staff-token",
    });

    render(<StaffCalendarSubscribePanel />);

    expect(
      screen.getByText(/Neue, geänderte und abgesagte Termine/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Abo-Link anzeigen/ }));

    await waitFor(() => expect(mockGetStaffFeed).toHaveBeenCalledOnce());
    expect(
      await screen.findByDisplayValue(
        "https://school.test/api/calendar-feed/staff-token",
      ),
    ).toBeInTheDocument();
  });

  it("states that creating a new link ends the previous subscription", async () => {
    mockGetStaffFeed.mockResolvedValue({ url: "", webcal_url: "" });

    render(<StaffCalendarSubscribePanel />);
    fireEvent.click(screen.getByRole("button", { name: /Abo-Link anzeigen/ }));

    expect(
      await screen.findByText(/Ein neuer Link beendet das bisherige Abo/),
    ).toBeInTheDocument();
    expect(mockRotateStaffFeed).not.toHaveBeenCalled();
  });
});
