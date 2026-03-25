import { useEffect, useRef } from "react";

export function useScrollToError(error: string | null | undefined) {
  const errorRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (error && errorRef.current) {
      errorRef.current.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  }, [error]);

  return errorRef;
}
