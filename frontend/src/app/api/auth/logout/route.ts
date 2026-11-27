import { NextResponse } from "next/server";
import { SESSION_COOKIE } from "@/lib/server/session";
import { apiError, prepareMutation, requestId } from "@/lib/server/http";

export async function POST(request: Request) {
  const id = requestId(request);
  const { response: guard } = await prepareMutation(request, id);
  if (guard) return guard;
  if (!request.headers.get("cookie"))
    return apiError(401, "unauthorized", "Sign in first", id);
  const response = NextResponse.json(
    { ok: true },
    { headers: { "X-Request-Id": id, "Cache-Control": "no-store" } },
  );
  response.cookies.set(SESSION_COOKIE, "", {
    maxAge: 0,
    path: "/",
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
  });
  return response;
}
