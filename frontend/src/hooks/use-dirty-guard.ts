"use client";
import { useEffect, useRef, useCallback } from "react";
import { discardChanges } from "@/lib/copy/polish";
const confirmed = new WeakSet<Event>();
export function useDirtyGuard(dirty: boolean) {
  const allowed = useRef(false);
  useEffect(() => {
    if (!dirty) {
      allowed.current = false;
      return;
    }
    const handler = (event: BeforeUnloadEvent) => {
      if (allowed.current) return;
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", handler);
    const navigate = (event: MouseEvent) => {
      if (
        allowed.current ||
        event.defaultPrevented ||
        confirmed.has(event) ||
        event.button !== 0 ||
        event.ctrlKey ||
        event.metaKey ||
        event.shiftKey ||
        event.altKey
      )
        return;
      const link =
        event.target instanceof Element
          ? event.target.closest("a[href]")
          : null;
      if (
        !(link instanceof HTMLAnchorElement) ||
        link.target === "_blank" ||
        link.hasAttribute("download")
      )
        return;
      const destination = new URL(link.href, location.href);
      if (
        destination.pathname === location.pathname &&
        destination.search === location.search
      )
        return;
      if (!window.confirm(discardChanges)) {
        event.preventDefault();
        event.stopImmediatePropagation();
        return;
      }
      confirmed.add(event);
      allowed.current = true;
      window.setTimeout(() => {
        allowed.current = false;
      }, 1000);
    };
    document.addEventListener("click", navigate, true);
    return () => {
      window.removeEventListener("beforeunload", handler);
      document.removeEventListener("click", navigate, true);
    };
  }, [dirty]);
  return useCallback(() => {
    allowed.current = true;
  }, []);
}
