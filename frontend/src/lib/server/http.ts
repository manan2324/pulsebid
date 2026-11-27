export function requestId(request: Request) {
  const value = request.headers.get("x-request-id");
  return value && /^[a-zA-Z0-9_.:-]{1,128}$/.test(value)
    ? value
    : crypto.randomUUID();
}
export function apiError(
  status: number,
  code: string,
  message: string,
  id: string,
) {
  return Response.json(
    { error: { code, message, request_id: id } },
    { status, headers: { "X-Request-Id": id, "Cache-Control": "no-store" } },
  );
}
export function mutationGuard(
  request: Request,
  id: string,
  bodyLength = request.body ? 1 : 0,
) {
  if (!["POST", "PUT", "PATCH", "DELETE"].includes(request.method)) return null;
  if (
    request.headers.get("origin") !==
    new URL(process.env.APP_ORIGIN ?? "http://localhost:3001").origin
  )
    return apiError(403, "forbidden", "Request origin is not allowed", id);
  if (
    bodyLength > 0 &&
    request.headers.get("content-type")?.split(";")[0]?.trim().toLowerCase() !==
      "application/json"
  )
    return apiError(
      415,
      "unsupported_media_type",
      "Content-Type must be application/json",
      id,
    );
  return null;
}

// Next can expose an empty stream for a bodyless DELETE. Check bytes, then reuse the body.
export async function prepareMutation(request: Request, id: string) {
  const originError = mutationGuard(request, id, 0);
  if (originError) return { response: originError, body: undefined };
  try {
    const body = request.body ? await request.arrayBuffer() : undefined;
    const response = mutationGuard(request, id, body?.byteLength ?? 0);
    return { response, body: body?.byteLength ? body : undefined };
  } catch {
    return {
      response: apiError(
        400,
        "invalid_request",
        "Request body could not be read",
        id,
      ),
      body: undefined,
    };
  }
}
