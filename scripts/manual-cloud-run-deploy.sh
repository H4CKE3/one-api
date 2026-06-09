#!/usr/bin/env bash
# One API 手动 Cloud Run 发布脚本
# 用法: ./scripts/manual-cloud-run-deploy.sh [--yes] [--tokyo-only] [--us-only] [--skip-build IMAGE@sha256:...]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

PROJECT_ID="optimal-chimera-472503-d9"
SERVICE_TOKYO="one-api"
SERVICE_US="one-api-us"
REGION_TOKYO="asia-northeast1"
REGION_US="us-west2"
IMAGE="asia-northeast1-docker.pkg.dev/${PROJECT_ID}/cloud-run-source-deploy/one-api/one-api"
TOKYO_URL="https://one-api-973403011091.asia-northeast1.run.app"
US_URL="https://one-api-us-973403011091.us-west2.run.app"

DEPLOY_TOKYO=true
DEPLOY_US=true
SKIP_BUILD=false
FULL_IMAGE=""
AUTO_YES=false

usage() {
  cat <<'EOF'
用法: ./scripts/manual-cloud-run-deploy.sh [选项]

默认流程: Cloud Build 构建镜像 -> 部署东京 -> 确认后部署美西

选项:
  --yes              跳过美西部署前的确认提示
  --tokyo-only       只部署东京
  --us-only          只部署美西（需配合 --skip-build 指定已有镜像）
  --skip-build IMG   跳过构建，直接使用已有镜像（例如 IMAGE@sha256:abc...）
  -h, --help         显示帮助

示例:
  ./scripts/manual-cloud-run-deploy.sh
  ./scripts/manual-cloud-run-deploy.sh --yes
  ./scripts/manual-cloud-run-deploy.sh --tokyo-only
  ./scripts/manual-cloud-run-deploy.sh --skip-build "asia-northeast1-docker.pkg.dev/.../one-api@sha256:..."
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --yes)
      AUTO_YES=true
      shift
      ;;
    --tokyo-only)
      DEPLOY_TOKYO=true
      DEPLOY_US=false
      shift
      ;;
    --us-only)
      DEPLOY_TOKYO=false
      DEPLOY_US=true
      shift
      ;;
    --skip-build)
      SKIP_BUILD=true
      FULL_IMAGE="${2:?--skip-build 需要提供完整镜像地址}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "未知参数: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "错误: 未找到命令 '$1'，请先安装 gcloud CLI。" >&2
    echo "文档: docs/manual-cloud-run-release.md" >&2
    exit 1
  fi
}

echo "==> 检查 gcloud"
require_cmd gcloud

ACTIVE_ACCOUNT="$(gcloud auth list --filter=status:ACTIVE --format='value(account)' 2>/dev/null || true)"
if [[ -z "${ACTIVE_ACCOUNT}" ]]; then
  echo "错误: 未检测到已登录的 gcloud 账号，请先运行: gcloud auth login" >&2
  exit 1
fi
echo "    当前账号: ${ACTIVE_ACCOUNT}"

echo "==> 设置 GCP 项目: ${PROJECT_ID}"
gcloud config set project "${PROJECT_ID}" >/dev/null

TAG="manual-$(git rev-parse --short HEAD)-$(date +%Y%m%d-%H%M%S)"

if [[ "${SKIP_BUILD}" == false ]]; then
  echo "==> 构建并推送镜像 (tag: ${TAG})"
  gcloud builds submit . \
    --project "${PROJECT_ID}" \
    --config cloudbuild.manual-release.yaml \
    --substitutions "_IMAGE=${IMAGE},_TAG=${TAG}"

  echo "==> 获取镜像 digest"
  DIGEST="$(gcloud artifacts docker images describe "${IMAGE}:${TAG}" \
    --project "${PROJECT_ID}" \
    --format='value(image_summary.digest)')"
  FULL_IMAGE="${IMAGE}@${DIGEST}"
else
  echo "==> 跳过构建，使用已有镜像"
fi

echo "    镜像: ${FULL_IMAGE}"

deploy_service() {
  local service="$1"
  local region="$2"
  local url="$3"

  echo "==> 部署 ${service} (${region})"
  gcloud run deploy "${service}" \
    --project "${PROJECT_ID}" \
    --region "${region}" \
    --image "${FULL_IMAGE}"

  local deployed
  deployed="$(gcloud run services describe "${service}" \
    --project "${PROJECT_ID}" \
    --region "${region}" \
    --format='value(spec.template.spec.containers[0].image)')"
  echo "    当前镜像: ${deployed}"

  echo "==> 健康检查 ${url}"
  curl -fsSI "${url}" | head -n 1
}

if [[ "${DEPLOY_TOKYO}" == true ]]; then
  deploy_service "${SERVICE_TOKYO}" "${REGION_TOKYO}" "${TOKYO_URL}"
fi

if [[ "${DEPLOY_US}" == true ]]; then
  if [[ "${DEPLOY_TOKYO}" == true && "${AUTO_YES}" == false ]]; then
    echo
    read -r -p "东京已部署，是否继续部署美西 ${SERVICE_US}? [y/N] " reply
    if [[ ! "${reply}" =~ ^[Yy]$ ]]; then
      echo "已跳过美西部署。"
      exit 0
    fi
  fi
  deploy_service "${SERVICE_US}" "${REGION_US}" "${US_URL}"
fi

echo
echo "手动发布完成。"
echo "镜像: ${FULL_IMAGE}"
