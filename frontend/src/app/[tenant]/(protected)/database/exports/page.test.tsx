import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useSession } from "next-auth/react";
import { useModuleBooked } from "~/lib/tenant-context";

import DatabaseExportsPage from "./page";

vi.mock("next-auth/react", () => ({ useSession: vi.fn() }));

vi.mock("~/components/ui/tenant-page", () => ({
  TenantPage: ({
    children,
    stats,
  }: {
    children: ReactNode;
    stats: ReactNode;
  }) => (
    <main>
      <output>{stats}</output>
      {children}
    </main>
  ),
}));

vi.mock("~/components/ui/section-card", () => ({
  SectionCard: ({
    children,
    title,
  }: {
    children: ReactNode;
    title?: string;
  }) => (
    <section>
      {title && <h2>{title}</h2>}
      {children}
    </section>
  ),
}));

vi.mock("~/components/ui/collection-grid", () => ({
  CollectionGrid: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

vi.mock("~/components/ui/info-card", () => ({
  InfoCard: ({ children, title }: { children: ReactNode; title: string }) => (
    <article>
      <h3>{title}</h3>
      {children}
    </article>
  ),
}));

vi.mock("~/components/ui/navigation-link", () => ({
  default: ({ children, href }: { children: ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock("~/components/students/student-export-modal", () => ({
  StudentExportModal: () => null,
}));

vi.mock("~/components/staff/staff-birthday-export-modal", () => ({
  StaffBirthdayExportModal: () => null,
}));

vi.mock("~/components/ui/export-menu", () => ({
  ExportMenu: () => null,
  exportFormatOptions: () => [],
}));

vi.mock("~/lib/hooks/use-settings-schema", () => ({
  useSettingsSchema: () => ({ data: null }),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: vi.fn() }),
  useApiErrorDisplay: () => ({ show: vi.fn() }),
}));

describe("DatabaseExportsPage", () => {
  const booked = new Set<string>();

  beforeEach(() => {
    booked.clear();
    for (const moduleKey of [
      "online_anmeldung",
      "zeiterfassung",
      "dienstplan",
    ]) {
      booked.add(moduleKey);
    }
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: { roles: ["admin"], permissions: ["admin:*"] },
      },
      status: "authenticated",
    } as never);
    vi.mocked(useModuleBooked).mockImplementation((module) =>
      booked.has(module),
    );
  });

  it("shows exports from booked modules and counts their links", () => {
    render(<DatabaseExportsPage />);

    expect(
      screen.getByRole("link", { name: "Zu den Anmeldephasen" }),
    ).toBeVisible();
    expect(screen.getByRole("link", { name: "Zum Personal" })).toBeVisible();
    expect(screen.getByText(/5 auf anderen Seiten/)).toBeVisible();
  });

  it("hides exports from modules the school did not book", () => {
    booked.delete("online_anmeldung");
    booked.delete("zeiterfassung");

    render(<DatabaseExportsPage />);

    // Anker: Die Basis-Verweise bleiben auf der geladenen Seite sichtbar.
    expect(
      screen.getByRole("link", { name: "Zu den Tageslisten" }),
    ).toBeVisible();
    expect(
      screen.queryByRole("link", { name: "Zu den Anmeldephasen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Zum Personal" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText(/3 auf anderen Seiten/)).toBeVisible();
  });
});
