import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { NotificationPreferencesSection } from "./notification-preferences-section";

const api = vi.hoisted(() => ({
  fetchNotificationPreferences: vi.fn(),
  setNotificationPreference: vi.fn(),
  disableAllNotificationPreferences: vi.fn(),
}));

vi.mock("~/lib/notification-preferences-api", () => api);

function preferences(overrides?: { enabled?: boolean; available?: boolean }) {
  return {
    tenant_enabled: true,
    types: [
      {
        key: "pickup_upcoming",
        label: "Anstehende Abholung",
        description: "Kurz bevor ein Kind abgeholt wird.",
        group: "abholung",
        enabled: overrides?.enabled ?? false,
        available: overrides?.available ?? true,
      },
    ],
  };
}

describe("NotificationPreferencesSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.fetchNotificationPreferences.mockResolvedValue(preferences());
    api.setNotificationPreference.mockResolvedValue(undefined);
    api.disableAllNotificationPreferences.mockResolvedValue(undefined);
  });

  it("renders the catalogue grouped under German headings", async () => {
    render(<NotificationPreferencesSection />);

    expect(await screen.findByText("Anstehende Abholung")).toBeInTheDocument();
    expect(screen.getByText("Abholungen")).toBeInTheDocument();
    expect(
      screen.getByRole("switch", { name: "Anstehende Abholung" }),
    ).toHaveAttribute("aria-checked", "false");
  });

  it("saves a decision when a switch is flipped", async () => {
    render(<NotificationPreferencesSection />);

    fireEvent.click(
      await screen.findByRole("switch", { name: "Anstehende Abholung" }),
    );

    await waitFor(() =>
      expect(api.setNotificationPreference).toHaveBeenCalledWith(
        "pickup_upcoming",
        true,
        "tenant",
      ),
    );
    expect(
      screen.getByRole("switch", { name: "Anstehende Abholung" }),
    ).toHaveAttribute("aria-checked", "true");
  });

  it("rolls the switch back when saving fails", async () => {
    api.setNotificationPreference.mockRejectedValue(new Error("boom"));
    render(<NotificationPreferencesSection />);

    fireEvent.click(
      await screen.findByRole("switch", { name: "Anstehende Abholung" }),
    );

    // The optimistic flip must not survive a failed save: otherwise the card
    // claims a consent the server never recorded.
    expect(
      await screen.findByText(
        "Die Einstellung konnte nicht gespeichert werden.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("switch", { name: "Anstehende Abholung" }),
    ).toHaveAttribute("aria-checked", "false");
  });

  it("disables switches only while a preference save is in flight", async () => {
    let finishSave: (() => void) | undefined;
    api.setNotificationPreference.mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          finishSave = resolve;
        }),
    );
    render(<NotificationPreferencesSection />);

    const preferenceSwitch = await screen.findByRole("switch", {
      name: "Anstehende Abholung",
    });
    fireEvent.click(preferenceSwitch);

    await waitFor(() => expect(preferenceSwitch).toBeDisabled());
    finishSave?.();
    await waitFor(() => expect(preferenceSwitch).not.toBeDisabled());
  });

  it("explains a type the school currently blocks but keeps it choosable", async () => {
    api.fetchNotificationPreferences.mockResolvedValue(
      preferences({ available: false }),
    );
    render(<NotificationPreferencesSection />);

    expect(
      await screen.findByText("Von Ihrer Schule derzeit deaktiviert."),
    ).toBeInTheDocument();
    // The choice belongs to the person and must survive the school toggling
    // its own setting off and on again.
    expect(
      screen.getByRole("switch", { name: "Anstehende Abholung" }),
    ).not.toBeDisabled();
  });

  it("keeps preferences editable when the school switched delivery off", async () => {
    api.fetchNotificationPreferences.mockResolvedValue({
      ...preferences(),
      tenant_enabled: false,
    });
    render(<NotificationPreferencesSection />);

    expect(
      await screen.findByText(/Ihre Schule hat Benachrichtigungen derzeit/),
    ).toBeInTheDocument();
    const preferenceSwitch = screen.getByRole("switch", {
      name: "Anstehende Abholung",
    });
    expect(preferenceSwitch).not.toBeDisabled();

    fireEvent.click(preferenceSwitch);
    await waitFor(() =>
      expect(api.setNotificationPreference).toHaveBeenCalledWith(
        "pickup_upcoming",
        true,
        "tenant",
      ),
    );
  });

  it("offers to switch everything off only when something is on", async () => {
    api.fetchNotificationPreferences.mockResolvedValue(
      preferences({ enabled: true }),
    );
    render(<NotificationPreferencesSection />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Alle deaktivieren" }),
    );

    await waitFor(() =>
      expect(api.disableAllNotificationPreferences).toHaveBeenCalledWith(
        "tenant",
      ),
    );
    expect(
      screen.queryByRole("button", { name: "Alle deaktivieren" }),
    ).not.toBeInTheDocument();
  });

  it("uses the parent portal when asked to", async () => {
    render(<NotificationPreferencesSection portal="parent" />);

    await waitFor(() =>
      expect(api.fetchNotificationPreferences).toHaveBeenCalledWith("parent"),
    );
  });
});
