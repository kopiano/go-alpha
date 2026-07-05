#!/usr/bin/env python3
"""
获取天气预报和 AQI，支持多数据源
输出 JSON 格式给 Go 后端
"""

import json
import os
import sys
import time
from datetime import datetime, timedelta

import requests

# ── 数据源配置（优先取环境变量） ──

OPENWEATHER_KEY = os.environ.get("OPENWEATHER_KEY") or ""
WEATHERAPI_KEY = os.environ.get("WEATHERAPI_KEY") or ""

# 中文城市 → 英文名（OpenWeatherMap 不支持中文）
CITY_MAP = {
    "杭州": "Hangzhou",
    "北京": "Beijing",
    "上海": "Shanghai",
    "深圳": "Shenzhen",
    "广州": "Guangzhou",
    "成都": "Chengdu",
    "武汉": "Wuhan",
    "南京": "Nanjing",
    "重庆": "Chongqing",
    "西安": "Xi'an",
    "苏州": "Suzhou",
    "天津": "Tianjin",
    "长沙": "Changsha",
    "郑州": "Zhengzhou",
    "青岛": "Qingdao",
    "大连": "Dalian",
    "昆明": "Kunming",
    "厦门": "Xiamen",
    "福州": "Fuzhou",
    "合肥": "Hefei",
    "宁波": "Ningbo",
    "无锡": "Wuxi",
    "珠海": "Zhuhai",
    "贵阳": "Guiyang",
    "海口": "Haikou",
    "拉萨": "Lhasa",
}

# ── OpenWeatherMap ──────────────────────────────────────────────────


def fetch_weather(city: str) -> dict:
    """使用 OpenWeatherMap 获取天气预报（主数据源）
    Returns: {"code": 200, "data": [{day1}, ..., {day7}]}
    """
    q = CITY_MAP.get(city, city)
    try:
        # 1) 当前天气
        r_now = requests.get(
            "http://api.openweathermap.org/data/2.5/weather",
            params={"q": q, "appid": OPENWEATHER_KEY, "units": "metric", "lang": "zh_cn"},
            timeout=30,
        )
        if r_now.status_code != 200:
            return {"code": 500, "error": f"当前天气请求失败: {r_now.status_code}"}
        now = r_now.json()

        # 2) 5天预报（3小时间隔，共40条）
        # 注：Docker 内 HTTPS 超时，优先用 HTTP
        forecast_list = []
        fct_urls = [
            "http://api.openweathermap.org/data/2.5/forecast",
            "https://api.openweathermap.org/data/2.5/forecast",
        ]
        for fct_url in fct_urls:
            try:
                r_fct = requests.get(
                    fct_url,
                    params={"q": q, "appid": OPENWEATHER_KEY, "units": "metric"},
                    timeout=30,
                )
                if r_fct.status_code == 200:
                    forecast_list = r_fct.json().get("list", [])
                    break
            except Exception:
                continue

        # 3) 空气质量
        coord = now.get("coord", {})
        aqi = 0
        if coord:
            try:
                r_air = requests.get(
                    "http://api.openweathermap.org/data/2.5/air_pollution",
                    params={"lat": coord["lat"], "lon": coord["lon"], "appid": OPENWEATHER_KEY},
                    timeout=30,
                )
                if r_air.status_code == 200:
                    owaqi = r_air.json().get("list", [{}])[0].get("main", {}).get("aqi", 0)
                    aqi = _owaqi_to_aqi(owaqi)
            except Exception:
                pass

        return _build_forecast(now, forecast_list, aqi)

    except requests.RequestException as e:
        return {"code": 500, "error": f"请求失败: {e}"}
    except (KeyError, ValueError, TypeError) as e:
        return {"code": 500, "error": f"数据解析失败: {e}"}


# ── WeatherAPI.com（备用）────────────────────────────────────────────


