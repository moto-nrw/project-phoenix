import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import {
  HOLIDAY_PERIOD,
  HOLIDAY_REGISTRATION,
} from "~/test/fixtures/holiday-registrations";
import { HolidayRegistrationChildCard } from "./holiday-registration-child-card";
import { knownChildrenKey, knownChildView } from "./holiday-known-child";
import { schoolNames } from "./holiday-registration-format";

vi.mock("~/lib/traeger-holiday-registrations-api", () => ({}));

const names = schoolNames([
  { id: "31", name: "OGS Nord", slug: "nord", subdomain: "nord", city: "" },
  { id: "32", name: "OGS Süd", slug: "sued", subdomain: "sued", city: "" },
]);

function renderCard(
  childIndex: number,
  knownSchoolId: string | undefined,
  onEdit?: () => void,
  registration = HOLIDAY_REGISTRATION,
) {
  render(
    <ToastProvider>
      <HolidayRegistrationChildCard
        period={HOLIDAY_PERIOD}
        registration={registration}
        child={HOLIDAY_REGISTRATION.children[childIndex]!}
        health={undefined}
        names={names}
        knownSchoolId={knownSchoolId}
        onChanged={vi.fn()}
        onEdit={onEdit}
      />
    </ToastProvider>,
  );
}

describe("hint on a known child (#6683)", () => {
  beforeEach(() =>
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", "traeger.localhost:3000"),
  );
  afterEach(() => vi.unstubAllEnvs());

  it("confirms the school the parents named with a plaque", () => {
    renderCard(0, "31");

    expect(screen.getByText("An der Schule bekannt")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Schule von/ })).toBeNull();
  });

  it("asks to check the school of a holiday guest another school knows", () => {
    const onEdit = vi.fn();
    renderCard(1, "32", onEdit);

    expect(
      screen.getByText(
        "Die Schule „OGS Süd“ führt ein Kind mit gleichem Namen und Geburtsdatum. Angemeldet ist Ben als Feriengast. Bitte prüfen Sie die Schule.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Schule von Ben Muster ändern" }),
    );
    expect(onEdit).toHaveBeenCalledTimes(1);
  });

  it("shows nothing for a child no school knows", () => {
    renderCard(1, undefined);

    // Anker: die Karte ist geladen.
    expect(screen.getByText("Ben Muster")).toBeInTheDocument();
    expect(screen.queryByText(/bekannt/)).not.toBeInTheDocument();
  });

  it("shows no hint once the registration is cancelled", () => {
    renderCard(1, "32", vi.fn(), {
      ...HOLIDAY_REGISTRATION,
      status: "cancelled",
    });

    // Anker: die Karte ist geladen.
    expect(screen.getByText("Ben Muster")).toBeInTheDocument();
    expect(
      screen.queryByText(/führt ein Kind mit gleichem Namen/),
    ).not.toBeInTheDocument();
  });

  it("names the other school of a child registered at a school", () => {
    const lia = HOLIDAY_REGISTRATION.children[0]!;
    expect(knownChildView(lia, "32", names)).toMatchObject({
      sameSchool: false,
      tone: "blue",
      label: "Bekannt an OGS Süd",
      title:
        "In der Anmeldung steht eine andere Schule. Die Schule „OGS Süd“ führt ein Kind mit gleichem Namen und Geburtsdatum.",
    });
    expect(knownChildView(lia, "99", names)).toBeNull();
  });

  it("asks again only when names or dates of birth change", () => {
    const children = HOLIDAY_REGISTRATION.children;
    const key = knownChildrenKey(children);
    expect(
      knownChildrenKey(
        children.map((child) => ({ ...child, holidayWish: "yes" })),
      ),
    ).toBe(key);
    expect(
      knownChildrenKey(
        children.map((child, i) =>
          i === 0 ? { ...child, lastName: "Neu" } : child,
        ),
      ),
    ).not.toBe(key);
    expect(knownChildrenKey(undefined)).toBe("");
  });
});
