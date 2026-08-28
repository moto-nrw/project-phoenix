import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import de from "~/i18n/messages/de.json";

// The global next-intl/server mock only provides getLocale; the root
// not-found page resolves its copy via getTranslations.
vi.mock("next-intl/server", () => ({
  getLocale: () => Promise.resolve("de"),
  getTranslations: (namespace: string) =>
    Promise.resolve((key: string) => {
      const catalog = (de as unknown as Record<string, Record<string, string>>)[
        namespace
      ];
      const value = catalog?.[key];
      if (!value) throw new Error(`missing message ${namespace}.${key}`);
      return value;
    }),
}));

import NotFound from "./not-found";
import { TenantNotFoundScreen } from "~/components/tenant/tenant-not-found-screen";

describe("root not-found page", () => {
  it("renders the German 404 copy with both actions", async () => {
    render(await NotFound());

    expect(
      screen.getByRole("heading", { name: "Seite nicht gefunden" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Zurück" })).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Zur Startseite" }),
    ).toHaveAttribute("href", "/");
  });
});

describe("TenantNotFoundScreen (unknown school subdomain)", () => {
  it("renders the school-specific copy without a home link", () => {
    render(<TenantNotFoundScreen />);

    expect(
      screen.getByRole("heading", { name: "Schule nicht gefunden" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Zurück" })).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
