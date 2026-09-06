import { render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import {
  getChildCourses,
  requestChildCourse,
  withdrawChildCourseRequest,
  type ChildCourses,
} from "~/lib/parent-api";
import { CoursesSection } from "./courses-section";

vi.mock("~/lib/parent-api", () => ({
  getChildCourses: vi.fn(),
  requestChildCourse: vi.fn(),
  withdrawChildCourseRequest: vi.fn(),
}));

const mockedCourses = vi.mocked(getChildCourses);
const mockedRequest = vi.mocked(requestChildCourse);
const mockedWithdraw = vi.mocked(withdrawChildCourseRequest);

function catalog(overrides: Partial<ChildCourses> = {}): ChildCourses {
  return {
    enabled: true,
    effective_from: "2026-09-21",
    pending_submitted_by_self: false,
    other_request_pending: false,
    items: [
      {
        id: "7",
        activity_group_id: "70",
        name: "Fußball",
        available_days: ["wed"],
        capacity: 20,
        free_slots: 4,
        booked: false,
        requested: false,
        waitlisted: false,
      },
    ],
    ...overrides,
  };
}

function renderSection() {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <CoursesSection studentId="1" careEnded={false} />
    </NextIntlClientProvider>,
  );
}

describe("CoursesSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("nennt den Vorgang eine Anfrage und zeigt die freien Plätze", async () => {
    mockedCourses.mockResolvedValue(catalog());

    renderSection();

    expect(await screen.findByText("Fußball")).toBeInTheDocument();
    expect(screen.getByText("Noch 4 Plätze frei")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Anfragen" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Kurse der OGS. Eine Anfrage gilt erst, wenn die OGS sie bestätigt.",
      ),
    ).toBeInTheDocument();
  });

  it("rendert nichts, wenn die Schule Kursanfragen aus hat", async () => {
    mockedCourses.mockResolvedValue(
      catalog({
        enabled: false,
        disabled_reason: "school_disabled",
        items: [],
      }),
    );

    const { container } = renderSection();

    await waitFor(() => expect(mockedCourses).toHaveBeenCalled());
    await waitFor(() => expect(container).toBeEmptyDOMElement());
  });

  it("sagt vor dem Tippen, dass ein voller Kurs auf die Warteliste führt", async () => {
    mockedCourses.mockResolvedValue(
      catalog({
        items: [
          {
            id: "7",
            activity_group_id: "70",
            name: "Fußball",
            available_days: ["wed"],
            capacity: 20,
            free_slots: 0,
            booked: false,
            requested: false,
            waitlisted: false,
          },
        ],
      }),
    );

    renderSection();

    expect(
      await screen.findByText("Voll. Eine Anfrage kommt auf die Warteliste."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Anfragen" }),
    ).toBeInTheDocument();
  });

  it("zeigt einen vollen Kurs als Wartelistenplatz, nicht als Zusage", async () => {
    mockedCourses.mockResolvedValue(
      catalog({
        pending_request_id: "55",
        pending_submitted_by_self: true,
        items: [
          {
            id: "7",
            activity_group_id: "70",
            name: "Fußball",
            available_days: ["wed"],
            capacity: 20,
            free_slots: 0,
            booked: false,
            requested: true,
            waitlisted: true,
            waitlist_position: 3,
          },
        ],
      }),
    );

    renderSection();

    expect(await screen.findByText("Warteliste, Platz 3")).toBeInTheDocument();
    expect(screen.getByText("Voll")).toBeInTheDocument();
    expect(screen.queryByText("Dabei")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Anfrage zurücknehmen" }),
    ).toBeInTheDocument();
  });

  it("bietet keine Rücknahme für die Anfrage eines anderen Elternteils", async () => {
    mockedCourses.mockResolvedValue(
      catalog({
        pending_request_id: "55",
        pending_submitted_by_self: false,
        items: [
          {
            id: "7",
            activity_group_id: "70",
            name: "Fußball",
            available_days: ["wed"],
            booked: false,
            requested: true,
            waitlisted: false,
          },
        ],
      }),
    );

    renderSection();

    expect(await screen.findByText("Angefragt")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Anfrage zurücknehmen" }),
    ).not.toBeInTheDocument();
  });

  it("sagt, warum gerade nichts angefragt werden kann", async () => {
    mockedCourses.mockResolvedValue(catalog({ other_request_pending: true }));

    renderSection();

    expect(
      await screen.findByText(
        "Sie haben schon eine offene Anfrage zur Betreuung. Bitte warten Sie die Antwort ab.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Anfragen" }),
    ).not.toBeInTheDocument();
  });

  it("erklärt einen leeren Abschnitt, statt ihn wortlos zu zeigen", async () => {
    mockedCourses.mockResolvedValue(
      catalog({ enabled: false, disabled_reason: "no_enrollment", items: [] }),
    );

    renderSection();

    expect(
      await screen.findByText(
        "Kurse können Sie anfragen, sobald Ihr Kind angemeldet ist.",
      ),
    ).toBeInTheDocument();
  });

  it("meldet einen Fehlschlag freundlich und behält die Liste", async () => {
    mockedCourses.mockResolvedValue(catalog());
    mockedRequest.mockRejectedValue(new Error("boom"));

    renderSection();
    (await screen.findByRole("button", { name: "Anfragen" })).click();

    expect(
      await screen.findByText(
        "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Fußball")).toBeInTheDocument();
    expect(mockedWithdraw).not.toHaveBeenCalled();
  });
});
