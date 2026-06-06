# One API 手动发布操作手册

这份手册用于从本机安装 `gcloud CLI` 开始，手动构建镜像并发布到 GCP 上的 `Cloud Run` 服务。

适用场景：

- 现有 `Cloud Build Trigger` 绑定了错误的 GitHub 仓库
- 当前仓库只有 GitHub `read/write` 权限，没有 `admin` 权限
- 需要绕过 Trigger，直接用本地代码手动发版

本手册使用的手动发布文件：

- `Dockerfile.manual-release`
- `cloudbuild.manual-release.yaml`

它们不会改动仓库原有的 `Dockerfile`。

## 1. 前提条件

- 你有 GCP 项目 `optimal-chimera-472503-d9` 的可用权限
- 至少具备这些能力：
  - Cloud Build 构建
  - Artifact Registry 推送镜像
  - Cloud Run 更新服务
  - 如有需要，具备运行时 Service Account 的 `iam.serviceAccountUser`

## 2. 安装 gcloud CLI

官方安装文档：

- 安装说明: <https://docs.cloud.google.com/sdk/docs/install-sdk>
- 初始化说明: <https://docs.cloud.google.com/sdk/docs/initialize>

### macOS

推荐使用 Google 提供的交互式安装器，安装完成后重开终端：

```bash
curl -O https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-darwin-arm.tar.gz
tar -xf google-cloud-cli-darwin-arm.tar.gz
./google-cloud-sdk/install.sh
```

Intel Mac 可将文件名改为 `google-cloud-cli-darwin-x86_64.tar.gz`。

### Debian / Ubuntu

```bash
sudo apt-get update
sudo apt-get install ca-certificates gnupg curl
curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg
echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | sudo tee -a /etc/apt/sources.list.d/google-cloud-sdk.list
sudo apt-get update && sudo apt-get install google-cloud-cli
```

## 3. 初始化与认证

本机终端执行：

```bash
gcloud init
```

如果是在远程终端或不方便自动打开浏览器的环境，执行：

```bash
gcloud init --console-only
```

在大多数“本机交互式登录”场景里，`gcloud init` 会顺带引导你完成 `gcloud auth login`，所以通常不需要再额外执行一次登录。

但以下情况，建议单独执行认证命令：

- `gcloud init` 过程中没有成功完成浏览器登录
- 当前激活账号不是你要发版用的账号
- 本地凭证过期
- 你在新的终端环境、远程环境、跳板机环境里重新配置

可用命令：

```bash
gcloud auth login
```

如果是在远程终端或无图形环境：

```bash
gcloud auth login --no-launch-browser
```

初始化或登录完成后，建议先确认账号和项目：

```bash
gcloud auth list
gcloud config list
```

如果要切换当前激活账号：

```bash
gcloud config set account "<your-account@example.com>"
```

说明：

- 本手册里的 `gcloud builds submit` 和 `gcloud run deploy` 不需要额外执行 `gcloud auth application-default login`
- `gcloud auth application-default login` 主要用于本地程序通过 ADC 调 Google API，不是这套纯 CLI 发布流程的必需步骤

## 4. 获取代码

```bash
cd /Users/asuka/project
git clone https://github.com/mogotech-ai/one-api.git
cd one-api
```

如果要发布特定分支或提交，先切换：

```bash
git checkout <branch-or-commit>
```

## 5. 关键文件说明

- `Dockerfile`：仓库原始文件，不用于这套手动发布流程
- `Dockerfile.manual-release`：手动发布专用 Dockerfile
- `cloudbuild.manual-release.yaml`：让 Cloud Build 显式使用 `Dockerfile.manual-release`

这样做的原因：

- 原始 `Dockerfile` 依赖 `BUILDPLATFORM`
- 原始 `Dockerfile` 在当前 Cloud Build 默认构建方式下会失败
- 手动发布专用文件已经针对这次实际发版问题做过兼容处理

## 6. 设置发布变量

```bash
PROJECT_ID="optimal-chimera-472503-d9"
SERVICE_TOKYO="one-api"
SERVICE_US="one-api-us"
REGION_TOKYO="asia-northeast1"
REGION_US="us-west2"
IMAGE="asia-northeast1-docker.pkg.dev/${PROJECT_ID}/cloud-run-source-deploy/one-api/one-api"
TAG="manual-$(git rev-parse --short HEAD)-$(date +%Y%m%d-%H%M%S)"
```

切换到正确项目：

```bash
gcloud config set project "${PROJECT_ID}"
```

## 7. 发布前检查

查看当前东京服务镜像：

```bash
gcloud run services describe "${SERVICE_TOKYO}" \
  --project "${PROJECT_ID}" \
  --region "${REGION_TOKYO}" \
  --format="value(spec.template.spec.containers[0].image)"
```

查看当前美西服务镜像：

```bash
gcloud run services describe "${SERVICE_US}" \
  --project "${PROJECT_ID}" \
  --region "${REGION_US}" \
  --format="value(spec.template.spec.containers[0].image)"
```

如需确认当前账号与项目：

```bash
gcloud auth list
gcloud config get-value project
```

## 8. 手动构建并推送镜像

使用手动发布专用配置构建：

```bash
gcloud builds submit . \
  --project "${PROJECT_ID}" \
  --config cloudbuild.manual-release.yaml \
  --substitutions "_IMAGE=${IMAGE},_TAG=${TAG}"
```

构建成功后，取回 digest：

```bash
DIGEST="$(gcloud artifacts docker images describe "${IMAGE}:${TAG}" \
  --project "${PROJECT_ID}" \
  --format='value(image_summary.digest)')"

FULL_IMAGE="${IMAGE}@${DIGEST}"
echo "${FULL_IMAGE}"
```

