import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import { Suspense } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.generated";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import {
  bookChildParentConference,
  fetchChildParentConferences,
  listMyChildren,
} from "~/lib/parent-api";
import type { Child } from "~/lib/parent-api";
import type { ParentConferenceOffer } from "~/lib/parent-conferences";
import { catalogText } from "~/test/error-catalog-text";
import ParentConferencePage from "./page";

vi.mock("~/lib/parent-api", () => ({
  listMyChildren: vi.fn(),
  fetchChildParentConferences: vi.fn(),
  bookChildParentConference: vi.fn(),
  cancelChildParentConference: vi.fn(),
}));
vi.mock("~/lib/parent-url", () => ({ parentPath: (path: string) => path }));

const offer: ParentConferenceOffer = {
  id: "5",
  school_class: "2a",
  teacher_name: "Lena Lehrerin",
  date: "2026-11-12",
  location: "Raum 12",
  free_slots: [
    { start_time: "14:00", end_time: "14:15" },
    { start_time: "14:30", end_time: "14:45" },
  ],
  can_change: true,
};

async function renderPage() {
  await act(async () => {
    render(
      <NextIntlClientProvider locale="de" messages={deMessages}>
        <ToastProvider>
          <Suspense fallback={null}>
            <ParentConferencePage params={Promise.resolve({ id: "42" })} />
          </Suspense>
        </ToastProvider>
      </NextIntlClientProvider>,
    );
  });
}

describe("Elternsprechtag in der Eltern-App (#4801)", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(listMyChildren).mockResolvedValue([
      { student_id: "42", first_name: "Mia" } as Child,
    ]);
  });

  it("shows only the free times and books one for the child", async () => {
    vi.mocked(fetchChildParentConferences).mockResolvedValue([offer]);
    vi.mocked(bookChildParentConference).mockResolvedValue({
      ...offer,
      free_slots: [{ start_time: "14:30", end_time: "14:45" }],
      booking: { start_time: "14:00", end_time: "14:15" },
    });
    await renderPage();

    expect(
      await screen.findByText("Lehrkraft: Lena Lehrerin · Raum: Raum 12"),
    ).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "14:00" }));

    await waitFor(() =>
      expect(bookChildParentConference).toHaveBeenCalledWith(
        "42",
        "5",
        "14:00",
      ),
    );
    expect(await screen.findByText("14:00–14:15 Uhr")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Termin absagen" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "14:00" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "14:30" })).toBeInTheDocument();
  });

  it("tells the family to call the teacher within 24 hours", async () => {
    vi.mocked(fetchChildParentConferences).mockResolvedValue([
      {
        ...offer,
        free_slots: [],
        booking: { start_time: "14:00", end_time: "14:15" },
        can_change: false,
      },
    ]);
    await renderPage();

    expect(
      await screen.findByText(/weniger als 24 Stunden/),
    ).toBeInTheDocument();
    expect(screen.getByText("14:00–14:15 Uhr")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Termin absagen" }),
    ).not.toBeInTheDocument();
  });

  it("shows why a parent without the relationship sees nothing", async () => {
    vi.mocked(fetchChildParentConferences).mockRejectedValue(
      new ApiError("diag", 403, { code: "care.child_not_linked" }),
    );
    await renderPage();

    expect(
      await screen.findByText(
        catalogText("care.child_not_linked", "den Elternsprechtag"),
        { exact: false },
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Elternsprechtag")).toBeInTheDocument();
  });
});
