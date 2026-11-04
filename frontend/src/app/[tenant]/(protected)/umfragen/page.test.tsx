import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import type { Survey, SurveyToAnswer } from "~/lib/surveys-api";

import SurveysPage from "./page";

const state = vi.hoisted(() => ({
  canManage: false,
  search: "",
  toAnswer: [] as SurveyToAnswer[],
  surveys: [] as Survey[],
  // Like useSWRAuth before the session is known: the key is held back, so
  // there is neither data nor a running request.
  sessionPending: false,
  push: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  usePathname: () => "/umfragen",
  useSearchParams: () => new URLSearchParams(state.search),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { user: { id: "7" } }, status: "authenticated" }),
}));

vi.mock("~/lib/auth-utils", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/auth-utils")>()),
  hasPermission: (_session: unknown, permission: string) =>
    permission === "surveys:manage" && state.canManage,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: state.push, replace: vi.fn() }),
}));

vi.mock("~/lib/swr", () => ({
  useTenantMutate: () => vi.fn(),
  useSWRAuth: (key: string | null) => ({
    data: state.sessionPending
      ? undefined
      : key === "surveys-to-answer"
        ? state.toAnswer
        : key === "surveys-list"
          ? state.surveys
          : undefined,
    error: undefined,
    isLoading: false,
    mutate: vi.fn(() => Promise.resolve(undefined)),
  }),
}));

vi.mock("~/lib/surveys-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/surveys-api")>()),
  fetchSurveyForm: vi.fn(() =>
    Promise.resolve({
      school_name: "OGS am Park",
      participated: false,
      survey: {
        id: "11",
        title: "Teamklima",
        intro: "Wie geht es Ihnen im Team?",
        anonymous: true,
        closes_at: "2026-09-30T16:00:00Z",
        questions: [
          {
            id: "q1",
            position: 1,
            kind: "scale",
            prompt: "Ich fühle mich im Team wohl.",
            required: true,
            options: [],
          },
        ],
      },
    }),
  ),
}));

const openSurvey: SurveyToAnswer = {
  id: "11",
  title: "Teamklima",
  intro: "Wie geht es Ihnen im Team?",
  anonymous: true,
  closes_at: "2026-09-30T16:00:00Z",
  participated: false,
};

const managed: Survey = {
  id: "21",
  title: "Elternzufriedenheit",
  intro: "",
  audience: "parents",
  anonymous: true,
  send_email: false,
  status: "open",
  closes_at: "2026-09-30T16:00:00Z",
  published_at: "2026-09-01T08:00:00Z",
  created_at: "2026-08-30T08:00:00Z",
  updated_at: "2026-09-01T08:00:00Z",
  participant_count: 3,
  targets: [{ target_type: "school_all" }],
  questions: [],
};

function renderPage() {
  return render(
    <ToastProvider>
      <SurveysPage />
    </ToastProvider>,
  );
}

describe("Umfragen (#4781)", () => {
  beforeEach(() => {
    state.canManage = false;
    state.search = "";
    state.toAnswer = [openSurvey];
    state.surveys = [managed];
    state.sessionPending = false;
    state.push.mockReset();
  });

  it("lists the open team survey for every staff member, without management", async () => {
    renderPage();

    // Anker: die offene Umfrage für die Person ist geladen.
    expect(await screen.findByText("Teamklima")).toBeInTheDocument();
    expect(screen.getByText("Antwort offen")).toBeInTheDocument();
    expect(screen.getByText("Anonym")).toBeInTheDocument();
    expect(screen.queryByText("Umfrage anlegen")).not.toBeInTheDocument();
    expect(screen.queryByText("Elternzufriedenheit")).not.toBeInTheDocument();
  });

  it("shows no empty state while the session is still unknown", () => {
    state.sessionPending = true;
    renderPage();

    // Anker: die Seite steht, nur ihre Daten fehlen noch.
    expect(
      screen.getByRole("heading", { level: 1, name: "Umfragen" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Keine offenen Umfragen"),
    ).not.toBeInTheDocument();
  });

  it("shows the anonymity notice when a staff member opens the survey", async () => {
    renderPage();

    fireEvent.click(await screen.findByText("Teamklima"));

    const dialog = await screen.findByRole("dialog");
    expect(
      await within(dialog).findByText(
        /Die Schule sieht nicht, wer was geantwortet hat\./,
      ),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText("Ich fühle mich im Team wohl."),
    ).toBeInTheDocument();
  });

  it("gives surveys:manage the list of all surveys with counts and the create action", async () => {
    state.canManage = true;
    renderPage();

    expect(
      (await screen.findAllByText("Elternzufriedenheit")).length,
    ).toBeGreaterThan(0);
    expect(
      screen.getAllByRole("button", { name: "Umfrage anlegen" }),
    ).toHaveLength(1);
    expect(screen.getAllByText("3 Teilnahmen").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Eltern: Ganze Schule").length).toBeGreaterThan(
      0,
    );
    expect(screen.getByRole("tab", { name: /Für Sie/ })).toBeInTheDocument();
  });
});
