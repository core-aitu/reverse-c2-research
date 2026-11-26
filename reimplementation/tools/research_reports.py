#!/usr/bin/env python3
"""Offline validation, comparison and HTML rendering of synthetic telemetry audits."""
import argparse
from collections import Counter
from datetime import datetime
from html import escape
import json
from pathlib import Path
import re
import sys

MAX_BYTES = 32 * 1024 * 1024
MAX_EVENTS = 20_000
IDENTIFIER = re.compile(r"[A-Za-z0-9_-]{1,80}\Z")
STATES = {"unknown", "open", "closed"}
CODES = {"unexpected_open", "unexpected_close", "observation_outside_open_session"}
SESSION_FIELDS = {"session", "state", "firstEvent", "lastEvent", "observations", "validObservations"}


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"duplicate JSON field: {key}")
        result[key] = value
    return result


def read_json(path, limit=MAX_BYTES):
    with Path(path).open("rb") as stream:
        data = stream.read(limit + 1)
    if len(data) > limit:
        raise ValueError(f"JSON input exceeds {limit} bytes")
    return json.loads(data, object_pairs_hook=unique_object)


def fields(value, expected, label):
    if not isinstance(value, dict) or set(value) != expected:
        raise ValueError(f"invalid fields in {label}")


def identifier(value):
    if not isinstance(value, str) or not IDENTIFIER.fullmatch(value):
        raise ValueError("invalid identifier")
    return value


def count(value, limit=MAX_EVENTS):
    if type(value) is not int or not 0 <= value <= limit:
        raise ValueError("invalid count")
    return value


def timestamp(value):
    if not isinstance(value, str) or not re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})", value):
        raise ValueError("invalid RFC3339 timestamp")
    # Retain nanosecond precision for ordering; datetime stores microseconds.
    parts = re.fullmatch(r"(.{19})(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})", value)
    base, fractional, zone = parts.groups()
    return datetime.fromisoformat(base + zone.replace("Z", "+00:00")), int((fractional or "").ljust(9, "0"))


def validate(report):
    fields(report, {"schema", "eventCount", "sessions", "issues"}, "report")
    if report["schema"] != "lab-telemetry-audit/v1":
        raise ValueError("unsupported report schema")
    total = count(report["eventCount"])
    sessions, issues = report["sessions"], report["issues"]
    if not isinstance(sessions, list) or not isinstance(issues, list) or len(sessions) > total or len(issues) > total:
        raise ValueError("invalid session or issue array")
    ids = set()
    observations = 0
    minimum_events = 0
    invalid_observations = {}
    for item in sessions:
        fields(item, SESSION_FIELDS, "session")
        key = identifier(item["session"])
        if key in ids or item["state"] not in STATES:
            raise ValueError("duplicate session or invalid state")
        ids.add(key)
        if timestamp(item["firstEvent"]) > timestamp(item["lastEvent"]):
            raise ValueError("session timestamps reversed")
        seen = count(item["observations"], total)
        valid = count(item["validObservations"], seen)
        if item["state"] == "unknown" and valid:
            raise ValueError("unknown session contains valid observations")
        observations += seen
        minimum_events += max(1, seen + {"unknown": 0, "open": 1, "closed": 2}[item["state"]])
        invalid_observations[key] = seen - valid
    event_ids = set()
    invalid_counts = Counter()
    transition_issues = 0
    transition_counts = Counter()
    state_by_session = {s["session"]: s["state"] for s in sessions}
    for item in issues:
        fields(item, {"eventId", "session", "code"}, "issue")
        key, sid = identifier(item["eventId"]), identifier(item["session"])
        if key in event_ids or sid not in ids or item["code"] not in CODES:
            raise ValueError("duplicate issue or unknown session/code")
        event_ids.add(key)
        if item["code"] == "observation_outside_open_session":
            invalid_counts[sid] += 1
        else:
            if item["code"] == "unexpected_open" and state_by_session[sid] == "unknown":
                raise ValueError("unknown session cannot have an unexpected open")
            transition_counts[sid] += 1
            transition_issues += 1
    # Unknown sessions have no valid opening/closing transitions. Each must still
    # be supported by at least one observation or erroneous close.
    if any(s["state"] == "unknown" and not (s["observations"] or transition_counts[s["session"]]) for s in sessions):
        raise ValueError("unknown session has no events")
    known_transitions = sum({"unknown": 0, "open": 1, "closed": 2}[s["state"]] for s in sessions)
    if (observations + known_transitions + transition_issues != total or minimum_events > total
            or any(invalid_counts[k] != n for k, n in invalid_observations.items())):
        raise ValueError("report totals are inconsistent")
    return report


