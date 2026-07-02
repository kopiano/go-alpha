#!/usr/bin/env python3
"""
实时爬取微博热搜榜单，以日期为文件名缓存到 data/ 目录
支持 --date MM-DD 参数查询历史缓存数据
"""

import argparse
import glob
import json
import os
import re
import sys
import time

from urllib.parse import quote

import requests

HEADERS = {
    "User-Agent": (
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
        "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
    ),
    "Referer": "https://weibo.com/",
}

DATA_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "data")

# 类别关键词映射（按优先级排列）
CATEGORY_RULES = [
    ("科技", ["苹果", "华为", "小米", "特斯拉", "SpaceX", "AI", "ChatGPT", "DeepSeek", "大模型", "芯片",
              "手机", "5G", "航天", "卫星", "火箭", "机器人", "自动驾驶", "新能源",
              "超导", "磁体", "人造太阳", "马斯克", "张一鸣", "公司"]),
    ("财经", ["股市", "股票", "基金", "比特币", "A股", "涨停", "跌停", "美元",
              "央行", "利率", "房价", "油价", "黄金", "期货", "IPO", "上市", "融资"]),
    ("国际", ["美国", "特朗普", "拜登", "俄罗斯", "乌克兰", "日本", "韩国", "朝鲜",
              "欧盟", "北约", "联合国", "中东", "以色列", "伊朗", "印度", "欧洲",
              "外交", "大使", "制裁", "冲突", "战争", "停火", "出局", "世界杯"]),
    ("体育", ["NBA", "CBA", "欧冠", "奥运", "世界杯", "足球", "篮球", "网球",
              "C罗", "梅西", "詹姆斯", "库里", "冠军", "决赛", "淘汰", "进球",
              "金牌", "银牌", "铜牌"]),
    ("娱乐", ["电影", "电视剧", "综艺", "歌手", "演员", "导演", "演唱会", "首映",
              "白玉兰", "金鸡奖", "奥斯卡", "票房", "专辑", "新歌",
              "杨紫", "迪丽热巴", "李现", "龚俊", "刘亦菲", "恋与深空", "TF家族",
              "浪姐", "祖海", "甲亢哥"]),
    ("社会", ["警方", "法院", "案件", "调查", "通报", "官方", "塌房", "人命", "逝世",
              "去世", "讣告", "地震", "台风", "暴雨", "洪水", "交通事故", "爆炸",
              "火灾", "疫情", "病毒", "确诊", "网红", "诋毁", "袁隆平"]),
]


def classify(title: str, note: str = "") -> str:
    """根据标题和备注智能分类（英文不区分大小写）"""
    text = title + note
    for category, keywords in CATEGORY_RULES:
        for kw in keywords:
            if re.search(re.escape(kw), text, re.IGNORECASE):
                return category
    return "其他"


def data_path(date_str: str = "") -> str:
    """获取当天缓存文件路径，含小时以保留多次快照"""
    os.makedirs(DATA_DIR, exist_ok=True)
    if date_str:
        return os.path.join(DATA_DIR, f"{date_str}.json")
    ts = time.strftime("%m-%d_%H")
    return os.path.join(DATA_DIR, f"{ts}.json")


def fetch_hot_search() -> list[dict]:
    """爬取微博实时热搜榜单"""
    url = "https://weibo.com/ajax/side/hotSearch"
    resp = requests.get(url, headers=HEADERS, timeout=10)
    resp.raise_for_status()
    data = resp.json()

    realtime = data.get("data", {}).get("realtime", [])
    results = []
    for idx, item in enumerate(realtime, start=1):
        title = item.get("word", "")
        note = item.get("note", "")
        results.append({
            "rank": idx,
            "title": title,
            "hot": item.get("num", 0),
            "label": item.get("label_name", ""),
            "category": classify(title, note),
            "url": f"https://s.weibo.com/weibo?q={quote(item.get('word_scheme', item.get('word', '')))}",
        })
    return results


def print_hot_search(results: list[dict], date_str: str = "") -> None:
    """格式化打印热搜榜单"""
    label = f" ({date_str})" if date_str else ""
    print(f"{'='*60}")
    print(f"  微博热搜榜{label}  ({time.strftime('%Y-%m-%d %H:%M:%S')})")
    print(f"{'='*60}")
    for item in results:
        label = f" [{item['label']}]" if item["label"] else ""
        cat = f"({item['category']})"
        hot = f"🔥 {item['hot']}" if item["hot"] else ""
        print(f"  {item['rank']:>3}. {cat} {item['title']}{label} {hot}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--json", action="store_true", help="输出 JSON 格式")
    args = parser.parse_args()

    try:
        results = fetch_hot_search()

        if args.json:
            print(json.dumps({
                "code": 200,
                "data": results,
                "time": time.strftime("%Y-%m-%d %H:%M:%S"),
            }, ensure_ascii=False))
        else:
            print_hot_search(results)
    except requests.RequestException as e:
        if args.json:
            print(json.dumps({"code": 500, "error": f"请求失败: {e}"}, ensure_ascii=False))
        else:
            print(f"[ERROR] 请求失败: {e}")
    except (KeyError, json.JSONDecodeError) as e:
        if args.json:
            print(json.dumps({"code": 500, "error": f"数据解析失败: {e}"}, ensure_ascii=False))
        else:
            print(f"[ERROR] 数据解析失败: {e}")


if __name__ == "__main__":
    main()
