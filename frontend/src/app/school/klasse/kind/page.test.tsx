import { act, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.unmock("swr");

const { fetchNotes, swr } = vi.hoisted(() => ({
  fetchNotes: vi.fn(),
  swr: { mutate: undefined as undefined | (() => Promise<unknown>) },
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams("klasse=3b&kind=42"),
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "teacher", tenantId: 7, scope: "school" } },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => null,
}));
vi.mock("~/lib/school-url", () => ({ schoolPath: (path: string) => path }));
vi.mock("~/lib/school-class-day-api", () => ({
  fetchSharedNotesSchool: fetchNotes,
}));
vi.mock("~/lib/swr", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/swr")>();
  return {
    ...actual,
    useSWRAuth: (...args: Parameters<typeof actual.useSWRAuth>) => {
      const result = actual.useSWRAuth(...args);
      swr.mutate = result.mutate;
      return result;
    },
  };
});

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { SWRConfig } from "~/lib/swr/testing";
import Page from "./page";

const notes = {
  student_id: "42",
  first_name: "Ida",
  last_name: "Beobachtung",
  school_class: "3b",
  notes: [{ id: "5", body: "Freigegebene Beobachtung", author_name: "" }],
};

describe("shared notes disclosure", () => {
  beforeEach(() => {
    fetchNotes.mockReset();
    fetchNotes.mockResolvedValue(notes);
  });

  // Keep the parent cache alive across page openings, as the portal does.
  function Opening({ open }: { open: boolean }) {
    return (
      <SWRConfig value={{ provider: () => new Map() }}>
        <ToastProvider>{open ? <Page /> : null}</ToastProvider>
      </SWRConfig>
    );
  }

  it("requires a fresh successful read on reopening within the deduplication interval", async () => {
    const view = render(<Opening open />);
    expect(
      await screen.findByText("Freigegebene Beobachtung"),
    ).toBeInTheDocument();
    view.rerender(<Opening open={false} />);
    let resolveRead!: (value: typeof notes) => void;
    fetchNotes.mockReturnValue(
      new Promise<typeof notes>((resolve) => {
        resolveRead = resolve;
      }),
    );
    view.rerender(<Opening open />);

    // Anker: die neue Öffnung wartet auf ihren eigenen Abruf.
    expect(
      await screen.findByLabelText("Beobachtungen werden geladen…"),
    ).toBeInTheDocument();
    expect(fetchNotes).toHaveBeenCalledTimes(2);
    expect(
      screen.queryByText("Freigegebene Beobachtung"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Ida Beobachtung")).not.toBeInTheDocument();
    await act(async () => resolveRead({ ...notes, notes: [] }));
    expect(await screen.findByText("Keine Beobachtungen")).toBeInTheDocument();
    expect(
      screen.queryByText("Freigegebene Beobachtung"),
    ).not.toBeInTheDocument();
  });

  it("removes the child's name and notes after a denied revalidation and revisit", async () => {
    const view = render(<Opening open />);
    expect(
      await screen.findByText("Freigegebene Beobachtung"),
    ).toBeInTheDocument();
    fetchNotes.mockRejectedValue(new ApiError("Forbidden", 403));
    await act(async () => {
      await swr.mutate?.();
    });

    // Anker: der fehlgeschlagene Abruf wird als Ladefehler angezeigt.
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(
      screen.queryByText("Freigegebene Beobachtung"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Ida Beobachtung")).not.toBeInTheDocument();
    view.rerender(<Opening open={false} />);
    view.rerender(<Opening open />);
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(
      screen.queryByText("Freigegebene Beobachtung"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Ida Beobachtung")).not.toBeInTheDocument();
  });
});
