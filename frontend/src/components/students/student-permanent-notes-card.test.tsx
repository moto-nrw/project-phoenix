import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { StudentPermanentNotesCard } from "./student-permanent-notes-card";

const mutate = vi.fn(async () => undefined);
const swrResult: { data: unknown; isLoading: boolean; error: unknown } = {
  data: undefined,
  isLoading: false,
  error: undefined,
};

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => ({ ...swrResult, mutate }),
}));

describe("StudentPermanentNotesCard", () => {
  it("shows a failed load in place with a retry instead of an empty card", async () => {
    swrResult.error = new ApiError("down", 503, {
      code: "general.unavailable",
      instance: "req-hints",
    });
    render(<StudentPermanentNotesCard studentId="7" />);

    expect(
      await screen.findByText(
        "Die Liste der Hinweise ist gerade nicht erreichbar. Bitte versuchen Sie es erneut.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Keine dauerhaften Hinweise"),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(mutate).toHaveBeenCalledOnce();
  });
});
