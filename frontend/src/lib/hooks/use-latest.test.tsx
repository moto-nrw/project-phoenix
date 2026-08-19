import { render } from "@testing-library/react";
import { useLayoutEffect } from "react";
import { describe, expect, it } from "vitest";

import { useLatest } from "./use-latest";

describe("useLatest", () => {
  it("exposes the new value during the same commit", () => {
    const committedValues: string[] = [];

    function Probe({ value }: Readonly<{ value: string }>) {
      const latest = useLatest(value);

      useLayoutEffect(() => {
        committedValues.push(latest.current);
      }, [latest, value]);

      return null;
    }

    const { rerender } = render(<Probe value="first" />);
    rerender(<Probe value="second" />);

    expect(committedValues).toEqual(["first", "second"]);
  });
});
