import { NextRequest, NextResponse } from "next/server";
import { SESSION_COOKIE, verifySession } from "@/lib/server/session";
import { apiError, requestId } from "@/lib/server/http";
export async function middleware(request: NextRequest) {
  const id = requestId(request);
  const path = request.nextUrl.pathname;
  const disabled =
    process.env.AUTH_DISABLED === "true" &&
    process.env.NODE_ENV !== "production";
  const publicPath =
    path === "/login" ||
    path === "/api/auth/login" ||
    path === "/api/auth/logout";
  if (
    !publicPath &&
    !disabled &&
    !(await verifySession(
      request.cookies.get(SESSION_COOKIE)?.value,
      process.env.SESSION_SECRET ?? "",
    ))
  ) {
    if (path.startsWith("/api/"))
      return apiError(401, "unauthorized", "Authentication required", id);
    const login = new URL("/login", request.url);
    login.searchParams.set("next", path + request.nextUrl.search);
    return NextResponse.redirect(login);
  }
  const nonce = btoa(crypto.randomUUID());
  const dev = process.env.NODE_ENV !== "production";
  const csp = `default-src 'self'; script-src 'self' 'nonce-${nonce}' 'strict-dynamic'${dev ? " 'unsafe-eval'" : ""}; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; frame-src 'self'; connect-src 'self'${dev ? " ws: http://localhost:*" : ""}; worker-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'`;
  const headers = new Headers(request.headers);
  headers.set("x-nonce", nonce);
  headers.set("x-request-id", id);
  headers.set("Content-Security-Policy", csp);
  const response = NextResponse.next({ request: { headers } });
  response.headers.set("Content-Security-Policy", csp);
  response.headers.set("X-Request-Id", id);
  return response;
}
export const config = {
  matcher: [
    "/((?!_next/static|_next/image|favicon.ico|icon.svg|mockServiceWorker.js|fonts/).*)",
  ],
};
