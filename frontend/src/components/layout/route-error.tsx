"use client";
import { Button } from "@/components/ui/button";
import { routeErrorCopy as c } from "@/lib/copy/polish";
export function RouteError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <section className="error-state" role="alert">
      <h1>{c.title}</h1>
      <p>{c.help}</p>
      {error.digest && <code>{error.digest}</code>}
      <Button onClick={reset}>{c.retry}</Button>
    </section>
  );
}
