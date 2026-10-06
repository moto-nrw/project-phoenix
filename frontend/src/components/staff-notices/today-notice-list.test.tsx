import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { StaffNotice } from "~/lib/staff-notices-api";
import { catalogText } from "~/test/error-catalog-text";

import { TodayNoticeList } from "./today-notice-list";

const notice: StaffNotice = {
  id: "42",
  title: "Dienstbesprechung",
  body: "Heute um 14 Uhr im Lehrerzimmer.",
  priority: "info",
  audience: "all",
  valid_from: "2026-09-09",
  weekdays: [],
  week_pattern: 0,
  requires_acknowledgement: true,
  active: true,
};

describe("TodayNoticeList", () => {
  // #2517: eine Aktion ohne Formular meldet den Fehler mit Katalogtext und
  // Wiederholen, nicht mit dem Satz des Servers.
  it("shows a failed acknowledgement as a message and retries it", async () => {
    const acknowledge = vi
      .fn()
      .mockRejectedValueOnce(
        new ApiError("ack kaputt", 500, { code: "general.server" }),
      )
      .mockResolvedValueOnce(undefined);
    const onChanged = vi.fn().mockResolvedValue(undefined);
    render(
      <TodayNoticeList
        notices={[notice]}
        onChanged={onChanged}
        acknowledge={acknowledge}
      />,
      { wrapper: ToastProvider },
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Zur Kenntnis nehmen" }),
    );

    expect(
      await screen.findByText(
        catalogText("general.server", "die Kenntnisnahme"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/kaputt/)).not.toBeInTheDocument();
    expect(onChanged).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: /Wiederholen/ }));

    await waitFor(() => expect(onChanged).toHaveBeenCalledOnce());
    expect(acknowledge).toHaveBeenCalledTimes(2);
    expect(acknowledge).toHaveBeenLastCalledWith("42");
  });
});
