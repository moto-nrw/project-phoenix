import { describe, expect, it } from "vitest";
import {
  answerDiffersFromSeries,
  answerSavedMessage,
  applyAnswer,
  defaultAnswerScope,
  seriesAnswerReplacesDates,
} from "./calendar-answers";
import type { CalendarEvent } from "./personal-calendar-api";

function cafe(overrides: Partial<CalendarEvent> = {}): CalendarEvent {
  return {
    id: "appointment:7:2026-11-18",
    source: "appointment",
    appointment_id: "7",
    occurrence_date: "2026-11-18",
    title: "Elterncafé",
    start_date: "2026-11-18",
    end_date: "2026-11-18",
    start_time: "15:00",
    end_time: "16:00",
    all_day: false,
    recurring: true,
    delivery_mode: "rsvp_required",
    response_status: "pending",
    series_response_status: "pending",
    recipient_id: "81",
    can_respond: true,
    can_edit: false,
    ...overrides,
  };
}

describe("defaultAnswerScope", () => {
  it("answers the whole series while it has no answer yet", () => {
    expect(defaultAnswerScope(cafe())).toBe("series");
  });

  it("answers only this date once the series has an answer", () => {
    expect(
      defaultAnswerScope(
        cafe({
          response_status: "accepted",
          series_response_status: "accepted",
        }),
      ),
    ).toBe("occurrence");
  });

  it("answers the appointment itself when it does not recur", () => {
    expect(
      defaultAnswerScope(
        cafe({
          recurring: false,
          response_status: "accepted",
          series_response_status: "accepted",
        }),
      ),
    ).toBe("series");
  });
});

describe("answerDiffersFromSeries", () => {
  it("marks a date answered apart from its series", () => {
    expect(
      answerDiffersFromSeries(
        cafe({
          response_status: "declined",
          series_response_status: "accepted",
        }),
      ),
    ).toBe(true);
    expect(
      answerDiffersFromSeries(
        cafe({
          response_status: "accepted",
          series_response_status: "accepted",
        }),
      ),
    ).toBe(false);
    expect(
      answerDiffersFromSeries(
        cafe({
          recurring: false,
          response_status: "declined",
          series_response_status: "accepted",
        }),
      ),
    ).toBe(false);
  });
});

describe("applyAnswer", () => {
  const first = cafe({
    response_status: "accepted",
    series_response_status: "accepted",
  });
  const second = cafe({
    id: "appointment:7:2026-11-25",
    occurrence_date: "2026-11-25",
    start_date: "2026-11-25",
    end_date: "2026-11-25",
    response_status: "declined",
    series_response_status: "accepted",
  });
  const other = cafe({
    id: "appointment:8:2026-11-18",
    appointment_id: "8",
    recipient_id: "99",
    response_status: "pending",
    series_response_status: "pending",
  });

  it("changes only the answered date", () => {
    const [a, b, c] = applyAnswer([first, second, other], {
      recipientId: "81",
      occurrenceDate: "2026-11-18",
      status: "declined",
    });
    expect(a?.response_status).toBe("declined");
    expect(a?.series_response_status).toBe("accepted");
    expect(b?.response_status).toBe("declined");
    expect(b?.has_occurrence_answers).toBe(true);
    expect(c).toBe(other);
  });

  it("sets every date of the series and drops the answers for single dates", () => {
    const [a, b, c] = applyAnswer([first, second, other], {
      recipientId: "81",
      status: "accepted",
    });
    expect(a?.response_status).toBe("accepted");
    expect(b?.response_status).toBe("accepted");
    expect(b?.series_response_status).toBe("accepted");
    expect(c).toBe(other);
  });
});

describe("answerSavedMessage", () => {
  it("names the day of an answer for one date", () => {
    expect(answerSavedMessage("declined", "2026-11-25")).toBe(
      "Ihre Absage für Mittwoch, 25.11. ist gespeichert.",
    );
    expect(answerSavedMessage("accepted")).toBe("Ihre Zusage ist gespeichert.");
  });
});

// #6987 review: a series without an answer, but with an answer for one
// date, must not offer to overwrite that date by default.
describe("dates answered on their own", () => {
  const open = cafe({ has_occurrence_answers: true });

  it("answers only this date by default", () => {
    expect(defaultAnswerScope(open)).toBe("occurrence");
    expect(seriesAnswerReplacesDates(open)).toBe(true);
    expect(seriesAnswerReplacesDates(cafe())).toBe(false);
  });

  it("tracks whether a series answer would replace answers for dates", () => {
    const [afterDate] = applyAnswer([cafe()], {
      recipientId: "81",
      occurrenceDate: "2026-11-25",
      status: "declined",
    });
    expect(afterDate?.has_occurrence_answers).toBe(true);
    expect(afterDate?.response_status).toBe("pending");
    const [afterSeries] = applyAnswer([open], {
      recipientId: "81",
      status: "accepted",
    });
    expect(afterSeries?.has_occurrence_answers).toBe(false);
  });
});
