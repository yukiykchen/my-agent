#!/usr/bin/env python3
"""Draw a clean swimlane activity diagram for evidence collection business flow."""
import subprocess
from pathlib import Path
from xml.sax.saxutils import escape

ROOT = Path(__file__).resolve().parents[1]
SVG_OUT = ROOT / "docs" / "evidence-business-activity-diagram.svg"
PNG_OUT = ROOT / "docs" / "evidence-business-activity-diagram.png"
W, H = 1400, 1000
FONT = "PingFang SC, Microsoft YaHei, Noto Sans CJK SC, Arial, sans-serif"
ARROW = "#475569"
LANE = "#cbd5e1"
SYS_FILL = "#c9f7ef"
SYS_STROKE = "#0f9488"
EXT_FILL = "#fff7ed"
EXT_STROKE = "#f97316"
USER_FILL = "#eaf2ff"
USER_STROKE = "#3b82f6"


def text(x, y, s, size=16, color="#334155", weight="400", anchor="middle"):
    return f'<text x="{x}" y="{y}" text-anchor="{anchor}" font-family="{FONT}" font-size="{size}" font-weight="{weight}" fill="{color}">{escape(s)}</text>'


def multi(x, y, lines, size=16, color="#155e63", weight="600"):
    if isinstance(lines, str):
        lines = [lines]
    gap = size + 5
    y0 = y - (len(lines) - 1) * gap / 2
    return "\n".join(text(x, y0 + i * gap, line, size, color, weight) for i, line in enumerate(lines))


def rect(x, y, w, h, label, fill=SYS_FILL, stroke=SYS_STROKE, color="#155e63"):
    return "\n".join([
        f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="10" fill="{fill}" stroke="{stroke}" stroke-width="2.1"/>',
        multi(x + w / 2, y + h / 2 + 5, label, 16, color, "600"),
    ])


def diamond(cx, cy, w, h, label):
    pts = f"{cx},{cy-h/2} {cx+w/2},{cy} {cx},{cy+h/2} {cx-w/2},{cy}"
    return "\n".join([
        f'<polygon points="{pts}" fill="#fffdf8" stroke="{EXT_STROKE}" stroke-width="2.1"/>',
        text(cx, cy + 5, label, 15, "#9a3412", "600"),
    ])


def bar(x, y, w=320):
    return f'<rect x="{x-w/2}" y="{y-4}" width="{w}" height="8" rx="4" fill="#334155"/>'


def start(cx, cy):
    return f'<circle cx="{cx}" cy="{cy}" r="14" fill="#111827"/>'


def end(cx, cy):
    return f'<circle cx="{cx}" cy="{cy}" r="14" fill="#fff" stroke="#111827" stroke-width="3"/><circle cx="{cx}" cy="{cy}" r="8" fill="#111827"/>'


def arrow(points, label=None, label_pos=None):
    if len(points) == 2:
        (x1, y1), (x2, y2) = points
        line = f'<line x1="{x1}" y1="{y1}" x2="{x2}" y2="{y2}" stroke="{ARROW}" stroke-width="2" marker-end="url(#arrow)"/>'
    else:
        pts = " ".join(f"{x},{y}" for x, y in points)
        line = f'<polyline points="{pts}" fill="none" stroke="{ARROW}" stroke-width="2" marker-end="url(#arrow)"/>'
    if label:
        lx, ly = label_pos
        return line + "\n" + f'<rect x="{lx-60}" y="{ly-16}" width="120" height="22" rx="5" fill="#f8fbff" opacity="0.96"/>\n' + text(lx, ly, label, 13, "#64748b", "500")
    return line


parts = [
    f'<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{H}" viewBox="0 0 {W} {H}">',
    "<defs>",
    '<marker id="arrow" markerWidth="10" markerHeight="10" refX="9" refY="3" orient="auto" markerUnits="strokeWidth"><path d="M0,0 L0,6 L9,3 z" fill="#475569"/></marker>',
    "</defs>",
    '<rect width="100%" height="100%" fill="#ffffff"/>',
    text(W / 2, 36, "网络侵权证据处理业务活动图", 28, "#334155", "700"),
    text(W / 2, 64, "判断场景类型后分叉处理，采集完成后统一固化与出证", 15, "#94a3b8", "500"),
]