def summary(report):
    validate(report)
    states = Counter(s["state"] for s in report["sessions"])
    codes = Counter(i["code"] for i in report["issues"])
    return {
        "schema": "lab-report-summary/v1", "events": report["eventCount"],
        "sessions": len(report["sessions"]), "issues": len(report["issues"]),
        "observations": sum(s["observations"] for s in report["sessions"]),
        "validObservations": sum(s["validObservations"] for s in report["sessions"]),
        "states": {k: states[k] for k in sorted(STATES)},
        "issueCodes": {k: codes[k] for k in sorted(CODES)},
    }


def compare(before, after):
    old, new = summary(before), summary(after)
    a = {s["session"]: s for s in before["sessions"]}
    b = {s["session"]: s for s in after["sessions"]}
    def key(issue):
        return issue["eventId"], issue["session"], issue["code"]
    old_issues, new_issues = set(map(key, before["issues"])), set(map(key, after["issues"]))
    def records(keys):
        return [dict(zip(("eventId", "session", "code"), k)) for k in sorted(keys)]
    return {"schema": "lab-report-comparison/v1", "before": old, "after": new,
            "delta": {k: new[k] - old[k] for k in ("events", "sessions", "issues", "observations", "validObservations")},
            "addedSessions": sorted(b.keys() - a.keys()), "removedSessions": sorted(a.keys() - b.keys()),
            "changedSessions": [{"session": k, "before": a[k], "after": b[k]} for k in sorted(a.keys() & b.keys()) if a[k] != b[k]],
            "addedIssues": records(new_issues - old_issues), "removedIssues": records(old_issues - new_issues)}


def render_html(report):
    totals = summary(report)
    def table(headers, rows):
        return '<table><thead><tr>' + ''.join(f'<th scope="col">{escape(h)}</th>' for h in headers) + '</tr></thead><tbody>' + ''.join('<tr>' + ''.join(f'<td>{escape(str(v))}</td>' for v in row) + '</tr>' for row in rows) + '</tbody></table>'
    def metrics(items):
        return '<dl>' + ''.join(f'<dt>{escape(k)}</dt><dd>{escape(str(v))}</dd>' for k, v in items) + '</dl>'
    return ('''<!doctype html><html lang="en"><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'">
<title>Laboratory telemetry audit</title><style>
body{font:16px system-ui,sans-serif;max-width:1100px;margin:2rem auto;padding:0 1rem;color:#172333;background:#f6f8fb}
section{margin:2rem 0}dl{display:grid;grid-template-columns:1fr 1fr;max-width:420px;gap:.4rem}dd{margin:0;font-weight:600}table{border-collapse:collapse;width:100%;background:white}th,td{padding:.6rem;text-align:left;border:1px solid #ccd5df;overflow-wrap:anywhere}th{background:#e6edf5}.scroll{overflow:auto}small{color:#475569}</style>
<main><h1>Laboratory telemetry audit</h1><p>Offline synthetic data only. Not a live host or agent view.</p><section><h2>Summary</h2>'''
            + metrics((k, totals[k]) for k in ("events", "sessions", "issues", "observations", "validObservations"))
            + '<h3>States</h3>' + metrics(totals["states"].items())
            + '<h3>Issue codes</h3>' + metrics(totals["issueCodes"].items())
            + '</section><section><h2>Sessions</h2><div class="scroll">'
            + table(["Session", "State", "First event", "Last event", "Observations", "Valid"], ([s[k] for k in ("session", "state", "firstEvent", "lastEvent", "observations", "validObservations")] for s in report["sessions"]))
            + '</div></section><section><h2>Issues</h2><div class="scroll">'
            + table(["Event ID", "Session", "Code"], ([i[k] for k in ("eventId", "session", "code")] for i in report["issues"]))
            + '</div></section><small>lab-telemetry-audit/v1 — research reporting, not legacy compatibility evidence.</small></main></html>\n')


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    for command in ("summary", "html"):
        commands.add_parser(command).add_argument("report")
    diff = commands.add_parser("compare")
    diff.add_argument("before")
    diff.add_argument("after")
    args = parser.parse_args(argv)
    try:
        if args.command == "compare":
            result = compare(read_json(args.before), read_json(args.after))
        elif args.command == "html":
            print(render_html(read_json(args.report)), end="")
            return 0
        else:
            result = summary(read_json(args.report))
        print(json.dumps(result, indent=2, ensure_ascii=False))
        return 0
    except (OSError, ValueError, TypeError, OverflowError) as error:
        print(f"research-reports: {error}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
