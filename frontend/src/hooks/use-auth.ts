"use client";
import { useMutation } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
export function useLogin() {
  return useMutation({
    mutationFn: (password: string) =>
      api.post<{ ok: boolean }>("/api/auth/login", { password }),
  });
}
export function useLogout() {
  return useMutation({
    mutationFn: () => api.post<{ ok: boolean }>("/api/auth/logout", {}),
  });
}
