"use client";
import { copy } from "@/lib/copy/app";
import { Button } from "@/components/ui/button";

export default function ErrorPage({
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <main id="main-content" className="standalone-state">
      <h1>{copy.error}</h1>
      <p>{copy.errorDescription}</p>
      <Button onClick={reset}>{copy.reset}</Button>
    </main>
  );
}
