import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { NotificationPreferencesSection } from "./notification-preferences-section";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

// The shared error path shows failures through the toast provider (#2517).
function renderWithToast(
  ui: Parameters<typeof render>[0],
  options?: Parameters<typeof render>[1],
) {
  return render(ui, { wrapper: ToastProvider, ...options });
}

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

  it("reserves grouped rows and header action while preferences load", () => {
    api.fetchNotificationPreferences.mockReturnValue(new Promise(() => {}));

    renderWithToast(<NotificationPreferencesSection portal="parent" />);

    const skeleton = screen.getByTestId("notification-preferences-skeleton");
    expect(skeleton.querySelectorAll(".min-h-16")).toHaveLength(5);
    expect(skeleton.querySelectorAll(".animate-pulse").length).toBeGreaterThan(
      10,
    );
  });

  it("renders the catalogue grouped under German headings", async () => {
    renderWithToast(<NotificationPreferencesSection />);

    expect(await screen.findByText("Anstehende Abholung")).toBeInTheDocument();
    expect(screen.getByText("Abholungen")).toBeInTheDocument();
    expect(
      screen.getByRole("switch", { name: "Anstehende Abholung" }),
    ).toHaveAttribute("aria-checked", "false");
  });

  it("renders every group the backend sends, in its order", async () => {
    // The card used to render from a hard-coded group list, so a group added in
    // the backend catalogue silently dropped its types — and with "no row means
    // off", nobody could ever switch them on.
    api.fetchNotificationPreferences.mockResolvedValue({
      tenant_enabled: true,
      types: [
        {
          key: "parent_announcement",
          label: "Neue Mitteilung der Schule",
          description: "Wenn die Schule etwas veröffentlicht.",
          group: "mitteilungen",
          enabled: false,
          available: true,
        },
        {
          key: "parent_appointment",
          label: "Neuer oder geänderter Termin",
          description: "Wenn ein Termin angelegt wird.",
          group: "termine",
          enabled: false,
          available: true,
        },
        {
          key: "something_new",
          label: "Ganz neue Art",
          description:
            "Aus einer Gruppe, die dieses Frontend noch nicht kennt.",
          group: "zukunft",
          enabled: false,
          available: true,
        },
      ],
    });

    renderWithToast(<NotificationPreferencesSection portal="parent" />);

    expect(await screen.findByText("Termine")).toBeInTheDocument();
    expect(
      screen.getByRole("switch", { name: "Neuer oder geänderter Termin" }),
    ).toBeInTheDocument();
    // An unknown group falls back to its raw name rather than vanishing.
    expect(screen.getByText("zukunft")).toBeInTheDocument();
    expect(
      screen.getByRole("switch", { name: "Ganz neue Art" }),
    ).toBeInTheDocument();
  });

  it("saves a decision when a switch is flipped", async () => {
    renderWithToast(<NotificationPreferencesSection />);

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
    api.setNotificationPreference.mockRejectedValue(new ApiError("boom", 500));
    renderWithToast(<NotificationPreferencesSection />);

    fireEvent.click(
      await screen.findByRole("switch", { name: "Anstehende Abholung" }),
    );

    // The optimistic flip must not survive a failed save: otherwise the card
    // claims a consent the server never recorded. #2517: catalog text toast.
    expect(
      await screen.findByText(
        catalogText(
          "general.server",
          "die Benachrichtigung „Anstehende Abholung“",
        ),
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
    renderWithToast(<NotificationPreferencesSection />);

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
    renderWithToast(<NotificationPreferencesSection />);

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
    renderWithToast(<NotificationPreferencesSection />);

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
    renderWithToast(<NotificationPreferencesSection />);

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

  it("offers one primary action to enable every parent notification", async () => {
    api.fetchNotificationPreferences.mockResolvedValue({
      tenant_enabled: true,
      types: [
        {
          ...preferences().types[0],
          enabled: true,
        },
        {
          key: "parent_message",
          label: "Neue Nachricht der OGS",
          description: "Wenn die OGS zu einem Kind schreibt.",
          group: "mitteilungen",
          enabled: false,
          available: true,
        },
      ],
    });
    renderWithToast(<NotificationPreferencesSection portal="parent" />);

    expect(
      await screen.findByText(
        "Bleiben Sie über Nachrichten und wichtige Änderungen zu Ihrem Kind informiert.",
      ),
    ).toBeVisible();
    const enableAllButton = screen.getByRole("button", {
      name: "Alle aktivieren",
    });
    expect(enableAllButton).toHaveClass(
      "rounded-lg",
      "px-4",
      "py-2",
      "text-sm",
    );
    expect(enableAllButton).not.toHaveClass("min-h-12", "rounded-xl", "w-full");
    fireEvent.click(enableAllButton);

    await waitFor(() =>
      expect(api.setNotificationPreference).toHaveBeenCalledWith(
        "parent_message",
        true,
        "parent",
      ),
    );
    expect(api.setNotificationPreference).toHaveBeenCalledTimes(1);
    expect(await screen.findByText("Alle aktiviert")).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Alle aktivieren" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Alle deaktivieren" }),
    ).not.toBeInTheDocument();
  });

  it("shows the compact completed status when all parent notifications are on", async () => {
    api.fetchNotificationPreferences.mockResolvedValue(
      preferences({ enabled: true }),
    );
    renderWithToast(<NotificationPreferencesSection portal="parent" />);

    expect(await screen.findByText("Alle aktiviert")).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Alle aktivieren" }),
    ).not.toBeInTheDocument();
  });

  it("keeps the parent action available when enabling all fails", async () => {
    api.setNotificationPreference.mockRejectedValue(
      new ApiError("diag", 503, { code: "general.unavailable" }),
    );
    renderWithToast(<NotificationPreferencesSection portal="parent" />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Alle aktivieren" }),
    );

    expect(
      await screen.findByText(
        catalogText(
          "general.unavailable",
          "das Einschalten aller Benachrichtigungen",
        ),
      ),
    ).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Alle aktivieren" }),
    ).toBeEnabled();
    expect(screen.queryByText("Alle aktiviert")).not.toBeInTheDocument();
  });

  it("shows a failed parent load in the card with retry", async () => {
    api.fetchNotificationPreferences
      .mockRejectedValueOnce(
        new ApiError("diag", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValue(preferences());
    renderWithToast(<NotificationPreferencesSection portal="parent" />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Benachrichtigungen"),
      ),
    ).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(
      await screen.findByRole("switch", { name: "Anstehende Abholung" }),
    ).toBeInTheDocument();
  });

  it("names the translated parent type when its switch fails", async () => {
    api.fetchNotificationPreferences.mockResolvedValue({
      tenant_enabled: true,
      types: [
        {
          key: "parent_message",
          label: "Backend-Name",
          description: "Backend-Text",
          group: "mitteilungen",
          enabled: false,
          available: true,
        },
      ],
    });
    api.setNotificationPreference.mockRejectedValue(
      new ApiError("diag", 500, { code: "general.server" }),
    );
    renderWithToast(<NotificationPreferencesSection portal="parent" />);

    fireEvent.click(
      await screen.findByRole("switch", { name: "Neue Nachricht der OGS" }),
    );

    expect(
      await screen.findByText(
        catalogText(
          "general.server",
          "die Benachrichtigung „Neue Nachricht der OGS“",
        ),
      ),
    ).toBeVisible();
  });

  it("uses the parent portal when asked to", async () => {
    renderWithToast(<NotificationPreferencesSection portal="parent" />);

    await waitFor(() =>
      expect(api.fetchNotificationPreferences).toHaveBeenCalledWith("parent"),
    );
  });
});