## 9. 部署东京

```bash
gcloud run deploy "${SERVICE_TOKYO}" \
  --project "${PROJECT_ID}" \
  --region "${REGION_TOKYO}" \
  --image "${FULL_IMAGE}"
```

验证东京服务：

```bash
gcloud run services describe "${SERVICE_TOKYO}" \
  --project "${PROJECT_ID}" \
  --region "${REGION_TOKYO}" \
  --format="value(spec.template.spec.containers[0].image)"

curl -I https://one-api-973403011091.asia-northeast1.run.app
```

## 10. 部署美西

确认东京无异常后，再发布美西：

```bash
gcloud run deploy "${SERVICE_US}" \
  --project "${PROJECT_ID}" \
  --region "${REGION_US}" \
  --image "${FULL_IMAGE}"
```

验证美西服务：

```bash
gcloud run services describe "${SERVICE_US}" \
  --project "${PROJECT_ID}" \
  --region "${REGION_US}" \
  --format="value(spec.template.spec.containers[0].image)"

curl -I https://one-api-us-973403011091.us-west2.run.app
```

## 11. 常见问题

### 1. `must provide an image name to deploy`

原因：`FULL_IMAGE` 变量是空的，或者当前 shell 会话里没有这个变量。

处理：

```bash
echo "${FULL_IMAGE}"
```

如果为空，就重新执行取 digest 的命令，或者直接把完整镜像地址写进 `gcloud run deploy --image ...`。

### 2. Cloud Build 提示 `failed to parse platform`

原因：原始 `Dockerfile` 使用了 `--platform=$BUILDPLATFORM`，但默认构建环境没有传这个变量。

处理：不要用原始 `Dockerfile`，改用：

- `Dockerfile.manual-release`
- `cloudbuild.manual-release.yaml`

### 3. 前端构建时报 `mv: cannot move 'build'`

原因：前端并行构建时目标目录不存在。

处理：`Dockerfile.manual-release` 已经包含目录初始化逻辑，不需要额外改命令。

### 4. 构建通过但部署失败

优先检查：

- 当前 `gcloud` 账号是否正确
- 当前 `project` 是否正确
- 是否有 Cloud Run 更新权限
- 是否缺少运行时 Service Account 的使用权限

## 12. 回滚

先查看服务历史 revision：

```bash
gcloud run revisions list \
  --project "${PROJECT_ID}" \
  --region "${REGION_TOKYO}" \
  --service "${SERVICE_TOKYO}"
```

如果只需要回滚到上一版镜像，也可以先找出旧镜像 digest，再重新部署：

```bash
gcloud run deploy "${SERVICE_TOKYO}" \
  --project "${PROJECT_ID}" \
  --region "${REGION_TOKYO}" \
  --image "<old-image@sha256:...>"
```

## 13. 旧镜像清理预案

默认建议：

- 不要在每次发版后立刻清理旧镜像
- 至少保留最近几个可回滚版本
- 优先保留当前线上正在使用的 digest

建议保留策略：

- 保留当前东京和美西线上正在使用的镜像
- 额外保留最近 5 到 10 个手动发布镜像
- 只清理历史 `manual-*` tag，对非手动发布 tag 保守处理

### 13.1 先列出镜像

```bash
gcloud artifacts docker images list "${IMAGE}" \
  --project "${PROJECT_ID}" \
  --include-tags
```

### 13.2 先确认当前线上正在使用的镜像

东京：

```bash
gcloud run services describe "${SERVICE_TOKYO}" \
  --project "${PROJECT_ID}" \
  --region "${REGION_TOKYO}" \
  --format="value(spec.template.spec.containers[0].image)"
```

美西：

```bash
gcloud run services describe "${SERVICE_US}" \
  --project "${PROJECT_ID}" \
  --region "${REGION_US}" \
  --format="value(spec.template.spec.containers[0].image)"
```

清理前，务必把这两个 digest 记下来，不要删除当前线上正在使用的镜像。

### 13.3 手动删除单个旧镜像

如果确认某个旧 digest 不再需要，可以手动删除：

```bash
gcloud artifacts docker images delete "<image@sha256:...>" \
  --project "${PROJECT_ID}" \
  --delete-tags \
  --quiet
```

示例：

```bash
gcloud artifacts docker images delete \
  "asia-northeast1-docker.pkg.dev/optimal-chimera-472503-d9/cloud-run-source-deploy/one-api/one-api@sha256:xxxxxxxx" \
  --project "${PROJECT_ID}" \
  --delete-tags \
  --quiet
```

### 13.4 推荐的保守清理顺序

1. 先查东京和美西当前线上 digest。
2. 列出所有镜像和 tag。
3. 只挑明显过旧的 `manual-*` 镜像。
4. 保留最近 5 到 10 个手动发布版本。
5. 删除前再次确认目标 digest 不等于当前线上 digest。

### 13.5 不建议做的事

- 不要按 tag 名字盲删，而不核对 digest
- 不要把东京和美西共用的当前线上镜像删掉
- 不要在发版后立刻批量删除全部旧镜像
- 不要在没有回滚窗口的情况下清理到只剩 1 个版本
## 14. 本次成功发布记录

本次手动发布成功使用的镜像：

```text
asia-northeast1-docker.pkg.dev/optimal-chimera-472503-d9/cloud-run-source-deploy/one-api/one-api@sha256:52fc985e5d5fad33de6c193e1a2cb154456899970a575cd642725501776096b2
```

对应服务：

- 东京 `one-api`
- 美西 `one-api-us`
