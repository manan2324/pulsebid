#!/usr/bin/env python3
"""Exact, isolated system proofs. Requires Python 3 and the local apps stack."""
import datetime
import decimal
import json
import os
import pathlib
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

ROOT = pathlib.Path(__file__).resolve().parent.parent
CAMPAIGN = os.environ.get("CAMPAIGN_URL", "http://localhost:8081")
GATEWAY = os.environ.get("GATEWAY_URL", "http://localhost:8088")
REPORTING = os.environ.get("REPORTING_URL", "http://localhost:8083")
KEY = os.environ.get("API_KEY", "dev-key")
REPORT = pathlib.Path(os.environ.get("REPORT_DIR", str(ROOT / "docs/test-reports" / str(datetime.date.today()) / "verification")))
REPORT.mkdir(parents=True, exist_ok=True)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def request(url, method="GET", body=None):
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(url, data=data, method=method,
                                 headers={"X-API-Key": KEY, "Content-Type": "application/json"})
    try:
        response = urllib.request.build_opener(NoRedirect).open(req, timeout=10)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        raw = response.read()
        status, headers = response.code, response.headers
    if status >= 400:
        raise RuntimeError(f"{method} {url}: {status} {raw[:300]!r}")
    if headers.get("Content-Type", "").startswith("application/json") and raw:
        return status, json.loads(raw), headers
    return status, raw, headers


def api(path, method="GET", body=None):
    return request(CAMPAIGN + path, method, body)[1]


def write(name, result):
    (REPORT / name).write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(result, indent=2))


def fixture(budget):
    suffix = uuid.uuid4().hex
    adv = api("/v1/advertisers", "POST", {"name": "Verification " + suffix})
    bidder = "verify-" + suffix[:16]
    api("/v1/bidders", "POST", {"id": bidder, "name": bidder,
        "endpoint_url": os.environ.get("BIDDER_ENDPOINT", "http://mock-bidders:9101/bid"),
        "timeout_ms": 80, "enabled": True})
    camp = api("/v1/campaigns", "POST", {"name": "Verification " + suffix,
        "advertiser_id": adv["id"], "bidder_id": bidder, "daily_budget": budget,
        "max_bid_cpm": 3, "pacing_mode": "asap"})
    path = "/v1/campaigns/" + camp["id"]
    api(path + "/targeting", "PUT", {"countries": ["IN"], "device_types": ["mobile"],
        "placement_ids": [suffix], "timezone": "UTC"})
    api(path + "/creatives", "POST", {"name": "Verification", "format": "banner",
        "width": 300, "height": 250, "markup": '<img src="{impression_url}">',
        "landing_url": "https://example.com/verification"})
    api(path + "/activate", "POST")
    time.sleep(3)
    return camp["id"], suffix


def bid(placement, number):
    return request(GATEWAY + "/v1/bid", "POST", {"id": f"verification-{uuid.uuid4()}-{number}",
        "imp": {"id": "1", "placement_id": placement, "w": 300, "h": 250, "bidfloor": 1},
        "device": {"type": "mobile"}, "geo": {"country": "IN"}, "tmax": 100})


def replica_counters():
    ids = subprocess.check_output(["docker", "ps", "-q", "--filter", "label=com.docker.compose.project=pulsebid",
                                   "--filter", "label=com.docker.compose.service=bid-gateway"], text=True).split()
    if len(ids) != 3:
        raise RuntimeError(f"Expected exactly 3 running gateways, found {len(ids)}")
    counters = {}
    for container in ids:
        port = subprocess.check_output(["docker", "port", container, "8090/tcp"], text=True).strip().split(":")[-1]
        _, raw, _ = request(f"http://localhost:{port}/metrics")
        counters[container] = sum(float(line.split()[-1]) for line in raw.decode().splitlines()
                                  if line.startswith("bid_requests_total{"))
    return counters


def overspend():
    campaign, placement = fixture(0.05)
    try:
        before = replica_counters()
        subprocess.run(["docker", "run", "--rm", "--network", "pulsebid_default",
            "--mount", f"type=bind,source={ROOT / 'loadtest/k6'},target=/scripts,readonly",
            "--mount", f"type=bind,source={REPORT.resolve()},target=/reports",
            "-e", "GATEWAY=http://traefik:8088", "-e", f"PLACEMENT={placement}",
            "-e", f"CAMPAIGN_ID={campaign}", "grafana/k6:latest", "run",
            "--summary-export=/reports/overspend-k6.json", "/scripts/overspend.js"], check=True)
        summary = json.loads((REPORT / "overspend-k6.json").read_text())
        fills = int(summary["metrics"]["fills"]["count"])
        if summary["metrics"]["http_reqs"]["count"] != 2000:
            raise RuntimeError("k6 did not complete all 2,000 requests")
        budget = api(f"/v1/campaigns/{campaign}/budget")
        after = replica_counters()
        deltas = {key: after[key] - value for key, value in before.items()}
        if not 0 < fills <= 49 or budget["spent_micros"] != 49 * 1010 or budget["remaining_micros"] != 510:
            raise RuntimeError(f"Exact cap proof failed: fills={fills}, budget={budget}")
        if any(value <= 0 for value in deltas.values()) or sum(deltas.values()) != 2000:
            raise RuntimeError(f"Traffic did not reach every replica exclusively: {deltas}")
        write("overspend-proof.json", {"pass": True, "requests": 2000, "fills": fills,
            "reservations": 49, "unconfirmed_reservations": 49 - fills,
            "error_no_fills": int(summary["metrics"].get("error_no_fills", {}).get("count", 0)),
            "bidder_no_fills": int(summary["metrics"].get("bidder_no_fills", {}).get("count", 0)),
            "note": "No-refund/fail-closed semantics can charge a reservation whose Redis response times out. Unconfirmed reservations are reported, not successful fills.",
            "replica_requests": deltas, "budget": budget})
    finally:
        api(f"/v1/campaigns/{campaign}/pause", "POST")


