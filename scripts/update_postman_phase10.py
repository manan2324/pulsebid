import json

collection_path = "postman/PulseBid.postman_collection.json"

with open(collection_path, "r", encoding="utf-8") as f:
    col = json.load(f)

# Find Folder 10
folder_idx = None
for i, item in enumerate(col["item"]):
    if "10" in item["name"]:
        folder_idx = i
        break

if folder_idx is None:
    raise ValueError("Folder 10 Resilience not found in collection")

f10_items = [
    {
        "name": "01 Bidder 1 tune latency 500ms",
        "event": [
            {
                "listen": "test",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'pm.test("Status is 200", () => pm.response.to.have.status(200));',
                        'pm.test("Latency set to 500ms", () => pm.expect(pm.response.json().latency_ms).to.eql(500));'
                    ]
                }
            }
        ],
        "request": {
            "method": "POST",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": {
                "mode": "raw",
                "raw": '{\n  "latency_ms": 500\n}'
            },
            "url": "http://{{bidder_host}}:9101/admin/config"
        }
    },
    {
        "name": "02 Slow bidder isolated within budget",
        "event": [
            {
                "listen": "test",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'pm.test("Status is 200 filled", () => pm.response.to.have.status(200));',
                        'pm.test("Latency under budget", () => {',
                        '  pm.expect(pm.response.responseTime).to.be.below(Number(pm.environment.get("latency_budget_ms") || 120) + 30);',
                        '});',
                        'pm.test("Timed out bidder recorded in debug header", () => {',
                        '  const debug = pm.response.headers.get("X-PulseBid-Debug") || "";',
                        '  pm.expect(debug).to.include("timed_out=1");',
                        '});',
                        'pm.test("Healthy bidder won", () => {',
                        '  const b = pm.response.json();',
                        '  pm.expect(b.campaign_id).to.eql(pm.collectionVariables.get("camp_b_id"));',
                        '});'
                    ]
                }
            }
        ],
        "request": {
            "method": "POST",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": {
                "mode": "raw",
                "raw": '{\n  "id": "req-resilience-slow-{{run_id}}",\n  "imp": {\n    "id": "imp-slow",\n    "placement_id": "pm-auction",\n    "w": 300,\n    "h": 250,\n    "bidfloor": 0.50\n  },\n  "device": {\n    "type": "mobile"\n  },\n  "geo": {\n    "country": "IN"\n  }\n}'
            },
            "url": "{{gateway_url}}/v1/bid"
        }
    },
    {
        "name": "03 Reset bidder 1 latency",
        "event": [
            {
                "listen": "test",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'pm.test("Status is 200", () => pm.response.to.have.status(200));',
                        'pm.test("Latency reset to 0", () => pm.expect(pm.response.json().latency_ms).to.eql(0));'
                    ]
                }
            }
        ],
        "request": {
            "method": "POST",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": {
                "mode": "raw",
                "raw": '{\n  "latency_ms": 0\n}'
            },
            "url": "http://{{bidder_host}}:9101/admin/config"
        }
    },
    {
        "name": "04 Bidder 1 tune error rate 1.0",
        "event": [
            {
                "listen": "test",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'pm.test("Status is 200", () => pm.response.to.have.status(200));',
                        'pm.test("Error rate set to 1", () => pm.expect(pm.response.json().error_rate).to.eql(1));'
                    ]
                }
            }
        ],
        "request": {
            "method": "POST",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": {
                "mode": "raw",
                "raw": '{\n  "error_rate": 1\n}'
            },
            "url": "http://{{bidder_host}}:9101/admin/config"
        }
    },
    {
        "name": "05 Error bidder isolated",
        "event": [
            {
                "listen": "test",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'pm.test("Status is 200 filled", () => pm.response.to.have.status(200));',
                        'pm.test("Errors recorded in debug header", () => {',
                        '  const debug = pm.response.headers.get("X-PulseBid-Debug") || "";',
                        '  pm.expect(debug).to.include("errors=1");',
                        '});',
                        'pm.test("Healthy bidder won", () => {',
                        '  const b = pm.response.json();',
                        '  pm.expect(b.campaign_id).to.eql(pm.collectionVariables.get("camp_b_id"));',
                        '});'
                    ]
                }
            }
        ],
        "request": {
            "method": "POST",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": {
                "mode": "raw",
                "raw": '{\n  "id": "req-resilience-err-{{run_id}}",\n  "imp": {\n    "id": "imp-err",\n    "placement_id": "pm-auction",\n    "w": 300,\n    "h": 250,\n    "bidfloor": 0.50\n  },\n  "device": {\n    "type": "mobile"\n  },\n  "geo": {\n    "country": "IN"\n  }\n}'
            },
            "url": "{{gateway_url}}/v1/bid"
        }
    },
    {
        "name": "06 Bidder 2 tune error rate 1.0",
        "event": [
            {
                "listen": "test",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'pm.test("Status is 200", () => pm.response.to.have.status(200));',
                        'pm.test("Bidder 2 error rate set to 1", () => pm.expect(pm.response.json().error_rate).to.eql(1));'
                    ]
                }
            }
        ],
        "request": {
            "method": "POST",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": {
                "mode": "raw",
                "raw": '{\n  "error_rate": 1\n}'
            },
            "url": "http://{{bidder_host}}:9102/admin/config"
        }
    },
    {
        "name": "07 All bidders error returns 204 no_bids",
        "event": [
            {
                "listen": "test",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'pm.test("Status is 204 No Content", () => pm.response.to.have.status(204));',
                        'pm.test("Reason header is no_bids", () => {',
                        '  pm.expect(pm.response.headers.get("X-PulseBid-Reason")).to.eql("no_bids");',
                        '});',
                        'pm.test("Debug header shows 0 valid bids", () => {',
                        '  const debug = pm.response.headers.get("X-PulseBid-Debug") || "";',
                        '  pm.expect(debug).to.include("valid_bids=0");',
                        '});'
                    ]
                }
            }
        ],
        "request": {
            "method": "POST",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": {
                "mode": "raw",
                "raw": '{\n  "id": "req-resilience-all-err-{{run_id}}",\n  "imp": {\n    "id": "imp-all-err",\n    "placement_id": "pm-auction",\n    "w": 300,\n    "h": 250,\n    "bidfloor": 0.50\n  },\n  "device": {\n    "type": "mobile"\n  },\n  "geo": {\n    "country": "IN"\n  }\n}'
            },
            "url": "{{gateway_url}}/v1/bid"
        }
    },
    {
        "name": "08 Reset bidders 1 and 2 error rate",
        "event": [
            {
                "listen": "prerequest",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'pm.sendRequest({',
                        '  url: "http://" + pm.environment.get("bidder_host") + ":9102/admin/config",',
                        '  method: "POST",',
                        '  header: { "Content-Type": "application/json" },',
                        '  body: { mode: "raw", raw: JSON.stringify({ error_rate: 0 }) }',
                        '}, () => {});'
                    ]
                }
            },
            {
                "listen": "test",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'pm.test("Status is 200", () => pm.response.to.have.status(200));',
                        'pm.test("Bidder 1 error rate reset", () => pm.expect(pm.response.json().error_rate).to.eql(0));'
                    ]
                }
            }
        ],
        "request": {
            "method": "POST",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": {
                "mode": "raw",
                "raw": '{\n  "error_rate": 0\n}'
            },
            "url": "http://{{bidder_host}}:9101/admin/config"
        }
    },
    {
        "name": "09 Tune all bidders timeout rate 1.0",
        "event": [
            {
                "listen": "prerequest",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'pm.sendRequest({',
                        '  url: "http://" + pm.environment.get("bidder_host") + ":9102/admin/config",',
                        '  method: "POST",',
                        '  header: { "Content-Type": "application/json" },',
                        '  body: { mode: "raw", raw: JSON.stringify({ timeout_rate: 1 }) }',
                        '}, () => {});'
                    ]
                }
            },
            {
                "listen": "test",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'pm.test("Status is 200", () => pm.response.to.have.status(200));',
                        'pm.test("Bidder 1 timeout rate set to 1", () => pm.expect(pm.response.json().timeout_rate).to.eql(1));'
                    ]
                }
            }
        ],
        "request": {
            "method": "POST",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": {
                "mode": "raw",
                "raw": '{\n  "timeout_rate": 1\n}'
            },
            "url": "http://{{bidder_host}}:9101/admin/config"
        }
    },
    {
        "name": "10 All bidders timeout returns 204 within budget",
        "event": [
            {
                "listen": "test",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'pm.test("Status is 204 No Content", () => pm.response.to.have.status(204));',
                        'pm.test("Reason header is no_bids", () => {',
                        '  pm.expect(pm.response.headers.get("X-PulseBid-Reason")).to.eql("no_bids");',
                        '});',
                        'pm.test("Response time bounded by deadline", () => {',
                        '  pm.expect(pm.response.responseTime).to.be.below(160);',
                        '});'
                    ]
                }
            }
        ],
        "request": {
            "method": "POST",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": {
                "mode": "raw",
                "raw": '{\n  "id": "req-resilience-all-timeout-{{run_id}}",\n  "imp": {\n    "id": "imp-all-to",\n    "placement_id": "pm-auction",\n    "w": 300,\n    "h": 250,\n    "bidfloor": 0.50\n  },\n  "device": {\n    "type": "mobile"\n  },\n  "geo": {\n    "country": "IN"\n  }\n}'
            },
            "url": "{{gateway_url}}/v1/bid"
        }
    },
    {
        "name": "11 Reset all bidders timeout rate",
        "event": [
            {
                "listen": "prerequest",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'pm.sendRequest({',
                        '  url: "http://" + pm.environment.get("bidder_host") + ":9102/admin/config",',
                        '  method: "POST",',
                        '  header: { "Content-Type": "application/json" },',
                        '  body: { mode: "raw", raw: JSON.stringify({ timeout_rate: 0 }) }',
                        '}, () => {});'
                    ]
                }
            },
            {
                "listen": "test",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'pm.test("Status is 200", () => pm.response.to.have.status(200));',
                        'pm.test("Bidder 1 timeout rate reset", () => pm.expect(pm.response.json().timeout_rate).to.eql(0));'
                    ]
                }
            }
        ],
        "request": {
            "method": "POST",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": {
                "mode": "raw",
                "raw": '{\n  "timeout_rate": 0\n}'
            },
            "url": "http://{{bidder_host}}:9101/admin/config"
        }
    },
    {
        "name": "12 Repeated failing requests trip circuit breaker",
        "event": [
            {
                "listen": "prerequest",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'const loopCount = Number(pm.collectionVariables.get("cb_loop") || 0);',
                        'if (loopCount === 0) {',
                        '  pm.sendRequest({',
                        '    url: "http://" + pm.environment.get("bidder_host") + ":9101/admin/config",',
                        '    method: "POST",',
                        '    header: { "Content-Type": "application/json" },',
                        '    body: { mode: "raw", raw: JSON.stringify({ error_rate: 1 }) }',
                        '  }, () => {});',
                        '}'
                    ]
                }
            },
            {
                "listen": "test",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'const maxLoops = 22;',
                        'const loop = Number(pm.collectionVariables.get("cb_loop") || 0);',
                        'if (loop < maxLoops) {',
                        '  pm.collectionVariables.set("cb_loop", loop + 1);',
                        '  postman.setNextRequest(pm.info.requestName);',
                        '} else {',
                        '  pm.collectionVariables.unset("cb_loop");',
                        '  pm.test("Status is 200 filled from bidder 2", () => pm.response.to.have.status(200));',
                        '  pm.test("Fast response time with tripped circuit breaker", () => {',
                        '    pm.expect(pm.response.responseTime).to.be.below(120);',
                        '  });',
                        '}'
                    ]
                }
            }
        ],
        "request": {
            "method": "POST",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": {
                "mode": "raw",
                "raw": '{\n  "id": "req-resilience-cb-{{run_id}}",\n  "imp": {\n    "id": "imp-cb",\n    "placement_id": "pm-auction",\n    "w": 300,\n    "h": 250,\n    "bidfloor": 0.50\n  },\n  "device": {\n    "type": "mobile"\n  },\n  "geo": {\n    "country": "IN"\n  }\n}'
            },
            "url": "{{gateway_url}}/v1/bid"
        }
    },
    {
        "name": "13 Final cleanup reset all bidders to default",
        "event": [
            {
                "listen": "prerequest",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'const defaults = {',
                        '  latency_ms: 0,',
                        '  jitter_ms: 0,',
                        '  error_rate: 0,',
                        '  timeout_rate: 0,',
                        '  no_bid_rate: 0,',
                        '  bid_mode: "fixed"',
                        '};',
                        '[9102, 9103, 9104, 9105, 9106, 9107, 9108].forEach(port => {',
                        '  pm.sendRequest({',
                        '    url: "http://" + pm.environment.get("bidder_host") + ":" + port + "/admin/config",',
                        '    method: "POST",',
                        '    header: { "Content-Type": "application/json" },',
                        '    body: { mode: "raw", raw: JSON.stringify(defaults) }',
                        '  }, () => {});',
                        '});'
                    ]
                }
            },
            {
                "listen": "test",
                "script": {
                    "type": "text/javascript",
                    "exec": [
                        'pm.test("Status is 200", () => pm.response.to.have.status(200));',
                        'pm.test("Bidder 1 defaults restored", () => {',
                        '  const cfg = pm.response.json();',
                        '  pm.expect(cfg.latency_ms).to.eql(0);',
                        '  pm.expect(cfg.error_rate).to.eql(0);',
                        '  pm.expect(cfg.timeout_rate).to.eql(0);',
                        '});'
                    ]
                }
            }
        ],
        "request": {
            "method": "POST",
            "header": [{"key": "Content-Type", "value": "application/json"}],
            "body": {
                "mode": "raw",
                "raw": '{\n  "latency_ms": 0,\n  "jitter_ms": 0,\n  "error_rate": 0,\n  "timeout_rate": 0,\n  "no_bid_rate": 0,\n  "bid_mode": "fixed"\n}'
            },
            "url": "http://{{bidder_host}}:9101/admin/config"
        }
    }
]

col["item"][folder_idx]["item"] = f10_items

with open(collection_path, "w", encoding="utf-8") as f:
    json.dump(col, f, indent=2)

print("Updated Folder 10 Resilience with 13 comprehensive requests successfully!")
