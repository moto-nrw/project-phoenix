/**
 * The value of the signature field (#4785), shared by every flow that asks
 * for a signature: Einverständnisse, the enrollment (#4786) and the pickup
 * permission (#4780). `imagePngBase64` carries no `data:` prefix.
 */
export interface SignatureValue {
  readonly method: "drawn" | "typed";
  readonly imagePngBase64: string;
  readonly typedName?: string;
}

/** The request body shape every signing endpoint accepts. */
export interface SignatureRequestBody {
  readonly method: "drawn" | "typed";
  readonly image_png: string;
  readonly typed_name?: string;
}

/** Maps the field value to the request body. */
export function signatureRequestBody(
  value: SignatureValue,
): SignatureRequestBody {
  return {
    method: value.method,
    image_png: value.imagePngBase64,
    ...(value.typedName === undefined ? {} : { typed_name: value.typedName }),
  };
}
