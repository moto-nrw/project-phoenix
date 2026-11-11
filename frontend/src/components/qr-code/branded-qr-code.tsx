"use client";

import { useMemo } from "react";
import { Skeleton } from "~/components/ui/skeleton";
import { brandedQrLayout, brandedQrSvg } from "~/lib/qr-code/branded-qr";
import type { QrLogo } from "~/lib/qr-code/qr-image";
import { useQrBrandLogo, type QrBrand } from "./qr-brand-logo";
import { useQrLogo } from "./use-qr-logo";

/** The moto mark without its word: at this size the figures stay readable. */
export const MOTO_QR_LOGO = "/images/moto_transparent.png";

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
  brand: brandOwner,
  className,
}: Readonly<{ url: string; brand: QrBrand; className?: string }>) {
  const brandLogo = useQrBrandLogo(brandOwner);
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
