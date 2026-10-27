import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { HolidayRegistrationDetail } from "~/lib/traeger-holiday-registrations-api";
import {
  HOLIDAY_PERIOD,
  HOLIDAY_REGISTRATION,
} from "~/test/fixtures/holiday-registrations";
import { schoolNames } from "./holiday-registration-format";

const { mockRebook } = vi.hoisted(() => ({ mockRebook: vi.fn() }));

vi.mock("~/lib/traeger-holiday-registrations-api", () => ({
  rebookHolidayChild: (...args: unknown[]) => mockRebook(...args),
}));

import { HolidayRegistrationChildCard } from "./holiday-registration-child-card";

const names = schoolNames([
  { id: "31", name: "OGS Nord", slug: "nord", subdomain: "nord", city: "" },
  { id: "32", name: "OGS Süd", slug: "sued", subdomain: "sued", city: "" },
]);

function renderCard(
  registration: HolidayRegistrationDetail,
  childIndex: number,
  onChanged = vi.fn(),
) {
  const child = registration.children[childIndex]!;
  render(
    <ToastProvider>
      <HolidayRegistrationChildCard
        period={HOLIDAY_PERIOD}
        registration={registration}
        child={child}
        health={registration.health.find((entry) => entry.childId === child.id)}
        names={names}
        onChanged={onChanged}
      />
    </ToastProvider>,
  );
  return onChanged;
}

describe("HolidayRegistrationChildCard", () => {
  beforeEach(() => {
    mockRebook.mockReset();
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", "traeger.localhost:3000");
  });
  afterEach(() => vi.unstubAllEnvs());

  it("shows school, weeks, health information and the duplicate hint", () => {
    renderCard(HOLIDAY_REGISTRATION, 0);

    expect(screen.getByText("Lia Muster")).toBeInTheDocument();
    expect(
      screen.getByText("Geboren am 01.03.2018 · OGS Nord"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("04.01. bis 08.01.2027: bis 16 Uhr, OGS Nord"),
    ).toBeInTheDocument();
    expect(screen.getByText("Erdnüsse")).toBeInTheDocument();
    expect(
      screen.getByText(/Vielleicht doppelt angemeldet\./),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Anmeldung Nr. 8 öffnen" }),
    ).toHaveAttribute("href", "/traeger/ferien/5/anmeldungen/8");
  });

  it("names a holiday guest with its school", () => {
    renderCard(HOLIDAY_REGISTRATION, 1);

    expect(
      screen.getByText(
        "Geboren am 12.09.2016 · Feriengast (Grundschule Melle)",
      ),
    ).toBeInTheDocument();
  });

  it("rebooks a child week by week", async () => {
    const rebooked = { ...HOLIDAY_REGISTRATION, history: [] };
    mockRebook.mockResolvedValue(rebooked);
    const onChanged = renderCard(HOLIDAY_REGISTRATION, 1);

    fireEvent.click(screen.getByRole("button", { name: "Umbuchen" }));
    const save = screen.getByRole("button", { name: "Speichern" });
    expect(save).toBeDisabled();

    fireEvent.click(
      screen.getByRole("combobox", {
        name: "Zeitmodell 04.01. bis 08.01.2027, OGS Nord",
      }),
    );
    fireEvent.click(await screen.findByRole("option", { name: "bis 16 Uhr" }));
    expect(save).toBeEnabled();
    fireEvent.click(save);

    await waitFor(() => expect(onChanged).toHaveBeenCalledWith(rebooked));
    expect(mockRebook).toHaveBeenCalledWith("5", "7", "10", [
      { weekId: "51", timeModelId: "62" },
      { weekId: "52", timeModelId: "63" },
    ]);
  });

  it("keeps a child in at least one week", async () => {
    renderCard(HOLIDAY_REGISTRATION, 1);

    fireEvent.click(screen.getByRole("button", { name: "Umbuchen" }));
    fireEvent.click(
      screen.getByRole("combobox", {
        name: "Zeitmodell 11.01. bis 15.01.2027, OGS Süd",
      }),
    );
    fireEvent.click(
      await screen.findByRole("option", { name: "Nicht angemeldet" }),
    );

    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
  });

  it("says how a frozen registration is rebooked", () => {
    renderCard({ ...HOLIDAY_REGISTRATION, frozen: true }, 0);

    expect(
      screen.getByText("Zum Umbuchen öffnen Sie die Anmeldung zuerst wieder."),
    ).toBeInTheDocument();
    expect(screen.getByText("Lia Muster")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Umbuchen" })).toBeNull();
  });
});
