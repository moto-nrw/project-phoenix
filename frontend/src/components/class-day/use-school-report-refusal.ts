"use client";

import { useLayoutEffect } from "react";
import { useDiscardRefusedEntity } from "~/lib/swr/hooks";

/** Access refusals remove a roster before paint; transient failures keep it. */
export function useSchoolReportRefusal(error: unknown, reportKey: string) {
  const discardRefused = useDiscardRefusedEntity();
  useLayoutEffect(() => {
    discardRefused(error, {
      codes: ["general.permission"],
      matches: (key) => key === reportKey,
    });
  }, [discardRefused, error, reportKey]);
}
