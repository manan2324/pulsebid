import json

with open("postman/PulseBid.postman_collection.json", "r", encoding="utf-8") as f:
    col = json.load(f)

for item in col.get("item", []):
    if item.get("name") == "08 Events":
        folder8_items = item.get("item", [])
        for req in folder8_items:
            if "08 Second click" in req.get("name", ""):
                req["request"]["url"] = "{{gateway_url}}/v1/events/click?t={{last_event_token}}"
            if "09 Poll report" in req.get("name", ""):
                # Ensure the url and test script are accurate
                req["event"] = [
                    {
                        "listen": "prerequest",
                        "script": {
                            "exec": [
                                "const tries = Number(pm.collectionVariables.get(\"poll_tries\") || 0);",
                                "if (tries > 0) {",
                                "  setTimeout(() => {}, 250);",
                                "}"
                            ],
                            "type": "text/javascript"
                        }
                    },
                    {
                        "listen": "test",
                        "script": {
                            "exec": [
                                "pm.test(\"HTTP 200 OK\", () => pm.response.to.have.status(200));",
                                "const expected = 1, maxTries = 20;",
                                "const tries = Number(pm.collectionVariables.get(\"poll_tries\") || 0);",
                                "const res = pm.response.json();",
                                "const imps = res.totals?.impressions ?? 0;",
                                "const clicks = res.totals?.clicks ?? 0;",
                                "if (imps !== expected && tries < maxTries) {",
                                "  pm.collectionVariables.set(\"poll_tries\", tries + 1);",
                                "  postman.setNextRequest(pm.info.requestName);",
                                "} else {",
                                "  pm.collectionVariables.unset(\"poll_tries\");",
                                "  pm.test(`impressions == ${expected}`, () => pm.expect(imps).to.eql(expected));",
                                "  pm.test(\"clicks >= 1\", () => pm.expect(clicks).to.be.at.least(1));",
                                "}"
                            ],
                            "type": "text/javascript"
                        }
                    }
                ]
    if item.get("name") == "09 Reporting":
        for req in item.get("item", []):
            if "04 Missing API key" in req.get("name", ""):
                req["event"] = [
                    {
                        "listen": "prerequest",
                        "script": {
                            "exec": [
                                "pm.request.headers.remove(\"X-API-Key\");"
                            ],
                            "type": "text/javascript"
                        }
                    },
                    {
                        "listen": "test",
                        "script": {
                            "exec": [
                                "pm.test(\"HTTP 401\", () => pm.response.to.have.status(401));"
                            ],
                            "type": "text/javascript"
                        }
                    }
                ]

with open("postman/PulseBid.postman_collection.json", "w", encoding="utf-8") as f:
    json.dump(col, f, indent=2)

print("Updated postman collection with prerequest removal and gateway_url for click.")
