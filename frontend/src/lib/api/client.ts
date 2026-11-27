import { copy } from "@/lib/copy/app";
export interface ErrorDetail {
  field: string;
  issue: string;
}
export class ApiClientError extends Error {
  constructor(
    public code: string,
    message: string,
    public details: ErrorDetail[],
    public requestId: string,
    public status: number,
  ) {
    super(message);
    this.name = "ApiClientError";
  }
}
export function queryString(
  params: Record<
    string,
    string | number | boolean | readonly string[] | null | undefined
  >,
) {
  const result = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null) continue;
    if (Array.isArray(value)) value.forEach((item) => result.append(key, item));
    else result.set(key, String(value));
  }
  return result.size ? `?${result}` : "";
}
function record(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}
export async function apiRequest<T>(
  path: string,
  options: RequestInit = {},
  parse?: (input: unknown) => T,
): Promise<T> {
  if (!path.startsWith("/api/") || path.includes("\\") || path.includes("#"))
    throw new Error("API requests must use a same-origin /api/ path");
  const headers = new Headers(options.headers);
  headers.set("Accept", "application/json");
  if (options.body) headers.set("Content-Type", "application/json");
  const response = await fetch(path, {
    ...options,
    headers,
    credentials: "same-origin",
    cache: "no-store",
  });
  const body: unknown =
    response.status === 204
      ? undefined
      : await response.json().catch(() => undefined);
  if (!response.ok) {
    const error = record(body) && record(body.error) ? body.error : {};
    const details = Array.isArray(error.details)
      ? error.details.filter(
          (item): item is ErrorDetail =>
            record(item) &&
            typeof item.field === "string" &&
            typeof item.issue === "string",
        )
      : [];
    throw new ApiClientError(
      typeof error.code === "string" ? error.code : "internal",
      typeof error.message === "string"
        ? error.message
        : `Request failed (${response.status})`,
      details,
      typeof error.request_id === "string"
        ? error.request_id
        : (response.headers.get("x-request-id") ?? "unknown"),
      response.status,
    );
  }
  if (!parse) return body as T;
  try {
    return parse(body);
  } catch (error) {
    if (error instanceof ApiClientError) throw error;
    throw new ApiClientError(
      "invalid_response",
      error instanceof Error ? error.message : copy.invalidResponse,
      [],
      response.headers.get("x-request-id") ?? "unknown",
      response.status,
    );
  }
}
export const api = {
  get: <T>(path: string, signal?: AbortSignal, parse?: (input: unknown) => T) =>
    apiRequest<T>(path, { signal }, parse),
  post: <T>(path: string, body: unknown, signal?: AbortSignal) =>
    apiRequest<T>(path, { method: "POST", body: JSON.stringify(body), signal }),
};
