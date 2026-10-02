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
    render(<MessageCountSection />);

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
    render(<MessageCountSection />);

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
    mockSaveScope.mockRejectedValue(new Error("network"));
    render(<MessageCountSection />);

    fireEvent.click(
      await screen.findByRole("radio", { name: /Keine Zahl anzeigen/ }),
    );

    expect(
      await screen.findByText(
        "Die Einstellung konnte nicht gespeichert werden.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("radio", { name: /Alle Nachrichten/ }),
    ).toBeChecked();
    expect(unreadRefreshes).toBe(0);
  });

  it("says when the own groups would count nothing", async () => {
    mockFetchSetting.mockResolvedValue({ scope: "all", hasOwnGroups: false });
    render(<MessageCountSection />);

    expect(
      await screen.findByText(
        "Sie gehören zurzeit zu keiner Gruppe. Dann zeigt die Zahl nichts.",
      ),
    ).toBeInTheDocument();
  });

  it("does not repeat the hint for a person with a group", async () => {
    render(<MessageCountSection />);

    await screen.findByRole("radio", { name: /Alle Nachrichten/ });
    expect(
      screen.queryByText(/Sie gehören zurzeit zu keiner Gruppe/),
    ).not.toBeInTheDocument();
  });

  it("stays hidden while the school has messaging off", () => {
    mockTenant.messagingEnabled = false;
    const { container } = render(<MessageCountSection />);

    expect(container).toBeEmptyDOMElement();
    expect(mockFetchSetting).not.toHaveBeenCalled();
  });

  it("stays hidden when the setting cannot be loaded", async () => {
    mockFetchSetting.mockRejectedValue(new Error("forbidden"));
    const { container } = render(<MessageCountSection />);

    await waitFor(() => expect(container).toBeEmptyDOMElement());
  });
});
