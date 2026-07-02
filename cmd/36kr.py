#!/usr/bin/env python3
"""
爬取 36氪人气热榜，缓存到 data/ 目录
支持 --json 输出 JSON 格式
"""

import argparse
import json
import os
import re
import sys
import time

try:
    import cloudscraper
    HAS_CLOUDSCRAPER = True
except ImportError:
    HAS_CLOUDSCRAPER = False

import requests

DATA_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "data")

HEADERS = {
    "User-Agent": (
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
        "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
    ),
    "Referer": "https://36kr.com/",
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
}


def fetch_hot_list() -> list[dict]:
    """
    爬取 36氪人气热榜今日数据（第一页），
    通过前端 xhr/json 接口直取 JSON 数据。
    """
    today = time.strftime("%Y-%m-%d")
    url = f"https://36kr.com/hot-list/renqi/{today}/1"

    text = _fetch_page(url)
    if not text:
        return []

    # jina.ai 返回的是 Markdown，尝试从中提取链接和数据
    if "jina.ai" in text[:200] or text.startswith("Title:"):
        results = _parse_markdown(text)
        if results:
            return results

    results = _parse_items(text)
    if results:
        return results

    # 尝试直接请求接口
    results = _try_api(today)
    return results


def _fetch_page(url: str) -> str:
    """尝试用多种方式获取页面内容"""
    # 方式1: 通过 jina.ai reader 代理（能绕过 WAF）
    try:
        r = requests.get(f"https://r.jina.ai/{url}",
                         headers={"User-Agent": "Mozilla/5.0", "Accept": "text/plain"}, timeout=8)
        if r.status_code == 200 and ("人气榜" in r.text or "36氪" in r.text):
            return r.text
    except Exception:
        pass

    # 方式2: cloudscraper
    if HAS_CLOUDSCRAPER:
        try:
            scraper = cloudscraper.create_scraper()
            r = scraper.get(url, timeout=8)
            if r.status_code == 200 and "人气榜" in r.text:
                return r.text
        except Exception:
            pass

    # 方式3: 普通 requests
    try:
        r = requests.get(url, headers=HEADERS, timeout=8)
        if r.status_code == 200 and "人气榜" in r.text:
            return r.text
    except Exception:
        pass

    return ""


def _parse_markdown(md: str) -> list[dict]:
    """从 jina.ai 返回的 Markdown 中提取榜单"""
    results = []
    articles = re.findall(r'\[(.*?)\]\((https://36kr\.com/p/\d+)\)', md)
    heats = re.findall(r'\|(\d+\.?\d*)w\s*热度', md)
    if not articles:
        return results

    seen = {}
    for raw_title, url in articles:
        title = re.sub(r'!\[.*?\]\(.*?\)', '', raw_title).strip()
        cleaned = re.sub(r'^\[.*?\]\(.*?\)\s*', '', title).strip()
        if cleaned: title = cleaned
        if not title: continue
        if url not in seen:
            seen[url] = {"title": title, "content": ""}
        elif seen[url]["content"] == "" and title != seen[url]["title"]:
            seen[url]["content"] = title

    urls = list(seen.keys())
    for idx, url in enumerate(urls, start=1):
        hot_val = 0
        if idx - 1 < len(heats):
            try:
                h = heats[idx - 1]
                hot_val = int(float(h) * 10000) if '.' in h else int(h) * 10000
            except (ValueError, IndexError):
                pass
        results.append({
            "rank": idx,
            "title": seen[url]["title"],
            "content": seen[url]["content"],
            "hot": hot_val,
            "url": url,
        })
    return results

def _parse_items(html: str) -> list[dict]:
    """从 HTML / __NEXT_DATA__ 中解析榜单"""
    import json as json_mod

    # 1) 尝试 __NEXT_DATA__
    match = re.search(r'__NEXT_DATA__\s*=\s*({.*?});', html, re.DOTALL)
    if match:
        try:
            nd = json_mod.loads(match.group(1))
            # Next.js 中榜单数据通常在 props.pageProps 下
            props = nd.get("props", {}).get("pageProps", {})
            items = _extract_from_props(props)
            if items:
                return items
        except Exception:
            pass

    # 2) 尝试 script 中的数据
    for script in re.findall(r'<script[^>]*id="__NEXT_DATA__"[^>]*>(.*?)</script>', html, re.DOTALL):
        try:
            nd = json_mod.loads(script)
            items = _extract_from_props(nd.get("props", {}).get("pageProps", {}))
            if items:
                return items
        except Exception:
            pass

    return []


