import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { SurveyResults } from "~/lib/surveys-api";
import { SurveyResultsView } from "./survey-results";

function results(patch: Partial<SurveyResults> = {}): SurveyResults {
  return {
    survey_id: "7",
    title: "Zufriedenheit",
    anonymous: true,
    // Eine anonyme Umfrage wird erst nach dem Ende ausgewertet.
    status: "closed",
    response_count: 6,
    min_responses: 5,
    available: true,
    questions: [
      {
        question_id: "q1",
        position: 1,
        kind: "scale",
        prompt: "Mein Kind fühlt sich wohl.",
        answer_count: 6,
        options: [],
        scale_counts: [0, 1, 1, 2, 2],
        average: 3.8333,
        texts: [],
        highest: ["4", "5"],
        lowest: ["1"],
        withheld: false,
      },
      {
        question_id: "q2",
        position: 2,
        kind: "single",
        prompt: "Wie kommt Ihr Kind?",
        answer_count: 6,
        options: [
          { option_id: "o1", label: "Zu Fuß", count: 4 },
          { option_id: "o2", label: "Mit dem Bus", count: 2 },
        ],
        scale_counts: [],
        average: null,
        texts: [],
        highest: ["Zu Fuß"],
        lowest: ["Mit dem Bus"],
        withheld: false,
      },
      {
        question_id: "q3",
        position: 3,
        kind: "text",
        prompt: "Was sollen wir beibehalten?",
        answer_count: 5,
        options: [],
        scale_counts: [],
        average: null,
        texts: [
          "Die AGs",
          "Das Mittagessen",
          "Den Garten",
          "Die Ruhe",
          "Die Ausflüge",
        ],
        highest: [],
        lowest: [],
        withheld: false,
      },
    ],
    strengths: [
      {
        question_id: "q1",
        prompt: "Mein Kind fühlt sich wohl.",
        average: 3.8333,
      },
    ],
    improvements: [],
    ...patch,
  };
}

describe("SurveyResultsView", () => {
  it("shows sums, the scale average, free texts and strengths", () => {
    render(<SurveyResultsView results={results()} />);

    expect(
      screen.getByText("Frage 1: Mein Kind fühlt sich wohl."),
    ).toBeInTheDocument();
    expect(screen.getByText("3,8 von 5")).toBeInTheDocument();
    expect(screen.getByText("4 · 67 %")).toBeInTheDocument();
    expect(screen.getByText("Die AGs")).toBeInTheDocument();
    expect(
      screen.getByText("Stärken und Verbesserungsbedarf"),
    ).toBeInTheDocument();
    expect(screen.getByText("Wert 4, Wert 5")).toBeInTheDocument();
    expect(
      screen.getByText("Mit dem Bus", { selector: "dd" }),
    ).toBeInTheDocument();
  });

  it("shows no question data while an anonymous survey has too few answers", () => {
    const { container } = render(
      <SurveyResultsView
        results={results({
          available: false,
          response_count: 3,
          // Selbst wenn das Backend Daten mitschickte: nichts davon erscheint.
          strengths: [],
        })}
      />,
    );

    // Anker: die Erklärung mit der Mindestzahl und dem bisherigen Stand.
    expect(
      screen.getByText(
        "Die Auswertung erscheint ab 5 Antworten, damit niemand erkennbar ist. Bisher: 3.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Mein Kind fühlt sich wohl/),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Die AGs")).not.toBeInTheDocument();
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    expect(container.textContent).not.toContain("Zu Fuß");
  });

  it("waits for the end of a running anonymous survey", () => {
    render(
      <SurveyResultsView
        results={results({ status: "open", available: false })}
      />,
    );

    expect(
      screen.getByText(
        "Die Auswertung erscheint nach dem Ende der Umfrage, wenn mindestens 5 geantwortet haben. So ist niemand an seinen Antworten erkennbar. Bisher: 6.",
      ),
    ).toBeInTheDocument();
  });

  it("shows only the count of a question with too few answers", () => {
    const base = results();
    const { container } = render(
      <SurveyResultsView
        results={results({
          questions: [
            {
              ...base.questions[0]!,
              answer_count: 2,
              scale_counts: [],
              average: null,
              highest: [],
              lowest: [],
              withheld: true,
            },
          ],
          strengths: [],
        })}
      />,
    );

    // Anker: die Frage mit ihrer Zahl und dem Hinweis.
    expect(
      screen.getByText("Frage 1: Mein Kind fühlt sich wohl."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Zu wenige Antworten. Das Ergebnis dieser Frage erscheint erst ab 5 Antworten, damit niemand erkennbar ist.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Durchschnitt/)).not.toBeInTheDocument();
    expect(container.querySelectorAll("li")).toHaveLength(0);
  });
});
