"use client";
import { useState } from "react";

import { Activity, ArrowRight } from "lucide-react";
import { ApiClientError } from "@/lib/api/client";
import { useLogin } from "@/hooks/use-auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { copy } from "@/lib/copy/app";
export default function Login() {
  const login = useLogin();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  return (
    <main id="main-content" className="login-screen">
      <div className="login-card">
        <span className="brand-mark">
          <Activity size={25} />
        </span>
        <div className="eyebrow">{copy.console}</div>
        <h1>{copy.loginTitle}</h1>
        <p>{copy.loginDescription}</p>
        <form
          onSubmit={async (event) => {
            event.preventDefault();
            const data = new FormData(event.currentTarget);
            setPending(true);
            setError("");
            try {
              await login.mutateAsync(String(data.get("password") ?? ""));
              const next =
                new URLSearchParams(window.location.search).get("next") ?? "/";
              window.location.replace(
                next.startsWith("/") &&
                  !next.startsWith("//") &&
                  !next.includes("\\")
                  ? next
                  : "/",
              );
            } catch (error) {
              setError(
                error instanceof ApiClientError
                  ? `${error.message} (${copy.requestId}: ${error.requestId})`
                  : copy.loginFailed,
              );
            } finally {
              setPending(false);
            }
          }}
        >
          <label htmlFor="password">{copy.password}</label>
          <Input
            id="password"
            name="password"
            type="password"
            autoComplete="current-password"
            required
            aria-invalid={Boolean(error)}
            aria-describedby={error ? "login-error" : undefined}
            disabled={pending}
          />
          {error && (
            <p id="login-error" role="alert" className="login-error">
              {error}
            </p>
          )}
          <Button type="submit" disabled={pending}>
            {pending ? copy.signingIn : copy.login}
            <ArrowRight size={16} />
          </Button>
        </form>
        <small>
          <Shield />
          {copy.authentication}
        </small>
      </div>
    </main>
  );
}
function Shield() {
  return <span className="mini-dot" />;
}