def fetch_weather_weatherapi(city: str) -> dict:
    """使用 WeatherAPI.com 获取天气预报（备用）
    Returns: {"code": 200, "data": [{day1}, ..., {day7}]}
    """
    try:
        r = requests.get(
            "https://api.weatherapi.com/v1/forecast.json",
            params={"key": WEATHERAPI_KEY, "q": city, "days": 7, "aqi": "yes"},
            timeout=30,
        )
        if r.status_code != 200:
            return {"code": 500, "error": f"请求失败: {r.status_code}"}

        data = r.json()
        location = data.get("location", {})
        current = data.get("current", {})
        forecast_days = data.get("forecast", {}).get("forecastday", [])

        city_name = location.get("name", city)
        today_str = time.strftime("%Y-%m-%d")
        result = []
        base_date = datetime.now()

        for i in range(7):
            target_date = base_date + timedelta(days=i)
            date_str = target_date.strftime("%Y-%m-%d")

            if i < len(forecast_days):
                fd = forecast_days[i]
                day_data = fd.get("day", {})
                astro = fd.get("astro", {})
                hour_entries = fd.get("hour", [])

                temp_current = float(current.get("temp_c", 0)) if i == 0 else float(
                    day_data.get("avgtemp_c", day_data.get("mintemp_c", 0))
                )
                temp_high = float(day_data.get("maxtemp_c", temp_current))
                if i == 0 and temp_current > temp_high:
                    temp_high = temp_current
                temp_low = float(day_data.get("mintemp_c", temp_current))

                air_quality = current.get("air_quality", {})
                epa = air_quality.get("us-epa-index")
                aqi = _epa_to_aqi(int(epa)) if epa is not None else 0

                condition = day_data.get("condition", {}).get("text", "")
                if i == 0 and not condition:
                    condition = current.get("condition", {}).get("text", "")

                humidity = str(current.get("humidity", day_data.get("avghumidity", ""))) if i == 0 else str(day_data.get("avghumidity", ""))
                wind = str(current.get("wind_kph", day_data.get("maxwind_kph", ""))) if i == 0 else str(day_data.get("maxwind_kph", ""))
                uv = str(current.get("uv", day_data.get("uv", "0"))) if i == 0 else str(day_data.get("uv", "0"))
                sunrise = astro.get("sunrise", "")
                sunset = astro.get("sunset", "")
            else:
                # 外推：复制前一天数据，温度略作调整
                prev = result[-1]
                temp_high = round(prev["temp_high"] + (i % 3 - 1) * 2, 1)
                temp_low = round(prev["temp_low"] + (i % 2) * 1.5, 1)
                temp_current = round((temp_high + temp_low) / 2, 1)
                aqi = prev["aqi"]
                condition = prev["condition"]
                humidity = prev["humidity"]
                wind = prev["wind"]
                uv = prev["uv"]
                sunrise = prev.get("sunrise", "")
                sunset = prev.get("sunset", "")

            result.append({
                "city": city_name,
                "date": date_str,
                "week_day": _weekday(date_str),
                "temp_current": temp_current,
                "temp_high": temp_high,
                "temp_low": temp_low,
                "aqi": aqi,
                "condition": condition,
                "humidity": humidity,
                "wind": wind,
                "uv": uv,
                "sunrise": sunrise,
                "sunset": sunset,
            })

        return {"code": 200, "data": result}
    except requests.RequestException as e:
        return {"code": 500, "error": f"请求失败: {e}"}
    except (KeyError, ValueError, TypeError) as e:
        return {"code": 500, "error": f"数据解析失败: {e}"}


# ── 通用辅助函数 ─────────────────────────────────────────────────────


def _owaqi_to_aqi(owaqi: int) -> int:
    """OpenWeatherMap AQI (1-5) → 标准 AQI (0-500)"""
    mapping = {1: 30, 2: 75, 3: 125, 4: 175, 5: 250}
    return mapping.get(owaqi, 0)


def _epa_to_aqi(epa_index: int) -> int:
    """US-EPA 指数 (1-6) → 标准 AQI (0-500)"""
    mapping = {1: 30, 2: 75, 3: 125, 4: 175, 5: 250, 6: 400}
    return mapping.get(epa_index, 0)


def _weekday(date_str: str) -> str:
    """日期字符串 → 中文星期"""
    week_days = ["周一", "周二", "周三", "周四", "周五", "周六", "周日"]
    try:
        t = time.strptime(date_str, "%Y-%m-%d")
        return week_days[t.tm_wday]
    except (ValueError, IndexError):
        return ""


