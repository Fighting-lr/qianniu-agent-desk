#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""
[AgentDesk] 导出场景分类评估集

从官方 xlsx 的「聊天记录」sheet 提取每条会话的买家原声 + 官方场景标注，
产出 JSON 供 Go 侧评估器消费。

用法:
    python scripts/export_eval_set.py [输出路径]

默认输出: build/eval_scene_set.json
"""

from __future__ import annotations

import io
import json
import sys
from collections import defaultdict
from pathlib import Path

from openpyxl import load_workbook

XLSX = Path("D:/浏览器下载/赛题 1：数据共情者-业务数据.xlsx")
DEFAULT_OUT = Path("build/eval_scene_set.json")

# 表头索引（与 xlsx 实际列序一致，改动会立即在断言处暴露）
COL_SESSION = 0
COL_ROLE = 4
COL_SCENE_MAJOR = 8
COL_SCENE_MINOR = 9
COL_IS_TARGET = 10
COL_TEXT = 11


def export(xlsx: Path, out: Path) -> dict:
    wb = load_workbook(xlsx, data_only=True, read_only=True)
    ws = wb["聊天记录"]

    rows = ws.iter_rows(values_only=True)
    header = next(rows)

    # 防御：列序若变了要立刻报错，而不是静默取错列
    assert header[COL_SESSION] == "会话ID", f"列 {COL_SESSION} 应为「会话ID」，实际 {header[COL_SESSION]!r}"
    assert header[COL_SCENE_MAJOR] == "scene_major", f"列 {COL_SCENE_MAJOR} 应为 scene_major"
    assert header[COL_SCENE_MINOR] == "scene_minor", f"列 {COL_SCENE_MINOR} 应为 scene_minor"
    assert header[COL_IS_TARGET] == "is_target_buyer_message", f"列 {COL_IS_TARGET} 应为 is_target_buyer_message"
    assert header[COL_TEXT] == "message_text", f"列 {COL_TEXT} 应为 message_text"

    # 会话级聚合
    sessions: dict[str, dict] = {}
    order: list[str] = []

    for r in rows:
        sid = r[COL_SESSION]
        if not sid:
            continue
        sid = str(sid).strip()

        if sid not in sessions:
            sessions[sid] = {
                "sessionNo": sid,
                "buyerTexts": [],
                "expectMajor": "",
                "expectMinor": "",
                "messageCount": 0,
            }
            order.append(sid)

        s = sessions[sid]
        s["messageCount"] += 1

        # 场景标注：同一会话内应一致；若不一致，保留首次出现（并计数以便发现）
        mj = (r[COL_SCENE_MAJOR] or "").strip() if isinstance(r[COL_SCENE_MAJOR], str) else (r[COL_SCENE_MAJOR] or "")
        mi = (r[COL_SCENE_MINOR] or "").strip() if isinstance(r[COL_SCENE_MINOR], str) else (r[COL_SCENE_MINOR] or "")
        if mj and not s["expectMajor"]:
            s["expectMajor"] = str(mj).strip()
        if mi and not s["expectMinor"]:
            s["expectMinor"] = str(mi).strip()

        # 只收买家消息作为分类器输入（客服话术不反映买家诉求）
        if r[COL_IS_TARGET] == 1:
            txt = r[COL_TEXT]
            if txt:
                txt = str(txt).strip()
                # 图片类消息文本常是「喏，图发你了」这类无信息量的话，仍保留
                # —— 分类器本就该处理"信息不足"的情况，过滤掉反而是作弊。
                if txt:
                    s["buyerTexts"].append(txt)

    cases = [sessions[sid] for sid in order]

    # ---- 统计 ----
    majors = defaultdict(int)
    minors = defaultdict(int)
    no_text = 0
    for c in cases:
        majors[c["expectMajor"]] += 1
        minors[c["expectMinor"]] += 1
        if not c["buyerTexts"]:
            no_text += 1

    payload = {
        "source": str(xlsx),
        "totalSessions": len(cases),
        "sessionsWithoutBuyerText": no_text,
        "distinctMajor": len(majors),
        "distinctMinor": len(minors),
        "majorDistribution": dict(sorted(majors.items(), key=lambda kv: -kv[1])),
        "minorDistribution": dict(sorted(minors.items(), key=lambda kv: -kv[1])),
        "cases": cases,
    }

    out.parent.mkdir(parents=True, exist_ok=True)
    with io.open(out, "w", encoding="utf-8", newline="\n") as f:
        json.dump(payload, f, ensure_ascii=False, indent=2)

    return payload


def main() -> int:
    out = Path(sys.argv[1]) if len(sys.argv) > 1 else DEFAULT_OUT
    if not XLSX.exists():
        print(f"[FAIL] 找不到官方数据文件: {XLSX}", file=sys.stderr)
        return 1

    p = export(XLSX, out)

    print(f"[OK] 导出 {p['totalSessions']} 条会话 → {out}")
    print(f"     大类 {p['distinctMajor']} 个 / 子类 {p['distinctMinor']} 个")
    print(f"     无买家原声的会话: {p['sessionsWithoutBuyerText']}")
    print("\n大类分布:")
    for k, v in p["majorDistribution"].items():
        print(f"   {k:<16} {v}")

    # 完整性提醒
    if p["distinctMinor"] != 41:
        print(f"\n[WARN] 子类应为 41，实际 {p['distinctMinor']} —— 官方数据覆盖不全，评估结论需注明")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