lane_top, header_h, lane_bottom = 86, 36, 950
lanes = [(60, 290, "用户 / 业务部门"), (290, 900, "系统 / Agent"), (900, 1340, "外部工具 / 存证服务")]
for x1, x2, title in lanes:
    parts.append(f'<rect x="{x1}" y="{lane_top}" width="{x2-x1}" height="{lane_bottom-lane_top}" fill="#f8fbff" stroke="{LANE}" stroke-width="1.6"/>')
    parts.append(f'<rect x="{x1}" y="{lane_top}" width="{x2-x1}" height="{header_h}" fill="#f8fafc" stroke="{LANE}" stroke-width="1.6"/>')
    parts.append(text((x1 + x2) / 2, lane_top + 24, title, 16, "#475569", "600"))

parts += [
    start(175, 155),
    rect(95, 200, 160, 54, "提交侵权线索", USER_FILL, USER_STROKE, "#1d4ed8"),
    rect(465, 200, 220, 54, "生成取证任务"),
    diamond(575, 315, 170, 70, "判断场景类型"),
    rect(350, 390, 240, 56, "静态网页 / 图文取证"),
    rect(630, 500, 240, 56, "直播 / 视频取证"),
    rect(965, 390, 305, 58, ["浏览器截图", "网页抓取 / OCR"], EXT_FILL, EXT_STROKE, "#c2410c"),
    rect(965, 500, 305, 58, ["录屏分段 / 视频抽帧", "ASR / OCR"], EXT_FILL, EXT_STROKE, "#c2410c"),
    bar(610, 610),
    rect(480, 645, 260, 56, "汇总原始取证数据"),
    rect(965, 645, 305, 58, ["哈希计算 / TSA 时间戳", "区块链上链"], EXT_FILL, EXT_STROKE, "#c2410c"),
    rect(480, 740, 260, 56, "生成证据材料"),
    rect(480, 805, 260, 56, "法律分析与证据组织"),
    rect(480, 870, 260, 56, "生成分析报告"),
    rect(95, 870, 160, 54, "接收证据包", USER_FILL, USER_STROKE, "#1d4ed8"),
    end(175, 970),
]

# 主流程与分叉/汇合
parts += [
    arrow([(175, 169), (175, 200)]),
    arrow([(255, 227), (465, 227)]),
    arrow([(575, 254), (575, 280)]),
    arrow([(535, 340), (470, 390)]),
    arrow([(615, 340), (750, 500)]),
    arrow([(470, 446), (470, 610)]),
    arrow([(750, 556), (750, 610)]),
    arrow([(610, 614), (610, 645)]),
    arrow([(610, 701), (610, 740)]),
    arrow([(610, 796), (610, 805)]),
    arrow([(610, 861), (610, 870)]),
    arrow([(480, 897), (255, 897)]),
    arrow([(175, 924), (175, 956)]),
]

# 外部工具调用：只画调用关系，不画回流交叉线
parts += [
    arrow([(590, 418), (965, 419)], "调用工具", (775, 410)),
    arrow([(870, 528), (965, 529)], "调用工具", (918, 520)),
    arrow([(740, 673), (965, 674)], "统一固化", (850, 666)),
    arrow([(965, 674), (740, 768)], "证据链结果", (860, 745)),
]

parts.append("</svg>")
SVG_OUT.write_text("\n".join(parts), encoding="utf-8")
print(f"generated svg: {SVG_OUT}")

chrome = Path("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome")
if chrome.exists():
    subprocess.run([str(chrome), "--headless", "--disable-gpu", "--no-sandbox", f"--window-size={W},{H}", f"--screenshot={PNG_OUT}", SVG_OUT.as_uri()], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    print(f"generated png: {PNG_OUT}")
else:
    print("png skipped: Google Chrome not found; SVG is still available")
