import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ParentAnnouncement } from "~/lib/parent-api";
import * as parentApi from "~/lib/parent-api";
import type { SurveyForm } from "~/lib/survey-form";
import {
  NewsCard,
  NewsDetailModal,
  isOutstandingAnnouncement,
} from "./news-components";

// Umfrage mit Fragebogen im Elternportal (#4781): die Karte zeigt, ob die
// Umfrage anonym ist und ob dieses Elternkonto teilgenommen hat; der Dialog
// sagt das vor der ersten Frage und schickt die Antworten einmal ab.

function surveyItem(
  survey: Partial<NonNullable<ParentAnnouncement["survey"]>> = {},
): ParentAnnouncement {
  return {
    id: "42",
    title: "Wie zufrieden sind Sie mit der OGS?",
    body: "Ihre Meinung hilft uns.",
    priority: "info",
    requires_acknowledgement: false,
    school_name: "OGS Am Berg",
    published_at: "2026-09-01T08:00:00Z",
    read: true,
    acknowledged: false,
    response_type: "none",
    system_kind: "survey",
    survey: {
      survey_id: "7",
      anonymous: true,
      closes_at: "2026-09-30T16:00:00Z",
      participated: false,
      ...survey,
    },
  };
}

const form: SurveyForm = {
  school_name: "OGS Am Berg",
  participated: false,
  survey: {
    id: "7",
    title: "Wie zufrieden sind Sie mit der OGS?",
    intro: "Ihre Meinung hilft uns.",
    anonymous: true,
    closes_at: "2026-09-30T16:00:00Z",
    questions: [
      {
        id: "q1",
        position: 1,
        kind: "single",
        prompt: "Wie kommt Ihr Kind nach Hause?",
        required: true,
        options: [
          { id: "o1", label: "Zu Fuß" },
          { id: "o2", label: "Mit dem Bus" },
        ],
      },
    ],
  },
};

beforeEach(() => {
  vi.restoreAllMocks();
  vi.spyOn(parentApi, "listAnnouncementAttachments").mockResolvedValue([]);
});

describe("Umfrage im Elternportal", () => {
  it("flags an open survey with its anonymity and deadline on the card", () => {
    render(<NewsCard item={surveyItem()} onOpen={vi.fn()} />);

    expect(screen.getByText("Umfrage")).toBeInTheDocument();
    expect(screen.getByText("Antwort nötig")).toBeInTheDocument();
    expect(screen.getByText("Anonym")).toBeInTheDocument();
    expect(
      screen.getByText("Teilnahme bis 30.09.2026, 18:00 Uhr"),
    ).toBeInTheDocument();
    expect(isOutstandingAnnouncement(surveyItem())).toBe(true);
  });

  it("marks a survey this account answered as done", () => {
    const item = surveyItem({ participated: true, anonymous: false });
    render(<NewsCard item={item} onOpen={vi.fn()} />);

    expect(screen.getByText("Teilgenommen")).toBeInTheDocument();
    expect(screen.getByText("Nicht anonym")).toBeInTheDocument();
    expect(isOutstandingAnnouncement(item)).toBe(false);
  });

  it("states the anonymity before the questions and sends the answers once", async () => {
    vi.spyOn(parentApi, "fetchParentSurveyForm").mockResolvedValue(form);
    const submit = vi
      .spyOn(parentApi, "submitParentSurveyAnswers")
      .mockResolvedValue(undefined);
    const onUpdated = vi.fn();

    render(
      <NewsDetailModal
        item={surveyItem()}
        onClose={vi.fn()}
        onUpdated={onUpdated}
      />,
    );

    expect(
      await screen.findByText(
        /Die Schule sieht nicht, wer was geantwortet hat\..*Sie antworten einmal für Ihr Elternkonto, nicht für jedes Kind\./,
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("radio", { name: "Mit dem Bus" }));
    fireEvent.click(screen.getByRole("button", { name: "Antwort absenden" }));

    expect(
      await screen.findByText("Danke, Ihre Antwort ist gespeichert."),
    ).toBeInTheDocument();
    expect(submit).toHaveBeenCalledWith("42", {
      answers: [{ question_id: "q1", option_ids: ["o2"] }],
    });
    expect(onUpdated).toHaveBeenCalledWith(
      "42",
      expect.objectContaining({
        survey: expect.objectContaining({ participated: true }),
      }),
    );
  });
});
