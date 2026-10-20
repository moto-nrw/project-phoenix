import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { StudentHistoryLoadingBack } from "./student-history-loading-back";

const mockParams: { value: { id?: string } | null } = { value: { id: "7" } };

vi.mock("next/navigation", () => ({
  useParams: () => mockParams.value,
  usePathname: () => "/students/7/change-history",
  useRouter: () => ({ push: vi.fn() }),
}));

describe("StudentHistoryLoadingBack", () => {
  beforeEach(() => {
    mockParams.value = { id: "7" };
  });

  it("führt schon beim Laden zurück zum Kinderprofil", () => {
    render(<StudentHistoryLoadingBack />);

    expect(
      screen.getByRole("link", { name: "Zurück zum Kinderprofil" }),
    ).toHaveAttribute(
      "href",
      expect.stringMatching(
        /\/students\/7\?from=%2Fstudents%2Fsearch&tab=historie$/,
      ),
    );
  });

  it("führt ohne Kind-ID zur Kinderübersicht", () => {
    mockParams.value = null;
    render(<StudentHistoryLoadingBack />);

    expect(
      screen.getByRole("link", { name: "Zurück zur Kinderübersicht" }),
    ).toHaveAttribute("href", expect.stringMatching(/\/students\/search$/));
  });
});
