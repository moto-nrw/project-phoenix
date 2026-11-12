"use client";

import { useEffect, useState } from "react";
import { createLogger } from "~/lib/logger";
import { prepareQrLogo, type QrLogo } from "~/lib/qr-code/qr-image";

const logger = createLogger({ component: "QrLogo" });

/** A picture for the middle of the code: an address in moto or a chosen file. */
export type QrLogoSource = string | Blob;

type QrLogoState =
  | { readonly status: "loading"; readonly logo: null }
  | { readonly status: "ready"; readonly logo: QrLogo }
  | { readonly status: "failed"; readonly logo: null };

function loadImage(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error("logo_not_available"));
    image.src = src;
  });
}

async function prepare(
  source: QrLogoSource,
  removeBackground: boolean,
): Promise<QrLogo> {
  if (typeof source !== "string") {
    return prepareQrLogo(source, { removeBackground });
  }
  // Addresses are same-origin pictures of moto; an <img> reads them with the
  // session and without a second request path.
  const image = await loadImage(source);
  return prepareQrLogo(image, { removeBackground });
}

/**
 * Prepares the logo for the code. `failed` covers a picture that does not
 * exist (no logo stored) and a file the browser cannot read.
 */
export function useQrLogo(
  source: QrLogoSource | null,
  removeBackground: boolean,
): QrLogoState {
  const [state, setState] = useState<{
    readonly source: QrLogoSource | null;
    readonly removeBackground: boolean;
    readonly result: QrLogoState;
  } | null>(null);

  useEffect(() => {
    if (source === null) return;
    let cancelled = false;
    prepare(source, removeBackground)
      .then((logo) => {
        if (cancelled) return;
        setState({
          source,
          removeBackground,
          result: { status: "ready", logo },
        });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        // Expected for a Träger without a stored logo: the dialog falls back.
        logger.info("qr_logo_unavailable", {
          error: err instanceof Error ? err.message : String(err),
        });
        setState({
          source,
          removeBackground,
          result: { status: "failed", logo: null },
        });
      });
    return () => {
      cancelled = true;
    };
  }, [source, removeBackground]);

  if (state?.source !== source || state.removeBackground !== removeBackground) {
    return { status: "loading", logo: null };
  }
  return state.result;
}
