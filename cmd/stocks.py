#!/usr/bin/env python3
import argparse
import json
import time
from datetime import datetime, timezone
from zoneinfo import ZoneInfo

import akshare as ak
import pandas as pd

NY_TZ = ZoneInfo("America/New_York")

SYMBOL_MAP = {
    "SPCX": "SPCX",
    "AAPL": "AAPL",
    "NVDA": "NVDA",
    "TSLA": "TSLA",
}

NAME_MAP = {
    "SPCX": "Space Exploration Technologies",
    "AAPL": "Apple Inc.",
    "NVDA": "NVIDIA Corporation",
    "TSLA": "Tesla, Inc.",
}


def fmt_time(value):
    if value is None or value == "":
        return ""
    if isinstance(value, str):
        try:
            dt = pd.to_datetime(value)
        except Exception:
            return value
    else:
        dt = pd.to_datetime(value)
    if pd.isna(dt):
        return ""
    if getattr(dt, "tzinfo", None) is None:
        dt = dt.tz_localize("UTC")
    dt = dt.tz_convert(NY_TZ)
    offset_hours = int(dt.utcoffset().total_seconds() / 3600)
    return dt.strftime("%-I:%M:%S %p ") + f"GMT{offset_hours:+d}"


def fmt_label(dt):
    local_dt = pd.to_datetime(dt)
    if getattr(local_dt, "tzinfo", None) is None:
        local_dt = local_dt.tz_localize("UTC")
    local_dt = local_dt.tz_convert(NY_TZ)
    return local_dt.strftime("%b %d %I:%M %p")


def range_to_period(range_key):
    key = range_key.lower()
    if key == "5d":
        return "5d"
    if key == "1mo":
        return "1mo"
    if key == "6mo":
        return "6mo"
    return "1y"


def get_us_code(symbol: str) -> str:
    try:
        spot = ak.stock_us_spot_em()
        matched = spot[spot["代码"].astype(str).str.upper().str.endswith(f".{symbol.upper()}")]
        if not matched.empty:
            return str(matched.iloc[0]["代码"])
    except Exception:
        pass
    return f"105.{symbol.upper()}"


def fetch_daily(symbol: str) -> pd.DataFrame:
    df = ak.stock_us_daily(symbol=symbol, adjust="")
    if df is None or df.empty:
        return pd.DataFrame()
    df = df.copy()
    if "date" not in df.columns:
        return pd.DataFrame()
    df["date"] = pd.to_datetime(df["date"], errors="coerce")
    df = df.dropna(subset=["date"])
    return df.sort_values("date").reset_index(drop=True)


def fetch_intraday(symbol_code: str) -> pd.DataFrame:
    last_err = None
    for attempt in range(3):
        try:
            df = ak.stock_us_hist_min_em(symbol=symbol_code)
            if df is None or df.empty or "时间" not in df.columns:
                return pd.DataFrame()
            df = df.copy()
            df["时间"] = pd.to_datetime(df["时间"], errors="coerce")
            df = df.dropna(subset=["时间"]).sort_values("时间").reset_index(drop=True)
            return df
        except Exception as exc:
            last_err = exc
            time.sleep(0.8 * (attempt + 1))
    if last_err:
        return pd.DataFrame()
    return pd.DataFrame()


def aggregate_to_10m(df: pd.DataFrame) -> pd.DataFrame:
    if df.empty:
        return df
    work = df.copy()
    work["bucket"] = work["时间"].dt.floor("10min")
    rows = []
    for bucket, group in work.groupby("bucket", sort=True):
        rows.append(
            {
                "time": bucket,
                "open": float(group.iloc[0]["开盘"]),
                "high": float(group["最高"].max()),
                "low": float(group["最低"].min()),
                "close": float(group.iloc[-1]["收盘"]),
                "volume": float(group["成交量"].sum()),
            }
        )
    return pd.DataFrame(rows)


