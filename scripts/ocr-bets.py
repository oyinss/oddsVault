#!/usr/bin/env python3
"""Extract 1xBet bet-detail screenshots (OCR) into the journal import format."""
import glob
import hashlib
import json
import os
import re
import subprocess
import sys

DATE = re.compile(r"(\d{2})[./](\d{2})[./](\d{4})\s*\(\s*(\d{1,2})[:.](\d{2})\s*\)")
DATE_ONLY = re.compile(r"(\d{2})[./](\d{2})[./](\d{4})")
DATE_COMPACT = re.compile(r"(?<!\d)(\d{2})(\d{2})(\d{4})(?!\d)")
ODDS = re.compile(r"Odds:\s*([0-9][0-9\s.,]*)", re.I)
STAKE = re.compile(r"Bet:\s*([0-9][0-9\s.,]*)", re.I)
WIN = re.compile(r"Winnings:\s*([0-9][0-9\s.,]*)", re.I)
STATUS = re.compile(r"Status:\s*([A-Za-z][A-Za-z ]+)", re.I)
KIND = re.compile(r"\b(Single|Accumulator|Express|System|Trixie|Patent|Lucky)\b", re.I)
BET_ID = re.compile(r"(?<!\d)(\d{9,14})(?!\d)")
MARKET_WORDS = re.compile(
    r"(total|handicap|winner|both teams|over|under|corners|result|draw|correct score|"
    r"goals|W1|W2|X|individual|multi|1X2|odd/even|clean sheet)",
    re.I,
)
TRAILING_ODDS = re.compile(r"([0-9]+[.,][0-9]{1,3})\s*$")
CACHE = os.environ.get("OCR_CACHE", "/tmp/oddsvault-ocr-cache")


def clean_number(raw):
    if raw is None:
        return None
    value = raw.replace(" ", "").replace("\u00a0", "").replace(",", ".").strip(".")
    try:
        return float(value)
    except ValueError:
        return None


def ocr(path):
    os.makedirs(CACHE, exist_ok=True)
    cached = os.path.join(CACHE, os.path.basename(path) + ".txt")
    if os.path.exists(cached):
        return open(cached, encoding="utf-8", errors="ignore").read()
    text = subprocess.run(
        ["tesseract", path, "stdout", "-l", "eng", "--psm", "6"],
        capture_output=True, text=True,
    ).stdout
    open(cached, "w", encoding="utf-8").write(text)
    return text


