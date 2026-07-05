#!/usr/bin/env python3
"""
扫描本地 assets/music 目录下的所有 MP3 文件并输出 JSON 列表
"""

import json
import os
import sys

MUSIC_DIR = "/app/assets/music"


def scan_music_files() -> list[dict]:
    results = []
    if not os.path.isdir(MUSIC_DIR):
        return results

    for fname in sorted(os.listdir(MUSIC_DIR)):
        if not fname.lower().endswith(".mp3"):
            continue
        name = fname.replace(".mp3", "").strip()
        parts = name.split(" - ", 1)
        if len(parts) == 2:
            artist, title = parts
        else:
            title = name
            artist = "未知"
        results.append({
            "title": title.strip(),
            "artist": artist.strip(),
            "src": f"/api/v1/music/file/{fname}",
        })
    return results


def main():
    try:
        results = scan_music_files()
        print(json.dumps({"code": 200, "data": results}, ensure_ascii=False))
    except Exception as e:
        print(json.dumps({"code": 500, "error": str(e)}, ensure_ascii=False))


if __name__ == "__main__":
    main()
