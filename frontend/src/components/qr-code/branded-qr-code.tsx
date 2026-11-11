"use client";

import { useMemo } from "react";
import { Skeleton } from "~/components/ui/skeleton";
import { brandedQrLayout, brandedQrSvg } from "~/lib/qr-code/branded-qr";
import type { QrLogo } from "~/lib/qr-code/qr-image";
import type { QrBrandLogo } from "./qr-code-dialog";
import { useQrLogo } from "./use-qr-logo";

export const MOTO_QR_LOGO = "/images/moto-logo-mit-schriftzug.png";

/** The code as a picture that fills its box. */
export function QrCodeImage({
  url,
  logo,
}: Readonly<{ url: string; logo: QrLogo | null }>) {
  const src = useMemo(() => {
    const svg = brandedQrSvg(brandedQrLayout(url, logo?.aspect), {
      label: "QR-Code",
      logoDataUrl: logo?.dataUrl,
    });
    return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
  }, [url, logo]);

  return (
    // oxlint-disable-next-line nextjs/no-img-element -- a data URL built in the browser, no remote image
    <img
      src={src}
      alt={`QR-Code. Er führt zu ${url}`}
      className="h-full w-full"
    />
  );
}

/**
 * A link as a QR code to scan from the screen, in the moto look: the stored
 * logo in the middle, or the moto logo without one.
 */
export function BrandedQrCode({
  url,
  brandLogo,
  className,
}: Readonly<{ url: string; brandLogo?: QrBrandLogo; className?: string }>) {
  const brand = useQrLogo(brandLogo?.src ?? null, true);
  const useMoto = brandLogo === undefined || brand.status === "failed";
  const moto = useQrLogo(useMoto ? MOTO_QR_LOGO : null, true);
  const active = useMoto ? moto : brand;

  return (
    <div className={className}>
      {active.status === "loading" ? (
        <Skeleton shape="inner" className="h-full w-full" />
      ) : (
        // Without any readable logo the plain code still works.
        <QrCodeImage url={url} logo={active.logo} />
      )}
    </div>
  );
}