def parse(path):
    text = ocr(path)
    lines = [line.strip() for line in text.splitlines() if line.strip()]
    if "Football." not in text:
        return None

    header, legs, leg = {}, [], None
    for line in lines:
        if "Football." in line:
            if leg:
                legs.append(leg)
            competition = line.split("Football.", 1)[1].strip(" .:-")
            leg = {"competition": competition, "date": None, "event": [], "market": None, "odds": None, "status": None}
            continue

        if leg is None:
            if "placed_at" not in header:
                match = DATE.search(line)
                if match:
                    d, m, y, hour, minute = match.groups()
                    header["placed_at"] = f"{d}/{m}/{y} / {int(hour):02d}:{minute}"
                    continue
                match = DATE_ONLY.search(line)
                if match:
                    d, m, y = match.groups()
                    header["placed_at"] = f"{d}/{m}/{y}"
                    continue
                match = DATE_COMPACT.search(line)
                if match:
                    d, m, y = match.groups()
                    header["placed_at"] = f"{d}/{m}/{y}"
                    continue
            if "bet_type" not in header:
                kind = KIND.search(line)
                if kind:
                    header["bet_type"] = kind.group(1).title()
                    continue
            if "bet_id" not in header:
                match = BET_ID.search(line)
                if match:
                    header["bet_id"] = match.group(1)
                    continue
            if "stake" not in header:
                match = STAKE.search(line)
                if match:
                    header["stake"] = clean_number(match.group(1))
                    continue
            if "odds" not in header:
                match = ODDS.search(line)
                if match:
                    header["odds"] = clean_number(match.group(1))
                    continue
            if "winnings" not in header:
                match = WIN.search(line)
                if match:
                    header["winnings"] = clean_number(match.group(1))
                    continue
            if "status" not in header:
                match = STATUS.search(line)
                if match:
                    header["status"] = match.group(1).strip().title()
                    continue
            continue

        if leg["date"] is None:
            match = DATE.search(line)
            if match:
                d, m, y, hour, minute = match.groups()
                leg["date"] = f"{d}/{m}/{y} / {int(hour):02d}:{minute}"
                continue
        if leg["status"] is None:
            match = STATUS.search(line)
            if match:
                leg["status"] = match.group(1).strip().title()
                continue
        if leg["market"] is None and MARKET_WORDS.search(line):
            leg["market"] = line
            trailing = TRAILING_ODDS.search(line)
            if trailing:
                value = clean_number(trailing.group(1))
                # A leg price above 50 is almost certainly a lost decimal point.
                while value is not None and value > 50:
                    value = value / 10
                leg["odds"] = value
            continue
        leg["event"].append(line)

    if leg:
        legs.append(leg)
    if "stake" not in header:
        return None
    header.setdefault("placed_at", "")

    stake = header["stake"]
    odds = header.get("odds")
    winnings = header.get("winnings")
    status_raw = (header.get("status") or "").lower()

    def divide_to_target(value, target):
        if value is None or target <= 0:
            return None
        for power in range(0, 5):
            candidate = value / (10 ** power)
            if abs(candidate - target) <= max(0.5, target * 0.005):
                return round(candidate, 2)
        return None

    # OCR often drops the dot in odds too ("1754" for 1.754).
    if stake and winnings and odds:
        implied_odds = winnings / stake
        for power in range(1, 4):
            candidate = odds / (10 ** power)
            if abs(candidate - implied_odds) <= max(0.01, implied_odds * 0.005):
                odds = round(candidate, 3)
                break

    if stake and odds and odds > 1.01 and winnings:
        target = stake * odds
        fixed_winnings = divide_to_target(winnings, target)
        fixed_stake = divide_to_target(stake, winnings / odds)
        if fixed_winnings is not None:
            winnings = fixed_winnings
        elif fixed_stake is not None:
            stake = fixed_stake
            winnings = round(stake * odds, 2)
        else:
            winnings = round(target, 2)
    elif stake and odds and odds > 1.01 and not winnings:
        winnings = round(stake * odds, 2)

    if not stake or stake < 10 or stake > 5_000_000:
        return None
    if not odds or odds < 1.0 or odds > 1000:
        return None

    if "lost" in status_raw or "losing" in status_raw:
        status = "Loss"
    elif "return" in status_raw:
        status = "Void"
    elif "paid" in status_raw or "win" in status_raw:
        if winnings is None:
            status = "Win"
        elif abs(winnings - stake) <= 0.01:
            status = "Void"
        elif winnings <= 0:
            status = "Loss"
        else:
            status = "Win"
    else:
        status = "Pending"

    fingerprint = hashlib.sha1(
        f"{header['placed_at']}|{stake:.2f}|{odds}|{winnings}".encode()
    ).hexdigest()[:12]

    return {
        "external_id": header.get("bet_id") or f"ocr:{fingerprint}",
        "placed_at": header["placed_at"],
        "bet_type": header.get("bet_type") or "Single",
        "stake": f"{stake:.2f}",
        "odds": f"{odds:.3f}" if odds else "",
        "potential_return": "" if status == "Loss" else (f"{winnings:.2f}" if winnings else f"{stake:.2f}"),
        "status_text": status,
        "legs": [
            {
                "competition": item["competition"],
                "raw_event": " | ".join(item["event"])[:300],
                "market_raw": item["market"],
                "odds_taken": f"{item['odds']:.3f}" if item["odds"] else "",
                "status": {"win": "won", "loss": "lost", "lost": "lost", "return": "void", "refund": "void"}.get(
                    (item["status"] or "").lower(), "unknown"
                ),
            }
            for item in legs
        ],
        "source_image": os.path.basename(path),
    }


def main():
    files = sorted(glob.glob(os.path.join(sys.argv[1], "*.jpg")))
    bets, seen = [], set()
    for path in files:
        bet = parse(path)
        if not bet:
            continue
        key = (bet["placed_at"], bet["stake"], bet["odds"], bet["potential_return"])
        if key in seen:
            continue
        seen.add(key)
        bets.append(bet)

    doc = {"bookmaker": "1xbet", "currency": "NGN", "source": "1xbet_screenshots_emife", "bets": bets}
    print(json.dumps(doc, indent=2))
    print(f"# {len(files)} images, {len(bets)} bets", file=sys.stderr)


if __name__ == "__main__":
    main()
