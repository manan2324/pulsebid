"use client";
import { copy } from "@/lib/copy/app";
export default function GlobalError({
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <html lang="en">
      <body>
        <main id="main-content">
          <h1>{copy.error}</h1>
          <p>{copy.errorDescription}</p>
          <button onClick={reset}>{copy.reset}</button>
        </main>
      </body>
    </html>
  );
}