def build_points(period: str, symbol: str) -> list[dict]:
    if period == "5d":
        code = get_us_code(symbol)
        try:
            intraday = fetch_intraday(code)
            if not intraday.empty:
                intraday = aggregate_to_10m(intraday)
                points = []
                for _, row in intraday.iterrows():
                    points.append(
                        {
                            "t": int(pd.Timestamp(row["time"]).timestamp() * 1000),
                            "label": fmt_label(row["time"]),
                            "open": float(row["open"]),
                            "high": float(row["high"]),
                            "low": float(row["low"]),
                            "close": float(row["close"]),
                            "volume": int(row["volume"] or 0),
                        }
                    )
                if points:
                    return points
        except Exception:
            pass

        daily = fetch_daily(symbol)
        if daily.empty:
            return []
        tail = daily.tail(5).reset_index(drop=True)
        points = []
        for _, row in tail.iterrows():
            points.append(
                {
                    "t": int(pd.Timestamp(row["date"]).timestamp() * 1000),
                    "label": pd.Timestamp(row["date"]).strftime("%b %d"),
                    "open": float(row["open"]),
                    "high": float(row["high"]),
                    "low": float(row["low"]),
                    "close": float(row["close"]),
                    "volume": int(row.get("volume", 0) or 0),
                }
            )
        return points

    daily = fetch_daily(symbol)
    if daily.empty:
        return []
    if period == "1mo":
        tail = daily.tail(22)
    elif period == "6mo":
        tail = daily.tail(126)
    else:
        tail = daily.tail(252)
    points = []
    for _, row in tail.iterrows():
        points.append(
            {
                "t": int(pd.Timestamp(row["date"]).timestamp() * 1000),
                "label": pd.Timestamp(row["date"]).strftime("%b %d"),
                "open": float(row["open"]),
                "high": float(row["high"]),
                "low": float(row["low"]),
                "close": float(row["close"]),
                "volume": int(row.get("volume", 0) or 0),
            }
        )
    return points


def fetch_quotes(symbol: str, daily: pd.DataFrame) -> tuple[dict, dict, str, str]:
    regular = {"price": None, "change": None, "change_percent": None, "time": "", "raw_time": None}
    pre_market = {"price": None, "change": None, "change_percent": None, "time": "", "raw_time": None}
    exchange = "NASDAQ"
    currency = "USD"

    try:
        spot = ak.stock_us_spot_em()
        row = spot[spot["代码"].astype(str).str.upper().str.endswith(f".{symbol.upper()}")]
        if not row.empty:
            item = row.iloc[0]
            exchange = "NASDAQ"
            currency = "USD"
            last = float(item.get("最新价") or regular["price"] or 0)
            prev = float(item.get("昨收价") or (daily.iloc[-2]["close"] if len(daily) > 1 else last))
            change = float(item.get("涨跌额") or (last - prev))
            pct = float(item.get("涨跌幅") or ((change / prev) * 100 if prev else 0))
            regular.update({"price": last, "change": change, "change_percent": pct})
    except Exception:
        pass

    if regular["price"] is None and not daily.empty:
        last = float(daily.iloc[-1]["close"])
        prev = float(daily.iloc[-2]["close"]) if len(daily) > 1 else last
        change = last - prev
        pct = (change / prev) * 100 if prev else 0
        regular.update(
            {
                "price": last,
                "change": change,
                "change_percent": pct,
                "time": fmt_time(datetime.now(timezone.utc)),
                "raw_time": int(datetime.now(timezone.utc).timestamp()),
            }
        )
        pre_market.update(regular)
    elif pre_market["price"] is None and regular["price"] is not None:
        pre_market.update(
            {
                "price": regular["price"],
                "change": regular["change"],
                "change_percent": regular["change_percent"],
                "time": regular["time"],
                "raw_time": regular["raw_time"],
            }
        )

    return regular, pre_market, exchange, currency


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--symbol", default="SPCX")
    parser.add_argument("--range", dest="range_key", default="5d")
    args = parser.parse_args()

    symbol = args.symbol.upper().strip()
    if symbol not in SYMBOL_MAP:
        symbol = "SPCX"

    period = range_to_period(args.range_key)
    daily = fetch_daily(symbol)
    points = build_points(period, symbol)
    regular, pre_market, exchange, currency = fetch_quotes(symbol, daily)

    data = {
        "symbol": symbol,
        "name": NAME_MAP.get(symbol, symbol),
        "exchange": exchange,
        "currency": currency,
        "market_state": "REGULAR",
        "regular": regular,
        "pre_market": pre_market,
        "points": points,
        "range": period,
        "interval": "10m" if period == "5d" else "1d",
    }
    print(json.dumps({"code": 200, "message": "ok", "data": data}, ensure_ascii=False))


if __name__ == "__main__":
    main()
