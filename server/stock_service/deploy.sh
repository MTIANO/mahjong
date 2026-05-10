#!/bin/bash
# AKShare 股票数据微服务部署脚本
# 使用方式: 在服务器上 cd 到 stock_service 目录后执行 bash deploy.sh

set -e

APP_DIR=$(cd "$(dirname "$0")" && pwd)
SERVICE_NAME="stock-service"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
VENV_DIR="${APP_DIR}/venv"
PORT=5001

echo "=== AKShare 微服务部署 ==="
echo "目录: ${APP_DIR}"
echo "端口: ${PORT}"
echo ""

# 1. 检查 Python3
if ! command -v python3 &> /dev/null; then
    echo "[错误] 未找到 python3，请先安装: sudo apt install python3 python3-venv python3-pip"
    exit 1
fi

PYTHON_VERSION=$(python3 --version)
echo "[✓] ${PYTHON_VERSION}"

# 2. 创建虚拟环境并安装依赖
echo ""
echo "--- 安装 Python 依赖 ---"
if [ ! -d "${VENV_DIR}" ]; then
    python3 -m venv "${VENV_DIR}"
    echo "[✓] 虚拟环境已创建"
fi

source "${VENV_DIR}/bin/activate"
pip install --upgrade pip -q
pip install -r "${APP_DIR}/requirements.txt" -q
echo "[✓] 依赖安装完成"
deactivate

# 3. 创建 systemd service 文件
echo ""
echo "--- 配置 systemd 服务 ---"

sudo tee "${SERVICE_FILE}" > /dev/null <<EOF
[Unit]
Description=AKShare Stock Data Service
After=network.target

[Service]
Type=simple
User=$(whoami)
WorkingDirectory=${APP_DIR}
ExecStart=${VENV_DIR}/bin/python ${APP_DIR}/app.py
Restart=always
RestartSec=5
Environment=PYTHONUNBUFFERED=1

[Install]
WantedBy=multi-user.target
EOF

echo "[✓] systemd 服务文件已创建: ${SERVICE_FILE}"

# 4. 启动服务
sudo systemctl daemon-reload
sudo systemctl enable "${SERVICE_NAME}"
sudo systemctl restart "${SERVICE_NAME}"

echo ""
echo "--- 检查服务状态 ---"
sleep 2
if sudo systemctl is-active --quiet "${SERVICE_NAME}"; then
    echo "[✓] 服务已启动"
    curl -s "http://localhost:${PORT}/health" && echo ""
else
    echo "[✗] 服务启动失败，查看日志:"
    sudo journalctl -u "${SERVICE_NAME}" -n 20 --no-pager
    exit 1
fi

echo ""
echo "=== 部署完成 ==="
echo ""
echo "常用命令:"
echo "  查看状态: sudo systemctl status ${SERVICE_NAME}"
echo "  查看日志: sudo journalctl -u ${SERVICE_NAME} -f"
echo "  重启服务: sudo systemctl restart ${SERVICE_NAME}"
echo "  停止服务: sudo systemctl stop ${SERVICE_NAME}"