def totals(campaign):
    now = datetime.datetime.now(datetime.timezone.utc)
    params = urllib.parse.urlencode({"from": (now - datetime.timedelta(hours=1)).isoformat(),
                                    "to": (now + datetime.timedelta(hours=1)).isoformat(), "granularity": "total"})
    return request(REPORTING + f"/v1/reports/campaigns/{campaign}?{params}")[1]["totals"]


def duplicate():
    campaign, placement = fixture(1)
    try:
        status, response, _ = bid(placement, 0)
        if status != 200 or response["campaign_id"] != campaign:
            raise RuntimeError("Failed to win isolated replay fixture")
        before = totals(campaign)
        if before["impressions"] != 0 or before["clicks"] != 0:
            raise RuntimeError("Replay fixture was not empty")
        compose = ["docker", "compose", "--env-file", os.environ.get("COMPOSE_ENV_FILE", ".env"),
                   "-f", "deploy/docker-compose.yml"]
        subprocess.run(compose + ["stop", "event-aggregator"], cwd=ROOT, check=True)
        try:
            for _ in range(3):
                if request(response["impression_url"])[0] != 200:
                    raise RuntimeError("Impression publish failed")
                if request(response["click_url"])[0] != 302:
                    raise RuntimeError("Click publish failed")
        finally:
            subprocess.run(compose + ["start", "event-aggregator"], cwd=ROOT, check=True)
        deadline = time.monotonic() + 30
        final = None
        while time.monotonic() < deadline:
            try:
                final = totals(campaign)
                if final["impressions"] > 1 or final["clicks"] > 1:
                    raise RuntimeError(f"Double counting: {final}")
                if final["impressions"] == final["clicks"] == 1:
                    break
            except (urllib.error.URLError, ConnectionError):
                pass
            time.sleep(0.25)
        if final is None or final["impressions"] != 1 or final["clicks"] != 1:
            raise RuntimeError(f"Replay did not converge: {final}")
        time.sleep(2)
        final = totals(campaign)
        spend = decimal.Decimal(str(final["spend"])) * 1_000_000
        if final["impressions"] != 1 or final["clicks"] != 1 or spend != 1010:
            raise RuntimeError(f"Incorrect replay totals: {final}")
        write("duplicate-proof.json", {"pass": True, "campaign_id": campaign,
            "impression_fires": 3, "click_fires": 3, "consumer_restarted_with_backlog": True,
            "totals": final, "note": "Exact crash-before/after-commit failpoints are covered by Go integration tests."})
    finally:
        api(f"/v1/campaigns/{campaign}/pause", "POST")


def budgets():
    cursor = None
    checked = []
    while True:
        page = api("/v1/campaigns?" + urllib.parse.urlencode({"limit": 200, **({"cursor": cursor} if cursor else {})}))
        for campaign in page["data"]:
            budget = api(f'/v1/campaigns/{campaign["id"]}/budget')
            if not 0 <= budget["spent_micros"] <= budget["daily_budget_micros"]:
                raise RuntimeError(f"Overspend: {budget}")
            checked.append(campaign["id"])
        cursor = page["next_cursor"]
        if not cursor:
            break
    if not checked:
        raise RuntimeError("No campaigns were available to verify")
    write("budget-scan.json", {"pass": True, "campaigns_checked": len(checked)})


def main():
    mode = sys.argv[1]
    if mode == "budgets":
        budgets()
        return
    bidder = os.environ.get("BIDDER_URL", "http://localhost:9101") + "/admin/config"
    original = request(bidder)[1]
    request(bidder, "POST", {"latency_ms": 0, "jitter_ms": 0, "error_rate": 0,
                             "timeout_rate": 0, "no_bid_rate": 0, "bid_mode": "fixed"})
    try:
        {"overspend": overspend, "duplicate": duplicate}[mode]()
    finally:
        request(bidder, "POST", original)


if __name__ == "__main__":
    main()
