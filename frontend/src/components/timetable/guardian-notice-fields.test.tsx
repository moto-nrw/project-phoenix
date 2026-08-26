import { useState } from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  GuardianNoticeFields,
  guardianNoticeIncomplete,
  guardianNoticePayload,
  suggestGuardianNotice,
  type GuardianNoticeDraft,
} from "./guardian-notice-fields";
import { cancelledToast } from "./guardian-notice-toast";
import type { GuardianNoticeReach } from "~/lib/timetable-types";

const getGuardianNoticeReach = vi.fn();

vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    getGuardianNoticeReach: (...args: unknown[]) =>
      getGuardianNoticeReach(...args),
  },
}));

const block = {
  id: "42",
  title: "Fußball-AG",
  date: "2026-09-15",
  startTime: "14:00",
  endTime: "15:30",
};

const reachOn: GuardianNoticeReach = {
  enabled: true,
  defaultOn: true,
  childCount: 3,
  familyCount: 2,
};

describe("suggestGuardianNotice", () => {
  it("names the block, the day and the time window", () => {
    const suggested = suggestGuardianNotice(block);
    expect(suggested.title).toBe("Fußball-AG am 15.09.2026 entfällt");
    expect(suggested.message).toContain("15.09.2026");
    expect(suggested.message).toContain("von 14:00 bis 15:30 Uhr");
  });
});

describe("guardianNoticePayload", () => {
  const draft: GuardianNoticeDraft = {
    send: true,
    title: " Entfällt ",
    message: " Heute keine AG. ",
  };

  it("trims the text when the notice is on and allowed", () => {
    expect(guardianNoticePayload(draft, reachOn)).toEqual({
      title: "Entfällt",
      message: "Heute keine AG.",
    });
  });

  it("sends nothing when the checkbox is off, the school disallows it, or text is missing", () => {
    expect(guardianNoticePayload({ ...draft, send: false }, reachOn)).toBe(
      undefined,
    );
    expect(guardianNoticePayload(draft, { ...reachOn, enabled: false })).toBe(
      undefined,
    );
    expect(guardianNoticePayload(draft, null)).toBe(undefined);
    expect(guardianNoticePayload({ ...draft, message: "  " }, reachOn)).toBe(
      undefined,
    );
  });

  it("flags an incomplete draft only while sending is on", () => {
    expect(guardianNoticeIncomplete({ ...draft, title: "" }, reachOn)).toBe(
      true,
    );
    expect(
      guardianNoticeIncomplete({ ...draft, title: "", send: false }, reachOn),
    ).toBe(false);
    expect(guardianNoticeIncomplete(draft, reachOn)).toBe(false);
  });

  it("blocks cancellation while the notice preview is still loading", () => {
    expect(guardianNoticeIncomplete(null, null, true)).toBe(true);
    expect(guardianNoticeIncomplete(null, null)).toBe(false);
  });
});

describe("cancelledToast", () => {
  it("reports how many families were told", () => {
    expect(cancelledToast("Block abgesagt", undefined)).toBe("Block abgesagt");
    expect(
      cancelledToast("Block abgesagt", {
        announcementId: "1",
        childCount: 2,
        familyCount: 0,
      }),
    ).toBe("Block abgesagt. Keine Familie mit Elternportal-Zugang betroffen.");
    expect(
      cancelledToast("Block abgesagt", {
        announcementId: "1",
        childCount: 2,
        familyCount: 1,
      }),
    ).toBe("Block abgesagt. 1 Familie wurde informiert.");
    expect(
      cancelledToast("Block abgesagt", {
        announcementId: "1",
        childCount: 2,
        familyCount: 3,
      }),
    ).toBe("Block abgesagt. 3 Familien wurden informiert.");
  });
});

describe("GuardianNoticeFields", () => {
  beforeEach(() => {
    getGuardianNoticeReach.mockReset();
  });

  function Harness({ today = "2026-09-01" }: Readonly<{ today?: string }>) {
    const [draft, setDraft] = useState<GuardianNoticeDraft | null>(null);
    return (
      <GuardianNoticeFields
        block={block}
        today={today}
        draft={draft}
        onDraftChange={setDraft}
        onReachChange={() => undefined}
      />
    );
  }

  it("loads the reach, pre-ticks from the school default and prefills the text", async () => {
    getGuardianNoticeReach.mockResolvedValue(reachOn);
    render(<Harness />);

    expect(await screen.findByText("Eltern informieren")).toBeInTheDocument();
    expect(getGuardianNoticeReach).toHaveBeenCalledWith("42");
    expect(
      screen.getByText("Erreicht 2 Familien im Elternportal."),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Betreff")).toHaveValue(
      "Fußball-AG am 15.09.2026 entfällt",
    );
    expect(screen.getByLabelText("Text an die Eltern")).toHaveValue(
      "Die Betreuung „Fußball-AG“ am 15.09.2026 von 14:00 bis 15:30 Uhr fällt aus.",
    );
  });

  it("hides the text fields when the checkbox is unticked", async () => {
    getGuardianNoticeReach.mockResolvedValue(reachOn);
    render(<Harness />);
    const checkbox = await screen.findByLabelText("Eltern informieren");
    fireEvent.click(checkbox);
    await waitFor(() =>
      expect(screen.queryByLabelText("Betreff")).not.toBeInTheDocument(),
    );
  });

  it("renders nothing when the school switched the notice off", async () => {
    getGuardianNoticeReach.mockResolvedValue({ ...reachOn, enabled: false });
    const { container } = render(<Harness />);
    await waitFor(() => expect(getGuardianNoticeReach).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });

  it("never asks for a block in the past", () => {
    const { container } = render(<Harness today="2026-09-16" />);
    expect(getGuardianNoticeReach).not.toHaveBeenCalled();
    expect(container).toBeEmptyDOMElement();
  });

  it("explains a failed lookup without blocking the cancellation", async () => {
    getGuardianNoticeReach.mockRejectedValue(new Error("boom"));
    vi.spyOn(console, "error").mockImplementation(() => undefined);
    render(<Harness />);
    expect(
      await screen.findByText(/ließ sich gerade nicht laden/),
    ).toBeInTheDocument();
  });
});
