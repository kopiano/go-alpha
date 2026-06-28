#!/usr/bin/env python3
"""
实时爬取微博热搜榜单
"""

import json
import sys
import time

import requests

HEADERS = {
    "User-Agent": (
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
        "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
    ),
    "Referer": "https://weibo.com/",
}


def fetch_hot_search() -> list[dict]:
    """爬取微博实时热搜榜单"""
    url = "https://weibo.com/ajax/side/hotSearch"
    resp = requests.get(url, headers=HEADERS, timeout=10)
    resp.raise_for_status()
    data = resp.json()

    realtime = data.get("data", {}).get("realtime", [])
    results = []
    for idx, item in enumerate(realtime, start=1):
        results.append({
            "rank": idx,
            "title": item.get("word", ""),
            "hot": item.get("raw_hot", 0),
            "label": item.get("label_name", ""),
            "category": item.get("category", ""),
            "url": f"https://weibo.com/weibo?q={item.get('word_scheme', item.get('word', ''))}",
        })
    return results


def print_hot_search(results: list[dict]) -> None:
    """格式化打印热搜榜单"""
    print(f"{'='*60}")
    print(f"  微博实时热搜榜  ({time.strftime('%Y-%m-%d %H:%M:%S')})")
    print(f"{'='*60}")
    for item in results:
        label = f" [{item['label']}]" if item["label"] else ""
        hot = f"🔥 {item['hot']}" if item["hot"] else ""
        print(f"  {item['rank']:>3}. {item['title']}{label} {hot}")


def main():
    try:
        results = fetch_hot_search()
        if "--json" in sys.argv:
            print(json.dumps({"code": 200, "data": results, "time": time.strftime("%Y-%m-%d %H:%M:%S")}, ensure_ascii=False))
        else:
            print_hot_search(results)
    except requests.RequestException as e:
        if "--json" in sys.argv:
            print(json.dumps({"code": 500, "error": f"请求失败: {e}"}, ensure_ascii=False))
        else:
            print(f"[ERROR] 请求失败: {e}")
    except (KeyError, json.JSONDecodeError) as e:
        if "--json" in sys.argv:
            print(json.dumps({"code": 500, "error": f"数据解析失败: {e}"}, ensure_ascii=False))
        else:
            print(f"[ERROR] 数据解析失败: {e}")


if __name__ == "__main__":
    main()
