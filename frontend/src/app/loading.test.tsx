import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import RootLoadingPage from "./loading";

describe("RootLoadingPage", () => {
  it("provides neutral loading feedback for routes without local boundaries", () => {
    render(<RootLoadingPage />);

    expect(screen.getByRole("status", { name: "Laden..." })).toBeVisible();
  });
});