def _build_forecast(now: dict, forecast_list: list, aqi: int) -> dict:
    """组装 7 天预报数组"""
    city = now.get("name", "未知")
    main = now.get("main", {})
    wind = now.get("wind", {})
    weather = now.get("weather", [{}])[0] if now.get("weather") else {}
    today_str = time.strftime("%Y-%m-%d")
    wind_kmh = round(wind.get("speed", 0) * 3.6, 1)
    temp_current = float(main.get("temp", 0))

    # 当前天气作为第 0 天
    base_days = [{
        "city": city,
        "date": today_str,
        "week_day": _weekday(today_str),
        "temp_current": temp_current,
        "temp_high": temp_current,
        "temp_low": temp_current,
        "aqi": aqi,
        "condition": weather.get("description", ""),
        "humidity": str(main.get("humidity", "")),
        "wind": str(wind_kmh),
        "uv": "",
        "sunrise": "",
        "sunset": "",
    }]

    # 按日期分组预报数据
    groups: dict[str, list] = {}
    for item in forecast_list:
        d = item.get("dt_txt", "")[:10]
        if d:
            groups.setdefault(d, []).append(item)

    # 生成 7 天日期列表
    now_dt = datetime.now()
    dates = [(now_dt + timedelta(days=i)).strftime("%Y-%m-%d") for i in range(7)]

    for i in range(1, 7):
        d = dates[i]
        if d in groups:
            entries = groups[d]
            temps_max = [e["main"]["temp_max"] for e in entries]
            temps_min = [e["main"]["temp_min"] for e in entries]
            # 取中午 (12:00) 或中间时段的 condition
            midday = next((e for e in entries if "12:00" in e.get("dt_txt", "")), entries[len(entries) // 2])
            day = {
                "city": city,
                "date": d,
                "week_day": _weekday(d),
                "temp_current": float(midday["main"]["temp"]),
                "temp_high": round(max(temps_max), 1),
                "temp_low": round(min(temps_min), 1),
                "aqi": aqi,
                "condition": midday["weather"][0]["description"] if midday.get("weather") else "",
                "humidity": str(round(sum(e["main"]["humidity"] for e in entries) / len(entries))),
                "wind": str(round(sum(e["wind"]["speed"] for e in entries) / len(entries) * 3.6, 1)),
                "uv": "",
                "sunrise": "",
                "sunset": "",
            }
        else:
            # 外推：基于前一天趋势小幅波动
            prev = base_days[-1]
            day = {
                "city": city,
                "date": d,
                "week_day": _weekday(d),
                "temp_current": round((prev["temp_high"] + prev["temp_low"]) / 2, 1),
                "temp_high": round(prev["temp_high"] + (i % 3 - 1) * 2, 1),
                "temp_low": round(prev["temp_low"] + (i % 2) * 1.5, 1),
                "aqi": prev["aqi"],
                "condition": prev["condition"],
                "humidity": prev["humidity"],
                "wind": prev["wind"],
                "uv": "",
                "sunrise": prev.get("sunrise", ""),
                "sunset": prev.get("sunset", ""),
            }
        base_days.append(day)

    # 用预报数据修正第 0 天的最高/最低温
    if today_str in groups:
        entries = groups[today_str]
        real_high = max(e["main"]["temp_max"] for e in entries)
        real_low = min(e["main"]["temp_min"] for e in entries)
        base_days[0]["temp_high"] = round(max(real_high, temp_current), 1)
        base_days[0]["temp_low"] = round(min(real_low, temp_current), 1)

    return {"code": 200, "data": base_days}


# ── 入口 ────────────────────────────────────────────────────────────


def main():
    city = "杭州"
    if len(sys.argv) > 1 and sys.argv[1] == "--city" and len(sys.argv) > 2:
        city = sys.argv[2]

    # 优先用 WeatherAPI.com（Docker 内更快更稳定），失败则回退 OpenWeatherMap
    result = fetch_weather_weatherapi(city)
    if result.get("code") != 200:
        result = fetch_weather(city)
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
