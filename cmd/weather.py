#!/usr/bin/env python3
"""
获取天气预报和 AQI，wttr.in 免费接口
输出 JSON 格式给 Go 后端
"""

import json
import sys
import time
import urllib.request
import urllib.error

API_URL = "https://wttr.in/{}?format=j1"


def fetch_weather(city: str) -> dict:
    """获取指定城市的天气数据"""
    url = API_URL.format(urllib.request.quote(city))
    req = urllib.request.Request(url, headers={"User-Agent": "curl/8.0"})

    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            data = json.loads(resp.read().decode("utf-8"))
    except Exception as e:
        return {"code": 500, "error": f"请求失败: {e}"}

    return parse_weather(data, city)


def parse_weather(data: dict, city: str) -> dict:
    """解析 wttr.in 返回的 JSON"""
    try:
        current = data.get("current_condition", [{}])[0]
        forecast = data.get("weather", [])
        nearest_area = data.get("nearest_area", [{}])[0]
        area_name = nearest_area.get("areaName", [{}])[0].get("value", city)

        temp_current = float(current.get("temp_C", 0))
        humidity = current.get("humidity", "")
        desc = current.get("weatherDesc", [{}])[0].get("value", "")
        wind = current.get("windspeedKmph", "")
        uv = current.get("uvIndex", 0)

        # AQI 估算（wttr.in 不直接提供 AQI，用 visibility 估算）
        visibility = current.get("visibility", "")
        aqi = _estimate_aqi(visibility)

        today = forecast[0] if forecast else {}
        temp_high = float(today.get("maxtempC", temp_current))
        temp_low = float(today.get("mintempC", temp_current))
        week_day = today.get("date", "")
        # 转为中文星期
        week_days = ["周一", "周二", "周三", "周四", "周五", "周六", "周日"]
        try:
            t = time.strptime(week_day, "%Y-%m-%d")
            week_day = week_days[t.tm_wday]
        except (ValueError, IndexError):
            week_day = ""

        return {
            "code": 200,
            "data": {
                "city": area_name,
                "date": time.strftime("%Y-%m-%d"),
                "week_day": week_day,
                "temp_current": temp_current,
                "temp_high": temp_high,
                "temp_low": temp_low,
                "aqi": aqi,
                "condition": desc,
                "humidity": humidity,
                "wind": wind,
                "uv": uv,
            },
        }
    except (KeyError, IndexError, ValueError) as e:
        return {"code": 500, "error": f"解析失败: {e}"}


def _estimate_aqi(visibility: str) -> int:
    """根据能见度(km)估算 AQI"""
    try:
        v = float(visibility)
    except (ValueError, TypeError):
        return 0
    if v >= 20:
        return 30
    elif v >= 10:
        return 60
    elif v >= 5:
        return 100
    elif v >= 2:
        return 150
    else:
        return 200


def main():
    city = "杭州"
    if len(sys.argv) > 1 and sys.argv[1] == "--city" and len(sys.argv) > 2:
        city = sys.argv[2]

    result = fetch_weather(city)
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
