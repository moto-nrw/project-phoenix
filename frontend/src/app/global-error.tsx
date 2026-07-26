"use client";

import * as Sentry from "@sentry/nextjs";
import { useEffect, type CSSProperties } from "react";

const PAGE_STYLE: CSSProperties = {
  display: "flex",
  flexDirection: "column",
  alignItems: "center",
  justifyContent: "center",
  minHeight: "100vh",
  fontFamily: "system-ui, sans-serif",
  padding: "2rem",
  textAlign: "center",
};
const TITLE_STYLE: CSSProperties = {
  fontSize: "1.5rem",
  marginBottom: "1rem",
};
const MESSAGE_STYLE: CSSProperties = {
  color: "#666",
  marginBottom: "1.5rem",
};
const RETRY_BUTTON_STYLE: CSSProperties = {
  padding: "0.75rem 1.5rem",
  backgroundColor: "#2563eb",
  color: "white",
  border: "none",
  borderRadius: "0.5rem",
  cursor: "pointer",
  fontSize: "1rem",
};

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    Sentry.captureException(error);
  }, [error]);

  return (
    <html lang="de">
      <body>
        <div style={PAGE_STYLE}>
          <h2 style={TITLE_STYLE}>Ein unerwarteter Fehler ist aufgetreten</h2>
          <p style={MESSAGE_STYLE}>
            Bitte versuchen Sie es erneut. Falls das Problem bestehen bleibt,
            kontaktieren Sie den Support.
          </p>
          <button
            type="button"
            onClick={() => reset()}
            style={RETRY_BUTTON_STYLE}
          >
            Erneut versuchen
          </button>
        </div>
      </body>
    </html>
  );
}
