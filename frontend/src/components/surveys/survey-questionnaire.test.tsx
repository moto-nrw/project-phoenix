import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { SurveyForm } from "~/lib/survey-form";
import { SurveyAnswerPanel } from "./survey-questionnaire";

function form(
  patch: Partial<SurveyForm["survey"]> = {},
  participated = false,
): SurveyForm {
  return {
    school_name: "OGS am Park",
    participated,
    survey: {
      id: "7",
      title: "Zufriedenheit",
      intro: "Ihre Meinung hilft uns.",
      anonymous: true,
      // Nach der Testuhr (09.09.2026), also noch offen.
      closes_at: "2026-09-30T16:00:00Z",
      questions: [
        {
          id: "q1",
          position: 1,
          kind: "scale",
          prompt: "Mein Kind fühlt sich wohl.",
          required: true,
          options: [],
        },
        {
          id: "q2",
          position: 2,
          kind: "text",
          prompt: "Was sollen wir beibehalten?",
          required: false,
          options: [],
        },
      ],
      ...patch,
    },
  };
}

describe("SurveyAnswerPanel", () => {
  it("states before the first question that the survey is anonymous", async () => {
    render(
      <SurveyAnswerPanel
        load={() => Promise.resolve(form())}
        submit={vi.fn()}
        respondent="parent"
      />,
    );

    const notice = await screen.findByText("Anonym");
    const question = screen.getByText("Mein Kind fühlt sich wohl.");
    expect(
      screen.getByText(
        /Die Schule sieht nicht, wer was geantwortet hat\. Sie sieht erst ab 5 Antworten eine Auswertung\. Sie antworten einmal für Ihr Elternkonto/,
      ),
    ).toBeInTheDocument();
    // Der Hinweis steht im Dokument vor der ersten Frage.
    expect(
      notice.compareDocumentPosition(question) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      screen.getByText("Teilnahme bis 30.09.2026, 18:00 Uhr"),
    ).toBeInTheDocument();
  });

  it("says plainly when the school sees the name", async () => {
    render(
      <SurveyAnswerPanel
        load={() => Promise.resolve(form({ anonymous: false }))}
        submit={vi.fn()}
        respondent="staff"
      />,
    );

    expect(await screen.findByText("Nicht anonym")).toBeInTheDocument();
    expect(
      screen.getByText("Die Schule sieht Ihren Namen bei Ihren Antworten."),
    ).toBeInTheDocument();
  });

  it("blocks sending until the required question is answered", async () => {
    const submit = vi.fn();
    render(
      <SurveyAnswerPanel
        load={() => Promise.resolve(form())}
        submit={submit}
        respondent="staff"
      />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "Antwort absenden" }),
    );

    expect(
      await screen.findByText(
        "Bitte beantworten Sie zuerst alle Pflichtfragen.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bitte beantworten Sie diese Frage."),
    ).toBeInTheDocument();
    expect(submit).not.toHaveBeenCalled();
  });

  it("sends the answers once and then thanks without a way to change them", async () => {
    const submit = vi.fn().mockResolvedValue(undefined);
    const onSubmitted = vi.fn();
    render(
      <SurveyAnswerPanel
        load={() => Promise.resolve(form())}
        submit={submit}
        respondent="staff"
        onSubmitted={onSubmitted}
      />,
    );

    fireEvent.click(await screen.findByRole("radio", { name: "4" }));
    fireEvent.change(
      screen.getByRole("textbox", { name: "Was sollen wir beibehalten?" }),
      {
        target: { value: " Die AGs " },
      },
    );
    fireEvent.click(screen.getByRole("button", { name: "Antwort absenden" }));

    expect(
      await screen.findByText("Danke, Ihre Antwort ist gespeichert."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Sie können sie nicht mehr ändern."),
    ).toBeInTheDocument();
    expect(submit).toHaveBeenCalledWith({
      answers: [
        { question_id: "q1", scale_value: 4 },
        { question_id: "q2", text: "Die AGs" },
      ],
    });
    expect(onSubmitted).toHaveBeenCalledTimes(1);
    expect(
      screen.queryByRole("button", { name: "Antwort absenden" }),
    ).not.toBeInTheDocument();
  });

  it("toggles a multiple-choice option once per click on its text", async () => {
    const user = userEvent.setup();
    const submit = vi.fn().mockResolvedValue(undefined);
    render(
      <SurveyAnswerPanel
        load={() =>
          Promise.resolve(
            form({
              questions: [
                {
                  id: "q3",
                  position: 1,
                  kind: "multiple",
                  prompt: "Was wünschen Sie sich?",
                  required: true,
                  options: [
                    { id: "o1", label: "Mehr Material" },
                    { id: "o2", label: "Kürzere Besprechungen" },
                    { id: "o3", label: "Feste Rückzugsräume" },
                  ],
                },
              ],
            }),
          )
        }
        submit={submit}
        respondent="staff"
      />,
    );

    await user.click(await screen.findByText("Mehr Material"));
    await user.click(screen.getByText("Kürzere Besprechungen"));
    await user.click(screen.getByText("Feste Rückzugsräume"));
    await user.click(screen.getByText("Feste Rückzugsräume"));

    expect(
      screen.getByRole("checkbox", { name: "Mehr Material" }),
    ).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Kürzere Besprechungen" }),
    ).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Feste Rückzugsräume" }),
    ).not.toBeChecked();

    await user.click(screen.getByRole("button", { name: "Antwort absenden" }));
    await waitFor(() =>
      expect(submit).toHaveBeenCalledWith({
        answers: [{ question_id: "q3", option_ids: ["o1", "o2"] }],
      }),
    );
  });

  it("shows the done state instead of the questions after taking part", async () => {
    render(
      <SurveyAnswerPanel
        load={() => Promise.resolve(form({}, true))}
        submit={vi.fn()}
        respondent="parent"
      />,
    );

    // Anker: der geladene Fragebogen mit dem Teilgenommen-Zustand.
    expect(
      await screen.findByText("Sie haben schon teilgenommen."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Mein Kind fühlt sich wohl."),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Antwort absenden" }),
    ).not.toBeInTheDocument();
  });

  it("shows a load failure with a retry instead of an empty form", async () => {
    const load = vi
      .fn()
      .mockRejectedValueOnce(
        new ApiError("Umfrage konnte nicht geladen werden", 503),
      )
      .mockResolvedValueOnce(form());
    render(
      <SurveyAnswerPanel load={load} submit={vi.fn()} respondent="staff" />,
    );

    fireEvent.click(
      await screen.findByRole("button", {
        name: /erneut versuchen|wiederholen/i,
      }),
    );

    await waitFor(() => expect(load).toHaveBeenCalledTimes(2));
    expect(
      await screen.findByText("Mein Kind fühlt sich wohl."),
    ).toBeInTheDocument();
  });
});
