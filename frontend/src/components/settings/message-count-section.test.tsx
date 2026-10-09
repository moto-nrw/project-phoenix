import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { mockFetchSetting, mockSaveScope, mockTenant } = vi.hoisted(() => ({
  mockFetchSetting: vi.fn(),
  mockSaveScope: vi.fn(),
  mockTenant: { messagingEnabled: true },
}));

vi.mock("~/lib/parent-messages-api", () => ({
  fetchMessageCountSetting: mockFetchSetting,
  saveMessageCountScope: mockSaveScope,
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSafe: () => ({ tenant: mockTenant }),
}));

import { MessageCountSection } from "./message-count-section";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError, unavailableApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const OBJECT = "die Einstellung zur Zahl bei Nachrichten";

// The shared error path shows failures through the toast provider (#2517).
function renderWithToast(
  ui: Parameters<typeof render>[0],
  options?: Parameters<typeof render>[1],
) {
  return render(ui, { wrapper: ToastProvider, ...options });
}

describe("MessageCountSection", () => {
  let unreadRefreshes: number;
  const countRefresh = () => {
    unreadRefreshes++;
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockTenant.messagingEnabled = true;
    mockFetchSetting.mockResolvedValue({ scope: "all", hasOwnGroups: true });
    mockSaveScope.mockResolvedValue(undefined);
    unreadRefreshes = 0;
    window.addEventListener("messages-unread-refresh", countRefresh);
  });

  afterEach(() => {
    window.removeEventListener("messages-unread-refresh", countRefresh);
  });

  it("shows the stored choice", async () => {
    mockFetchSetting.mockResolvedValue({
      scope: "own_groups",
      hasOwnGroups: true,
    });
    renderWithToast(<MessageCountSection />);

    expect(
      await screen.findByRole("radio", {
        name: /Nur Kinder aus meinen Gruppen/,
      }),
    ).toBeChecked();
    expect(
      screen.getByRole("radio", { name: /Alle Nachrichten/ }),
    ).not.toBeChecked();
    expect(screen.getByText("Zahl bei Nachrichten")).toBeInTheDocument();
  });

  it("saves a new choice and refreshes the own counter", async () => {
    renderWithToast(<MessageCountSection />);

    fireEvent.click(
      await screen.findByRole("radio", { name: /Keine Zahl anzeigen/ }),
    );

    await waitFor(() => expect(mockSaveScope).toHaveBeenCalledWith("none"));
    await waitFor(() => expect(unreadRefreshes).toBe(1));
    expect(
      screen.getByRole("radio", { name: /Keine Zahl anzeigen/ }),
    ).toBeChecked();
  });

  it("rolls back and says so when saving fails", async () => {
    mockSaveScope.mockRejectedValue(unavailableApiError(new Error("network")));
    renderWithToast(<MessageCountSection />);

    fireEvent.click(
      await screen.findByRole("radio", { name: /Keine Zahl anzeigen/ }),
    );

    // #2517: a toast with the catalog text, no box in the card.
    expect(
      await screen.findByText(catalogText("general.unavailable", OBJECT)),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("radio", { name: /Alle Nachrichten/ }),
    ).toBeChecked();
    expect(unreadRefreshes).toBe(0);
  });

  it("says when the own groups would count nothing", async () => {
    mockFetchSetting.mockResolvedValue({ scope: "all", hasOwnGroups: false });
    renderWithToast(<MessageCountSection />);

    expect(
      await screen.findByText(
        "Sie gehören zurzeit zu keiner Gruppe. Dann zeigt die Zahl nichts.",
      ),
    ).toBeInTheDocument();
  });

  it("does not repeat the hint for a person with a group", async () => {
    renderWithToast(<MessageCountSection />);

    await screen.findByRole("radio", { name: /Alle Nachrichten/ });
    expect(
      screen.queryByText(/Sie gehören zurzeit zu keiner Gruppe/),
    ).not.toBeInTheDocument();
  });

  it("stays hidden while the school has messaging off", () => {
    mockTenant.messagingEnabled = false;
    renderWithToast(<MessageCountSection />);

    expect(screen.queryByText("Zahl bei Nachrichten")).not.toBeInTheDocument();
    expect(mockFetchSetting).not.toHaveBeenCalled();
  });

  it("stays hidden for an account without read access", async () => {
    mockFetchSetting.mockRejectedValue(new ApiError("forbidden", 403));
    renderWithToast(<MessageCountSection />);

    await waitFor(() => expect(mockFetchSetting).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(
        screen.queryByText("Zahl bei Nachrichten"),
      ).not.toBeInTheDocument(),
    );
  });

  // #2517: any other failure is shown in place with retry, never hidden.
  it("shows a failed load in the card and retries", async () => {
    mockFetchSetting.mockRejectedValueOnce(new ApiError("boom", 500));
    renderWithToast(<MessageCountSection />);

    expect(
      await screen.findByText(catalogText("general.server", OBJECT)),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(
      await screen.findByRole("radio", { name: /Alle Nachrichten/ }),
    ).toBeChecked();
    expect(mockFetchSetting).toHaveBeenCalledTimes(2);
  });
});
