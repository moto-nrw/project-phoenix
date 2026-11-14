import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const tab = vi.hoisted(() => ({ value: null as string | null }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => ({ get: () => tab.value }),
}));
vi.mock("~/components/ui/tenant-page", () => ({
  TenantPage: ({
    title,
    children,
  }: {
    title: string;
    children: React.ReactNode;
  }) => (
    <main>
      <h1>{title}</h1>
      {children}
    </main>
  ),
}));
// Each section has its own tests; here only which one a tab opens.
vi.mock("~/components/settings/notification-preferences-section", () => ({
  NotificationPreferencesSection: () => <section aria-label="Themen" />,
}));
vi.mock("~/components/settings/push-notification-section", () => ({
  PushNotificationSection: () => <section aria-label="Dieses Gerät" />,
}));
vi.mock("~/components/settings/analytics-consent-section", () => ({
  SchoolAnalyticsConsentSection: () => <section aria-label="Nutzungsdaten" />,
}));
vi.mock("~/components/settings/trusted-devices-section", () => ({
  TrustedDevicesSection: ({ scope }: { scope: string }) => (
    <section aria-label={`Gemerkte Geräte (${scope})`} />
  ),
}));

import SchoolSettingsPage from "./page";

describe("SchoolSettingsPage", () => {
  beforeEach(() => {
    tab.value = null;
  });

  it("opens the notifications without a tab", () => {
    render(<SchoolSettingsPage />);
    expect(
      screen.getByRole("heading", { name: "Benachrichtigungen" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Themen" })).toBeInTheDocument();
  });

  // #6955: a Lehrkraft sees and removes the devices the school portal
  // remembered, on the school session.
  it("lists the remembered devices of the school portal in their own section", () => {
    tab.value = "devices";
    render(<SchoolSettingsPage />);
    expect(
      screen.getByRole("heading", { name: "Vertraute Geräte" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("region", { name: "Gemerkte Geräte (school)" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("region", { name: "Themen" }),
    ).not.toBeInTheDocument();
  });

  it("keeps the privacy section on its tab", () => {
    tab.value = "privacy";
    render(<SchoolSettingsPage />);
    expect(
      screen.getByRole("region", { name: "Nutzungsdaten" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("region", { name: "Gemerkte Geräte (school)" }),
    ).not.toBeInTheDocument();
  });
});
