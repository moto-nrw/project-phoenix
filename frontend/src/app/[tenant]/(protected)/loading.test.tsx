import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import ProtectedLoadingPage from "./loading";

describe("ProtectedLoadingPage", () => {
  it("renders neutral loading feedback inside the app shell", () => {
    render(<ProtectedLoadingPage />);

    const loading = screen.getByRole("status", { name: "Lädt..." });
    expect(loading).not.toHaveClass("fixed");
    expect(loading).toHaveClass("min-h-40");
  });
});
