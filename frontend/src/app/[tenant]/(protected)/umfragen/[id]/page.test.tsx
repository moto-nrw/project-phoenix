import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import type { Survey, SurveyResults } from "~/lib/surveys-api";

import SurveyDetailPage from "./page";

const state = vi.hoisted(() => ({
  canManage: true,
  search: "",
  survey: undefined as unknown,
  results: undefined as unknown,
  respondents: undefined as unknown,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  usePathname: () => "/umfragen/21",
  useParams: () => ({ id: "21" }),
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
  useTenantRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));

vi.mock("~/lib/swr", () => ({
  useTenantMutate: () => vi.fn(),
  useSWRAuth: (key: string | null) => ({
    data:
      key === "survey-21"
        ? state.survey
        : key === "survey-results-21"
          ? state.results
          : key === "survey-respondents-21"
            ? state.respondents
            : undefined,
    error: undefined,
    isLoading: false,
    mutate: vi.fn(() => Promise.resolve(undefined)),
  }),
}));

function survey(patch: Partial<Survey> = {}): Survey {
  return {
    id: "21",
    title: "Elternzufriedenheit",
    intro: "Ihre Meinung hilft uns.",
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
    questions: [
      {
        id: "q1",
        position: 1,
        kind: "scale",
        prompt: "Mein Kind fühlt sich wohl.",
        required: true,
        options: [],
      },
    ],
    ...patch,
  };
}

function results(patch: Partial<SurveyResults> = {}): SurveyResults {
  return {
    survey_id: "21",
    title: "Elternzufriedenheit",
    anonymous: true,
    status: "open",
    response_count: 3,
    min_responses: 5,
    available: false,
    questions: [],
    strengths: [],
    improvements: [],
    ...patch,
  };
}

function renderPage() {
  return render(
    <ToastProvider>
      <SurveyDetailPage />
    </ToastProvider>,
  );
}

describe("Umfrage, Objektseite (#4781)", () => {
  beforeEach(() => {
    state.canManage = true;
    state.search = "";
    state.survey = survey();
    state.results = results();
    state.respondents = undefined;
  });

  it("explains the minimum instead of showing results below 5 answers", async () => {
    renderPage();

    // Anker: die Umfrage ist geladen, die Auswertung erklärt die Mindestzahl.
    expect(
      await screen.findByText(
        "Die Auswertung erscheint ab 5 Antworten, damit niemand erkennbar ist. Bisher: 3.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Auswertung" })).toBeInTheDocument();
    expect(screen.queryByText(/Frage 1:/)).not.toBeInTheDocument();
    expect(
      screen.queryByRole("tab", { name: "Antworten je Person" }),
    ).not.toBeInTheDocument();
  });

  it("offers answers per person only for a survey that is not anonymous", async () => {
    state.survey = survey({ anonymous: false });
    state.results = results({ anonymous: false, available: true });

    renderPage();

    expect(
      await screen.findByRole("tab", { name: "Antworten je Person" }),
    ).toBeInTheDocument();
  });

  it("lists answers with names on the per-person tab of a named survey", async () => {
    state.search = "reiter=personen";
    state.survey = survey({ anonymous: false });
    state.results = results({ anonymous: false, available: true });
    state.respondents = [
      {
        name: "Maria Muster",
        answers: [{ question_id: "q1", options: [], scale_value: 4 }],
      },
    ];

    renderPage();

    expect(await screen.findByText("Maria Muster")).toBeInTheDocument();
    expect(screen.getByText("4 von 5")).toBeInTheDocument();
  });

  it("shows no 'no longer exists' while the session is still unknown", () => {
    // useSWRAuth before the session is known: no data, no error, no request.
    state.survey = undefined;

    renderPage();

    // Anker: der Kopf der Seite steht im Ladezustand.
    expect(
      screen.getByRole("heading", { level: 1, name: "Umfrage" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Diese Umfrage gibt es nicht mehr."),
    ).not.toBeInTheDocument();
  });

  it("refuses the page without surveys:manage", async () => {
    state.canManage = false;

    renderPage();

    // Anker: der Hinweis auf das fehlende Recht.
    expect(
      await screen.findByText(/Umfragen anlegen und auswerten darf nur/),
    ).toBeInTheDocument();
    expect(screen.queryByText("Elternzufriedenheit")).not.toBeInTheDocument();
  });
});
