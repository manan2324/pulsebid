"use client";
import { useEffect, useState } from "react";
import dynamic from "next/dynamic";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTheme } from "next-themes";
import {
  Activity,
  ArrowUpRight,
  BarChart3,
  Building2,
  ChevronRight,
  FlaskConical,
  Gauge,
  Layers3,
  LogOut,
  Menu,
  Moon,
  Radio,
  Search,
  Server,
  Sun,
  X,
} from "lucide-react";
import { copy } from "@/lib/copy/app";
import { useHealth } from "@/hooks/use-health";
import { useTimeMode } from "@/components/providers";
import { useLogout } from "@/hooks/use-auth";
import { Button } from "@/components/ui/button";
const CommandPalette = dynamic(
  () => import("./command-palette").then((m) => m.CommandPalette),
  { ssr: false },
);
const KeyboardHelp = dynamic(
  () => import("./keyboard-help").then((m) => m.KeyboardHelp),
  { ssr: false },
);
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
const icons = [
  Gauge,
  Layers3,
  Building2,
  Radio,
  BarChart3,
  Activity,
  FlaskConical,
  Server,
];
export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();

  const logout = useLogout();
  const page = copy.nav.find((item) =>
    item.href === "/" ? pathname === "/" : pathname.startsWith(item.href),
  );
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState(false);
  const [help, setHelp] = useState(false);
  const { theme, resolvedTheme, setTheme } = useTheme();
  const { mode, toggle } = useTimeMode();
  const health = useHealth();
  const healthy =
    health.data?.services.filter((service) => service.status === "healthy")
      .length ?? 0;
  const total = health.data?.services.length ?? 0;
  const status = !health.data
    ? "pending"
    : healthy === total
      ? "healthy"
      : healthy > 0
        ? "degraded"
        : "unavailable";
  const label =
    status === "pending"
      ? copy.checking
      : status === "healthy"
        ? copy.healthy
        : status === "degraded"
          ? copy.degraded
          : copy.unavailable;
  useEffect(() => {
    function key(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key === "k") {
        event.preventDefault();
        setSearch((value) => !value);
      }
      if (
        event.key === "?" &&
        !(
          event.target instanceof HTMLElement &&
          (event.target.matches("input,textarea,select") ||
            event.target.isContentEditable)
        )
      )
        setHelp(true);
      if (event.key === "Escape") setOpen(false);
    }
    document.addEventListener("keydown", key);

    return () => document.removeEventListener("keydown", key);
  }, []);
  const sidebar = (
    <>
      <Link href="/" className="brand" onClick={() => setOpen(false)}>
        <span className="brand-mark">
          <Activity size={23} strokeWidth={2.5} />
        </span>
        <span>
          {copy.brand}
          <small>{copy.console}</small>
        </span>
      </Link>
      <div className="workspace-chip">
        <span className="workspace-icon">P</span>
        <div>
          {copy.workspace}
          <small>{copy.development}</small>
        </div>
        <span className="workspace-dot" />
      </div>
      <div className="nav-caption">{copy.platform}</div>
      <nav aria-label={copy.all}>
        {copy.nav.map((item, index) => {
          const Icon = icons[index] ?? Gauge;
          const active = item === page;
          return (
            <Link
              key={item.href}
              href={item.href}
              aria-current={active ? "page" : undefined}
              className={`nav-link ${active ? "nav-active" : ""}`}
              onClick={() => setOpen(false)}
            >
              <Icon size={18} />
              <span>{item.label}</span>
              {active && <ChevronRight size={15} />}
            </Link>
          );
        })}
      </nav>
      <div className="sidebar-bottom">
        <div className="foundation-note">
          <span className="mini-dot" />
          {copy.foundations}
          <small>{copy.phaseHelp}</small>
        </div>
        <div className="external-links">
          <a
            href={
              process.env.NEXT_PUBLIC_GRAFANA_URL ?? "http://localhost:3000"
            }
            target="_blank"
            rel="noreferrer"
          >
            {copy.grafana}
            <ArrowUpRight size={13} />
          </a>
          <a
            href={
              process.env.NEXT_PUBLIC_JAEGER_URL ?? "http://localhost:16686"
            }
            target="_blank"
            rel="noreferrer"
          >
            {copy.jaeger}
            <ArrowUpRight size={13} />
          </a>
        </div>
      </div>
    </>
  );
  return (
    <div className="console-shell">
      <aside className="sidebar">{sidebar}</aside>
      <Sheet open={open} onOpenChange={setOpen}>
        <SheetContent
          side="left"
          className="mobile-sidebar"
          aria-describedby={undefined}
        >
          <SheetTitle className="sr-only">{copy.workspace}</SheetTitle>
          {sidebar}
        </SheetContent>
      </Sheet>
      <div className="workspace-main">
        <header className="topbar">
          <div className="breadcrumb">
            <Button
              variant="ghost"
              size="icon"
              className="mobile-menu"
              aria-label={copy.menu}
              aria-expanded={open}
              onClick={() => setOpen(!open)}
            >
              {open ? <X /> : <Menu />}
            </Button>
            <span className="breadcrumb-root">{copy.workspace}</span>
            <ChevronRight size={13} />
            <strong>{page?.label ?? copy.campaignDetail}</strong>
          </div>
          <div className="topbar-actions">
            <Button
              variant="ghost"
              className="search-button"
              aria-label={`${copy.search}: ⌘ K`}
              onClick={() => setSearch(true)}
            >
              <Search size={16} />
              <kbd>⌘ K</kbd>
            </Button>
            <span className="topbar-divider" />
            <Button
              variant="ghost"
              size="sm"
              onClick={toggle}
              aria-label={`${copy.time}: ${mode}`}
              className="timezone"
            >
              {mode}
            </Button>
            <Button
              variant="ghost"
              size="icon"
              aria-label={copy.theme}
              onClick={() =>
                setTheme(resolvedTheme === "dark" ? "light" : "dark")
              }
            >
              <Sun size={17} className="dark:hidden" />
              <Moon size={17} className="hidden dark:block" />
            </Button>
            <select
              className="theme-select"
              aria-label={`${copy.theme}: ${copy.system}`}
              value={theme ?? "system"}
              onChange={(event) => setTheme(event.target.value)}
            >
              <option value="system">{copy.system}</option>
              <option value="light">{copy.light}</option>
              <option value="dark">{copy.dark}</option>
            </select>
            <Link
              href="/system"
              className="health-indicator"
              aria-label={`${health.data ? `${healthy}/${total}: ` : ""}${label}`}
              title={label}
            >
              <span className={`health-dot ${status}`} />
              <span>{health.data ? `${healthy}/${total}` : "…"}</span>
            </Link>
            <Button
              variant="ghost"
              size="icon"
              aria-label={copy.logout}
              onClick={() => {
                logout.mutate(undefined, {
                  onSuccess: () => {
                    window.location.replace("/login");
                  },
                });
              }}
            >
              <LogOut size={16} />
            </Button>
          </div>
        </header>
        <main id="main-content" className="page-content">
          {children}
        </main>
        <footer className="workspace-footer">
          <span>
            <span className="mini-dot" />
            {copy.secure}
          </span>
          <span>
            {copy.timestamp}: {mode}
          </span>
        </footer>
      </div>
      {search && (
        <CommandPalette open={search} onClose={() => setSearch(false)} />
      )}
      {help && <KeyboardHelp open={help} onClose={() => setHelp(false)} />}
    </div>
  );
}
