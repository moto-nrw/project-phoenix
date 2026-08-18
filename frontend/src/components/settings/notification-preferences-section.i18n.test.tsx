import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import en from "~/i18n/messages/en.json";
import { NotificationPreferencesSection } from "./notification-preferences-section";

const api = vi.hoisted(() => ({
  fetchNotificationPreferences: vi.fn(),
  setNotificationPreference: vi.fn(),
  disableAllNotificationPreferences: vi.fn(),
}));

vi.mock("~/lib/notification-preferences-api", () => api);
vi.mock("next-intl", () => ({
  useTranslations: (namespace: keyof typeof en) => {
    const messages = en[namespace] as Record<string, unknown>;
    return (key: string) =>
      key.split(".").reduce<unknown>((value, part) => {
        if (value && typeof value === "object") {
          return (value as Record<string, unknown>)[part];
        }
        return undefined;
      }, messages) as string;
  },
}));

describe("parent notification preference translations", () => {
  beforeEach(() => {
    api.fetchNotificationPreferences.mockResolvedValue({
      tenant_enabled: true,
      types: [
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
  });

  it("renders parent labels from the active catalog instead of backend German", async () => {
    render(<NotificationPreferencesSection portal="parent" />);

    expect(await screen.findByText("Notifications")).toBeVisible();
    expect(screen.getByText("Messages")).toBeVisible();
    expect(
      screen.getByRole("switch", { name: "New message from the OGS" }),
    ).toBeVisible();
    expect(
      screen.queryByText("Neue Nachricht der OGS"),
    ).not.toBeInTheDocument();
  });
});
