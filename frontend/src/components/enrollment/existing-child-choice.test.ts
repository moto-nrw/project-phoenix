import { describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";
import {
  existingChildChoiceOptions,
  possiblyEnrolledCandidates,
  renewedChildNameOf,
  sendWithExistingChildChoice,
} from "./existing-child-choice";

const candidate = {
  student_id: "42",
  first_name: "Mara",
  last_name: "Papier",
  date_of_birth: "2019-03-01",
  school_class: "1a",
};

function possiblyEnrolled(candidates: unknown = [candidate]) {
  return new ApiError("child may already be enrolled", 409, {
    code: "enrollment.child_possibly_enrolled",
    details: { candidates },
  });
}

describe("possiblyEnrolledCandidates (#6148)", () => {
  it("reads the candidates of the refusal", () => {
    expect(possiblyEnrolledCandidates(possiblyEnrolled())).toEqual([candidate]);
  });

  it("ignores other errors and a refusal without usable candidates", () => {
    expect(
      possiblyEnrolledCandidates(
        new ApiError("full", 409, { code: "students.child_quota_reached" }),
      ),
    ).toBeNull();
    expect(possiblyEnrolledCandidates(possiblyEnrolled([]))).toBeNull();
    expect(
      possiblyEnrolledCandidates(possiblyEnrolled([{ first_name: "Mara" }])),
    ).toBeNull();
    expect(possiblyEnrolledCandidates(new Error("offline"))).toBeNull();
  });
});

describe("sendWithExistingChildChoice (#6148)", () => {
  it("sends once without a choice when no child matches", async () => {
    const send = vi.fn().mockResolvedValue("created");
    const ask = vi.fn();

    await expect(sendWithExistingChildChoice(send, ask)).resolves.toBe(
      "created",
    );
    expect(send).toHaveBeenCalledOnce();
    expect(send).toHaveBeenCalledWith(undefined);
    expect(ask).not.toHaveBeenCalled();
  });

  it("asks the office and sends again with the chosen child", async () => {
    const send = vi
      .fn()
      .mockRejectedValueOnce(possiblyEnrolled())
      .mockResolvedValueOnce("renewed");
    const ask = vi.fn().mockResolvedValue({ existingStudentId: "42" });

    await expect(sendWithExistingChildChoice(send, ask)).resolves.toBe(
      "renewed",
    );
    expect(ask).toHaveBeenCalledWith([candidate]);
    expect(send).toHaveBeenLastCalledWith({ existingStudentId: "42" });
  });

  it("sends again as a new child when the office says so", async () => {
    const send = vi
      .fn()
      .mockRejectedValueOnce(possiblyEnrolled())
      .mockResolvedValueOnce("created");
    const ask = vi.fn().mockResolvedValue({ createNewStudent: true });

    await sendWithExistingChildChoice(send, ask);
    expect(send).toHaveBeenLastCalledWith({ createNewStudent: true });
  });

  it("stops without saving when the office cancels", async () => {
    const send = vi.fn().mockRejectedValueOnce(possiblyEnrolled());
    const ask = vi.fn().mockResolvedValue(null);

    await expect(sendWithExistingChildChoice(send, ask)).resolves.toBeNull();
    expect(send).toHaveBeenCalledOnce();
  });

  it("passes every other failure on", async () => {
    const failure = new ApiError("full", 409, {
      code: "students.child_quota_reached",
    });
    const send = vi.fn().mockRejectedValue(failure);
    const ask = vi.fn();

    await expect(sendWithExistingChildChoice(send, ask)).rejects.toBe(failure);
    expect(ask).not.toHaveBeenCalled();
  });
});

describe("existingChildChoiceOptions for the manual enrollment", () => {
  it("offers the candidate and a new child, like the detail view", () => {
    expect(
      existingChildChoiceOptions({
        first_name: "Mara",
        last_name: "Papier",
        possible_existing_children: [candidate],
      }),
    ).toEqual([
      {
        value: "42",
        label: "Bestehendes Kind verwenden",
        description: "Mara Papier · Klasse 1a · geboren am 01.03.2019",
      },
      {
        value: "new",
        label: "Als neues Kind anlegen",
        description: "Wählen Sie dies nur, wenn es ein anderes Kind ist.",
      },
    ]);
  });
});

describe("renewedChildNameOf (#6148)", () => {
  it("names the chosen enrolled child as the school knows it", () => {
    expect(
      renewedChildNameOf({ existingStudentId: "42" }, [
        { ...candidate, first_name: "Mara-Sophie" },
      ]),
    ).toBe("Mara-Sophie Papier");
  });

  it("names no child for a new child or no choice", () => {
    expect(renewedChildNameOf({ createNewStudent: true }, [candidate])).toBe(
      null,
    );
    expect(renewedChildNameOf(undefined, [candidate])).toBe(null);
  });
});