def _extract_from_props(props: dict) -> list[dict]:
    """从 props 中提取 hotList 数据"""
    items = []

    # 尝试不同路径
    for path in ["hotList", "hotListData", "list", "dataList"]:
        data = props
        for key in path.split("."):
            if isinstance(data, dict):
                data = data.get(key, {})
            else:
                data = {}
                break
        if isinstance(data, list):
            items = data
            break
        elif isinstance(data, dict):
            items = data.get("list", data.get("data", data.get("items", [])))
            break

    if not items:
        return []

    results = []
    for idx, item in enumerate(items, start=1):
        if isinstance(item, dict):
            title = item.get("title", item.get("name", item.get("articleTitle", "")))
            content = item.get("content", item.get("summary", item.get("description", item.get("articleSummary", ""))))
            hot = item.get("hot", item.get("heat", item.get("hotScore", item.get("readCount", item.get("statRead", 0)))))
            url = item.get("url", item.get("link", item.get("articleUrl", "")))
            if not url:
                id_val = item.get("id", item.get("itemId", item.get("articleId", "")))
                if id_val:
                    url = f"https://36kr.com/p/{id_val}"

            if title:
                results.append({
                    "rank": idx,
                    "title": str(title).strip(),
                    "content": str(content).strip() if content else "",
                    "hot": int(hot) if isinstance(hot, (int, float)) else 0,
                    "url": url,
                })

    return results


def _try_api(today: str) -> list[dict]:
    """尝试直接请求 36kr JSON 接口"""
    import json as json_mod

    api_urls = [
        f"https://36kr.com/api/hot-list/renqi?date={today}&page=1",
        f"https://36kr.com/motif/hot-list/renqi?page=1",
    ]

    for api_url in api_urls:
        try:
            if HAS_CLOUDSCRAPER:
                scraper = cloudscraper.create_scraper()
                r = scraper.get(api_url, timeout=10)
            else:
                r = requests.get(api_url, headers=HEADERS, timeout=10)

            if r.status_code == 200:
                try:
                    data = json_mod.loads(r.text)
                    items = _extract_from_api(data)
                    if items:
                        return items
                except Exception:
                    pass
        except Exception:
            pass

    return []


def _extract_from_api(data: dict) -> list[dict]:
    """从 API JSON 中提取榜单"""
    items = []
    # 尝试各种可能的 data 路径
    for key in ["data", "result", "content", "list"]:
        d = data.get(key, data)
        if isinstance(d, list):
            items = d
            break
        elif isinstance(d, dict):
            for sub in ["list", "data", "items", "records", "rows"]:
                if sub in d and isinstance(d[sub], list):
                    items = d[sub]
                    break
            if items:
                break
    if not items and isinstance(data, list):
        items = data

    results = []
    for idx, item in enumerate(items, start=1):
        if isinstance(item, dict):
            title = item.get("title", item.get("name", ""))
            content = item.get("content", item.get("summary", item.get("description", "")))
            hot = item.get("hot", item.get("heat", item.get("hotScore", item.get("readCount", 0))))
            url = item.get("url", item.get("link", ""))
            if not url and item.get("id"):
                url = f"https://36kr.com/p/{item['id']}"
            if title:
                results.append({
                    "rank": idx,
                    "title": str(title).strip(),
                    "content": str(content).strip() if content else "",
                    "hot": int(hot) if isinstance(hot, (int, float)) else 0,
                    "url": url,
                })

    return results


def print_list(results: list[dict]) -> None:
    """格式化打印榜单"""
    if not results:
        print("暂无数据")
        return
    print(f"{'='*60}")
    print(f"  36氪人气热榜  ({time.strftime('%Y-%m-%d %H:%M:%S')})")
    print(f"{'='*60}")
    for item in results:
        hot = f"🔥 {item['hot']}" if item['hot'] else ""
        print(f"  {item['rank']:>3}. {item['title']} {hot}")
        if item['content']:
            print(f"       {item['content'][:80]}")
        print()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--json", action="store_true", help="输出 JSON 格式")
    args = parser.parse_args()

    results = fetch_hot_list()

    if not results:
        if args.json:
            print(json.dumps({"code": 500, "error": "无法获取36氪人气热榜数据"}, ensure_ascii=False))
        else:
            print("[ERROR] 无法获取36氪人气热榜数据，请稍后重试")
        return

    if args.json:
        print(json.dumps({
            "code": 200,
            "data": results,
            "time": time.strftime("%Y-%m-%d %H:%M:%S"),
        }, ensure_ascii=False))
    else:
        print_list(results)


if __name__ == "__main__":
    main()
