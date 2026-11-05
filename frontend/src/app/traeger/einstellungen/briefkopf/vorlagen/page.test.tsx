import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";

const { templatesApi, push } = vi.hoisted(() => ({
  templatesApi: { fetchDocumentTemplates: vi.fn() },
  push: vi.fn(),
}));

vi.mock("~/lib/traeger-document-templates", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("~/lib/traeger-document-templates")
  >()),
  ...templatesApi,
}));
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
  usePathname: () => "/einstellungen/briefkopf/vorlagen",
}));

import TraegerVorlagenPage from "./page";

describe("TraegerVorlagenPage", () => {
  beforeEach(() => {
    push.mockReset();
    templatesApi.fetchDocumentTemplates.mockResolvedValue([
      {
        kind: "bestaetigung",
        version: 2,
        standard: false,
        updatedAt: "2026-09-08T10:00:00Z",
      },
      { kind: "rechnung", version: 0, standard: true, updatedAt: null },
    ]);
  });

  it("lists every kind with its template and opens one", async () => {
    render(
      <ToastProvider>
        <TraegerVorlagenPage />
      </ToastProvider>,
    );
    // The table renders a desktop and a stacked phone layout.
    const [confirmation] = await screen.findAllByText(
      "Bestätigung der Ferienanmeldung",
    );
    expect(screen.getAllByText("Eigene, Version 2").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Standardvorlage").length).toBeGreaterThan(0);
    expect(screen.getAllByText("08.09.2026").length).toBeGreaterThan(0);
    fireEvent.click(confirmation!);
    expect(push).toHaveBeenCalledWith(
      "/einstellungen/briefkopf/vorlagen/bestaetigung",
    );
  });
});
