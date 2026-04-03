"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { createLogger } from "~/lib/logger";
import {
  validateOperatorInvitation,
  type OperatorInvitationValidation,
} from "~/lib/operator/invitation-api";
import { OperatorInvitationAcceptForm } from "./operator-invitation-accept-form";

const logger = createLogger({ component: "OperatorInvitePageContent" });

type PageState =
  | { type: "loading" }
  | { type: "error"; message: string }
  | { type: "ready"; invitation: OperatorInvitationValidation; token: string };

/**
 * Extracts the token from the URL fragment (#token=...).
 * Using a fragment instead of a query parameter prevents the token from
 * leaking in Referer headers, server access logs, or CDN logs.
 */
function extractTokenFromHash(): string | null {
  if (typeof window === "undefined") return null;
  const hash = window.location.hash;
  if (!hash.startsWith("#token=")) return null;
  const token = hash.slice("#token=".length);
  return token || null;
}

export function OperatorInvitePageContent() {
  const [state, setState] = useState<PageState>({ type: "loading" });

  useEffect(() => {
    const token = extractTokenFromHash();

    // Strip token from URL immediately to prevent leakage via
    // browser history or shoulder-surfing.
    if (token) {
      window.history.replaceState({}, "", window.location.pathname);
    }

    if (!token) {
      setState({
        type: "error",
        message: "Kein Einladungstoken angegeben.",
      });
      return;
    }

    validateOperatorInvitation(token)
      .then((invitation) => {
        setState({ type: "ready", invitation, token });
      })
      .catch((error) => {
        logger.warn("invitation_validation_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        setState({
          type: "error",
          message:
            error instanceof Error
              ? error.message
              : "Diese Einladung ist ungültig oder abgelaufen.",
        });
      });
  }, []);

  if (state.type === "loading") {
    return (
      <div className="text-center">
        <div className="mx-auto h-8 w-8 animate-spin rounded-full border-4 border-purple-600 border-t-transparent" />
        <p className="mt-4 text-sm text-gray-500">Einladung wird geprüft...</p>
      </div>
    );
  }

  if (state.type === "error") {
    return (
      <div className="w-full max-w-md rounded-xl bg-white p-8 text-center shadow-lg">
        <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-red-100">
          <svg
            className="h-6 w-6 text-red-600"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </div>
        <h2 className="mb-2 text-lg font-semibold text-gray-900">
          Einladung ungültig
        </h2>
        <p className="text-sm text-gray-600">{state.message}</p>
        <Link
          href="/operator/login"
          className="mt-6 inline-block rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800"
        >
          Zum Login
        </Link>
      </div>
    );
  }

  return (
    <OperatorInvitationAcceptForm
      token={state.token}
      invitation={state.invitation}
    />
  );
}
