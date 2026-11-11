import { render, screen } from "@testing-library/react";
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";

import { NotBookedSlotProvider, type NotBookedSlots } from "./not-booked-slot";
import { PagePathProvider } from "./page-path";
import { PageBody } from "./page-body";
import { TenantPage } from "./tenant-page";

// The test setup resolves app paths for the test school; these tests check
// the kit's own resolution.
vi.unmock("~/components/ui/page-path");

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: ComponentProps<"a">) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

// Die Naht für „Modul nicht gebucht“ (#6577): Das Kit kennt kein
// Modul-Register und keinen Tenant. Was eine Seite gesperrt macht und wie
// die Erklärung lautet, reicht der Aufrufer über den Slot herein.
const SLOTS: NotBookedSlots = {
  Alert: ({ module }) => <p>Hinweis zu {module}</p>,
  State: ({ module }) => <p>Zustand für {module}</p>,
};

describe("NotBookedSlot im Seitengerüst", () => {
  it("zeigt im Seitenrumpf den Zustand für das Modul der Seite", () => {
    render(
      <NotBookedSlotProvider slots={{ ...SLOTS, pageModule: "dienstplan" }}>
        <PageBody loading={false} loadingLabel="Lädt">
          <p>Inhalt</p>
        </PageBody>
      </NotBookedSlotProvider>,
    );

    expect(screen.getByText("Zustand für dienstplan")).toBeVisible();
    expect(screen.queryByText("Inhalt")).not.toBeInTheDocument();
  });

  it("zeigt den Inhalt, wenn die Seite kein gesperrtes Modul hat", () => {
    render(
      <NotBookedSlotProvider slots={{ ...SLOTS, pageModule: null }}>
        <PageBody loading={false} loadingLabel="Lädt">
          <p>Inhalt</p>
        </PageBody>
      </NotBookedSlotProvider>,
    );

    expect(screen.getByText("Inhalt")).toBeVisible();
  });

  it("behält im gesperrten Gerüst Titel und Rückweg, ohne Aktionen und Inhalt", () => {
    render(
      <NotBookedSlotProvider slots={{ ...SLOTS, pageModule: "dienstplan" }}>
        <TenantPage
          title="Dienstplan"
          back
          backHref="/rooms"
          backLabel="Zurück zu den Räumen"
          stats="12 Dienste"
          actions={<button type="button">Anlegen</button>}
        >
          <p>Inhalt</p>
        </TenantPage>
      </NotBookedSlotProvider>,
    );

    expect(
      screen.getByRole("heading", { level: 1, name: "Dienstplan" }),
    ).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Zurück zu den Räumen" }),
    ).toBeVisible();
    expect(screen.getByText("Zustand für dienstplan")).toBeVisible();
    expect(screen.queryByText("Inhalt")).not.toBeInTheDocument();
    expect(screen.queryByText("12 Dienste")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Anlegen" }),
    ).not.toBeInTheDocument();
  });
});

describe("Rückweg im Seitengerüst", () => {
  it("nimmt den Pfad ohne Pfadauflösung so, wie er übergeben wird", () => {
    render(<TenantPage title="Standort" back backHref="/traeger/schulen" />);

    expect(screen.getByRole("link", { name: "Zurück" })).toHaveAttribute(
      "href",
      "/traeger/schulen",
    );
  });

  it("löst den Pfad über die eingereichte Pfadauflösung auf", () => {
    render(
      <PagePathProvider resolve={(path) => `/schule-a${path}`}>
        <TenantPage title="Raum 1" back backHref="/rooms" />
      </PagePathProvider>,
    );

    expect(screen.getByRole("link", { name: "Zurück" })).toHaveAttribute(
      "href",
      "/schule-a/rooms",
    );
  });
});
