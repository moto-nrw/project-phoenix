"use client";

import { useCallback, useLayoutEffect } from "react";
import { useDiscardRefusedEntity } from "~/lib/swr/hooks";

/** Access refusals remove a roster before paint; transient failures keep it. */
export function useSchoolReportRefusal(error: unknown, reportKey: string) {
  const discardRefused = useDiscardRefusedEntity();
  const discardReport = useCallback(
    (failure: unknown) =>
      discardRefused(failure, {
        codes: ["general.permission"],
        matches: (key) => key === reportKey,
        refusedKey: reportKey,
      }),
    [discardRefused, reportKey],
  );
  useLayoutEffect(() => {
    discardReport(error);
  }, [discardReport, error]);
  return discardReport;
}
