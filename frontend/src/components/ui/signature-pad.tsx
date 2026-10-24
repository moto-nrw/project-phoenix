"use client";

/**
 * Unterschriftsfeld (#4785): eine Zeichenfläche für Finger, Stift und Maus,
 * „Löschen“, „Namen tippen“ als Weg ohne Zeichnen (Tastatur, Barrierefreiheit)
 * und ein Bestätigen-Schritt. Erst die Bestätigung liefert den Wert; vorher
 * ist er `null`.
 *
 * Das Feld kennt keinen Zweck. Einverständnisse, die Online-Anmeldung (#4786,
 * auch öffentlich ohne Konto) und die Abholerlaubnis (#4780) binden es gleich
 * ein und schicken den Wert an ihren eigenen Endpunkt, wo der Backend-Baustein
 * „Unterschrift“ ihn prüft und speichert.
 *
 * Texte: wie die übrigen Kit-Bausteine mit deutschen Vorgaben über Props
 * (`labels`). Das Eltern-Portal reicht seine übersetzten Texte hinein
 * (`useSignaturePadLabels` im Aufrufer), deutschsprachige Portale lassen sie weg.
 *
 * Grenzen, die der Server prüft: PNG, höchstens 1600 × 800 Pixel und 200 KB,
 * mindestens ein gezeichneter Punkt. Die Fläche hält die Pixelgröße unter der
 * Grenze, auch bei hoher Bildschirmauflösung.
 */

import Image from "next/image";
import {
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

import { Button } from "~/components/ui/button";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { Input } from "~/components/ui/input";
import type { SignatureValue } from "~/lib/signature";

export type { SignatureValue };

export interface SignaturePadLabels {
  readonly label: string;
  readonly hint: string;
  readonly canvas: string;
  readonly clear: string;
  readonly typeInstead: string;
  readonly drawInstead: string;
  readonly typedName: string;
  readonly confirm: string;
  readonly confirmed: string;
  readonly confirmedTyped: string;
  readonly change: string;
  readonly preview: string;
  readonly emptyDrawing: string;
  readonly nameTooShort: string;
}

const SIGNATURE_PAD_LABELS_DE: SignaturePadLabels = {
  label: "Unterschrift",
  hint: "Unterschreiben Sie mit dem Finger, dem Stift oder der Maus.",
  canvas: "Feld zum Unterschreiben",
  clear: "Feld leeren",
  typeInstead: "Namen tippen",
  drawInstead: "Lieber zeichnen",
  typedName: "Ihr Name",
  confirm: "Unterschrift bestätigen",
  confirmed: "Ihre Unterschrift ist bereit.",
  confirmedTyped: "Ihr getippter Name ist bereit.",
  change: "Neu unterschreiben",
  preview: "Ihre Unterschrift",
  emptyDrawing: "Bitte unterschreiben Sie im Feld.",
  nameTooShort: "Bitte tippen Sie Ihren Namen.",
};

/** Upper bounds the backend enforces; the canvas stays below them. */
const SIGNATURE_MAX_WIDTH = 1600;
const SIGNATURE_MAX_HEIGHT = 800;
const SIGNATURE_TYPED_NAME_MIN = 2;
const SIGNATURE_TYPED_NAME_MAX = 120;

const DISPLAY_HEIGHT = 160;
const STROKE_COLOR = "#111827";
const TYPED_FONT =
  '"Segoe Script", "Brush Script MT", "Apple Chancery", cursive';

type Mode = "draw" | "type";

/** Strips the `data:image/png;base64,` prefix of a canvas export. */
export function pngBase64FromDataUrl(dataUrl: string): string {
  const comma = dataUrl.indexOf(",");
  return comma === -1 ? dataUrl : dataUrl.slice(comma + 1);
}

export function SignaturePad({
  id,
  value,
  onChange,
  disabled = false,
  error,
  signerNameHint,
  labels: labelOverrides,
}: Readonly<{
  /** Base id for the field, its hint and its error. */
  id?: string;
  /** The confirmed signature, or `null` while nothing is confirmed. */
  value: SignatureValue | null;
  onChange: (value: SignatureValue | null) => void;
  disabled?: boolean;
  /** Field error (e.g. from the server); shown under the field. */
  error?: string | null;
  /** Prefills the typed name, e.g. the signed-in guardian's name. */
  signerNameHint?: string;
  labels?: Partial<SignaturePadLabels>;
}>) {
  const labels = { ...SIGNATURE_PAD_LABELS_DE, ...labelOverrides };
  const generatedId = useId();
  const baseId = id ?? `signature-${generatedId}`;
  const hintId = `${baseId}-hint`;
  const errorId = `${baseId}-error`;

  const canvasRef = useRef<HTMLCanvasElement>(null);
  const drawing = useRef(false);
  const last = useRef<{ x: number; y: number } | null>(null);
  const [mode, setMode] = useState<Mode>("draw");
  const [hasInk, setHasInk] = useState(false);
  const [typedName, setTypedName] = useState(signerNameHint ?? "");
  const [localError, setLocalError] = useState<string | null>(null);

  const context = useCallback((): CanvasRenderingContext2D | null => {
    return canvasRef.current?.getContext("2d") ?? null;
  }, []);

  // Pixel size: the shown size times the device ratio, capped at the server's
  // bounds. Resizing clears the drawing, so it only runs on mount and mode.
  const sizeCanvas = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ratio =
      typeof window === "undefined" ? 1 : window.devicePixelRatio || 1;
    const width = Math.max(1, canvas.clientWidth || 600);
    canvas.width = Math.min(Math.round(width * ratio), SIGNATURE_MAX_WIDTH);
    canvas.height = Math.min(
      Math.round(DISPLAY_HEIGHT * ratio),
      SIGNATURE_MAX_HEIGHT,
    );
  }, []);

  const clearCanvas = useCallback(() => {
    const canvas = canvasRef.current;
    const ctx = context();
    if (canvas && ctx) ctx.clearRect(0, 0, canvas.width, canvas.height);
    setHasInk(false);
  }, [context]);

  const renderTyped = useCallback(
    (name: string) => {
      const canvas = canvasRef.current;
      const ctx = context();
      if (!canvas || !ctx) return;
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      const text = name.trim();
      if (!text) return;
      let size = Math.round(canvas.height * 0.4);
      ctx.fillStyle = STROKE_COLOR;
      ctx.textBaseline = "middle";
      ctx.font = `italic ${size}px ${TYPED_FONT}`;
      // Shrink long names until they fit the width.
      while (size > 12 && ctx.measureText(text).width > canvas.width * 0.9) {
        size -= 2;
        ctx.font = `italic ${size}px ${TYPED_FONT}`;
      }
      ctx.fillText(text, canvas.width * 0.05, canvas.height / 2);
    },
    [context],
  );

  useLayoutEffect(() => {
    if (value) return;
    sizeCanvas();
    setHasInk(false);
    if (mode === "type") renderTyped(typedName);
    // typedName is rendered by its own handler; re-sizing on every keystroke
    // would only redraw the same text.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mode, value, sizeCanvas, renderTyped]);

  useEffect(() => {
    setLocalError(null);
  }, [mode]);

  const point = (event: React.PointerEvent<HTMLCanvasElement>) => {
    const canvas = event.currentTarget;
    const rect = canvas.getBoundingClientRect();
    const scaleX = rect.width ? canvas.width / rect.width : 1;
    const scaleY = rect.height ? canvas.height / rect.height : 1;
    return {
      x: (event.clientX - rect.left) * scaleX,
      y: (event.clientY - rect.top) * scaleY,
    };
  };

  const strokeTo = (to: { x: number; y: number }) => {
    const ctx = context();
    const canvas = canvasRef.current;
    if (!ctx || !canvas) return;
    const from = last.current ?? to;
    ctx.strokeStyle = STROKE_COLOR;
    ctx.fillStyle = STROKE_COLOR;
    ctx.lineWidth = Math.max(2, canvas.height / 60);
    ctx.lineCap = "round";
    ctx.lineJoin = "round";
    ctx.beginPath();
    ctx.moveTo(from.x, from.y);
    ctx.lineTo(to.x + (from === to ? 0.1 : 0), to.y);
    ctx.stroke();
    last.current = to;
    setHasInk(true);
  };

  const onPointerDown = (event: React.PointerEvent<HTMLCanvasElement>) => {
    if (disabled || mode !== "draw") return;
    event.preventDefault();
    event.currentTarget.setPointerCapture?.(event.pointerId);
    drawing.current = true;
    last.current = null;
    setLocalError(null);
    strokeTo(point(event));
  };

  const onPointerMove = (event: React.PointerEvent<HTMLCanvasElement>) => {
    if (!drawing.current) return;
    event.preventDefault();
    strokeTo(point(event));
  };

  const endStroke = () => {
    drawing.current = false;
    last.current = null;
  };

  const confirm = () => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    if (mode === "type") {
      const name = typedName.trim();
      if (Array.from(name).length < SIGNATURE_TYPED_NAME_MIN) {
        setLocalError(labels.nameTooShort);
        return;
      }
      renderTyped(name);
      onChange({
        method: "typed",
        imagePngBase64: pngBase64FromDataUrl(canvas.toDataURL("image/png")),
        typedName: name,
      });
      return;
    }
    if (!hasInk) {
      setLocalError(labels.emptyDrawing);
      return;
    }
    onChange({
      method: "drawn",
      imagePngBase64: pngBase64FromDataUrl(canvas.toDataURL("image/png")),
    });
  };

  const shownError = localError ?? error ?? null;

  if (value) {
    return (
      <div className="space-y-2" id={baseId}>
        <p className="text-sm font-medium text-gray-700">{labels.label}</p>
        <div className="rounded-xl bg-gray-50 p-2">
          {/* A canvas export: a data URL next/image cannot optimise. */}
          <Image
            src={`data:image/png;base64,${value.imagePngBase64}`}
            alt={labels.preview}
            width={600}
            height={160}
            unoptimized
            className="h-24 w-full object-contain"
          />
        </div>
        <p className="text-sm text-gray-700" role="status">
          {value.method === "typed" ? labels.confirmedTyped : labels.confirmed}
        </p>
        <Button
          type="button"
          variant="outline"
          size="md"
          disabled={disabled}
          onClick={() => {
            setLocalError(null);
            onChange(null);
          }}
        >
          {labels.change}
        </Button>
        <FieldErrorHint id={errorId} message={error} />
      </div>
    );
  }

  return (
    <div className="space-y-2" id={baseId}>
      <p className="text-sm font-medium text-gray-700">{labels.label}</p>
      <p id={hintId} className="text-sm text-gray-600">
        {labels.hint}
      </p>
      {mode === "type" && (
        <Input
          id={`${baseId}-typed`}
          name={`${baseId}-typed`}
          label={labels.typedName}
          value={typedName}
          // HTML counts UTF-16 code units; onChange also enforces the server's
          // 120-code-point limit, including for pasted mixed Unicode text.
          maxLength={SIGNATURE_TYPED_NAME_MAX * 2}
          autoComplete="name"
          disabled={disabled}
          onChange={(event) => {
            const name = Array.from(event.target.value)
              .slice(0, SIGNATURE_TYPED_NAME_MAX)
              .join("");
            setTypedName(name);
            setLocalError(null);
            renderTyped(name);
          }}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              confirm();
            }
          }}
        />
      )}
      <canvas
        ref={canvasRef}
        role="img"
        aria-label={labels.canvas}
        aria-describedby={[hintId, shownError ? errorId : null]
          .filter(Boolean)
          .join(" ")}
        className={`block w-full rounded-xl border bg-gray-50 ${
          shownError ? "border-moto-red" : "border-gray-300"
        } ${mode === "draw" && !disabled ? "cursor-crosshair" : ""}`}
        style={{ height: DISPLAY_HEIGHT, touchAction: "none" }}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={endStroke}
        onPointerCancel={endStroke}
        onPointerLeave={endStroke}
      />
      <FieldErrorHint id={errorId} message={shownError} live={!!localError} />
      <div className="flex flex-wrap gap-2">
        <Button
          type="button"
          variant="primary"
          size="md"
          disabled={disabled}
          onClick={confirm}
        >
          {labels.confirm}
        </Button>
        {mode === "draw" && (
          <Button
            type="button"
            variant="outline"
            size="md"
            disabled={disabled || !hasInk}
            onClick={clearCanvas}
          >
            {labels.clear}
          </Button>
        )}
        <Button
          type="button"
          variant="ghost"
          size="md"
          disabled={disabled}
          onClick={() => setMode(mode === "draw" ? "type" : "draw")}
        >
          {mode === "draw" ? labels.typeInstead : labels.drawInstead}
        </Button>
      </div>
    </div>
  );
}
