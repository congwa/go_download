#!/bin/bash

# 检查 GITHUB_TOKEN 环境变量
if [ -z "$GITHUB_TOKEN" ]; then
    echo "错误: 未设置 GITHUB_TOKEN 环境变量"
    echo "请先设置 GitHub Token:"
    echo "export GITHUB_TOKEN='your_token_here'"
    exit 1
fi

# GitHub 仓库信息
GITHUB_REPO="congwa/go_auto_download"
MIRROR_REPO="congwa/update_allinone"

# 初始化变量
CUSTOM_MESSAGE=""
VERSION=""

# 获取当前最新版本
LATEST_VERSION=$(curl -s -H "Authorization: token ${GITHUB_TOKEN}" \
    -H "Accept: application/vnd.github.v3+json" \
    "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')

# 如果获取失败，使用默认版本
if [ -z "$LATEST_VERSION" ]; then
    LATEST_VERSION="1.0.0"
fi

# 解析命令行参数
while getopts "m:v:" opt; do
    case $opt in
        m)
            CUSTOM_MESSAGE="$OPTARG"
            ;;
        v)
            VERSION="$OPTARG"
            ;;
        \?)
            echo "无效的选项: -$OPTARG" >&2
            exit 1
            ;;
    esac
done

# 如果没有指定版本，则自动增加小版本号
if [ -z "$VERSION" ]; then
    # 分割版本号
    IFS='.' read -r major minor patch <<< "$LATEST_VERSION"
    # 增加小版本号
    patch=$((patch + 1))
    VERSION="${major}.${minor}.${patch}"
fi

echo "使用版本号: v${VERSION}"

# 检查版本是否已存在
echo "检查版本 v${VERSION} 是否已存在..."
EXISTING_RELEASE=$(curl -s -H "Authorization: token ${GITHUB_TOKEN}" \
    -H "Accept: application/vnd.github.v3+json" \
    "https://api.github.com/repos/${GITHUB_REPO}/releases/tags/v${VERSION}")

if [[ $(echo "$EXISTING_RELEASE" | grep -c "\"tag_name\"") -gt 0 ]]; then
    echo "错误: 版本 v${VERSION} 已经存在！"
    echo "请修改 VERSION 后重试"
    exit 1
fi

# 清理并重新创建 build 目录
echo "清理构建目录..."
if [ -d "build" ]; then
    rm -rf build/* # 只清空内容，保留目录
else
    mkdir -p build
fi

echo "开始编译 v${VERSION}"
echo "=========================="

# Linux AMD64 和 ARM64
echo "编译 Linux 版本..."
GOOS=linux GOARCH=amd64 go build -o build/download_allinone_linux_amd64_${VERSION} main.go
GOOS=linux GOARCH=arm64 go build -o build/download_allinone_linux_arm64_${VERSION} main.go
GOOS=linux GOARCH=arm go build -o build/download_allinone_linux_arm_${VERSION} main.go

# macOS (Darwin)
echo "编译 macOS 版本..."
GOOS=darwin GOARCH=amd64 go build -o build/download_allinone_darwin_amd64_${VERSION} main.go
GOOS=darwin GOARCH=arm64 go build -o build/download_allinone_darwin_arm64_${VERSION} main.go

echo "=========================="
echo "编译完成！文件位于 build 目录"
echo "编译的文件列表："
ls -lh build/

# 计算文件的 SHA256 哈希值并保存到文件
echo "=========================="
echo "生成 SHA256 哈希值..."
echo "# SHA256 Checksums" > build/SHA256SUMS.txt
cd build
# 检测系统类型并使用适当的命令
if command -v sha256sum >/dev/null 2>&1; then
    # Linux 系统使用 sha256sum
    for file in *; do
        if [ -f "$file" ] && [ "$file" != "SHA256SUMS.txt" ]; then
            sha256sum "$file" >> SHA256SUMS.txt
        fi
    done
else
    # macOS 系统使用 shasum -a 256
    for file in *; do
        if [ -f "$file" ] && [ "$file" != "SHA256SUMS.txt" ]; then
            shasum -a 256 "$file" >> SHA256SUMS.txt
        fi
    done
fi
cd ..

# 创建发布包
echo "=========================="
echo "创建发布包..."
RELEASE_NAME="download_allinone_${VERSION}"  # 更新文件名
ZIP_NAME="${RELEASE_NAME}.zip"

# 如果存在旧的zip包，先删除
rm -f "${ZIP_NAME}"

# 创建zip包
cd build
zip -r "../${ZIP_NAME}" ./*
cd ..

echo "=========================="
echo "发布包创建完成："
ls -lh "${ZIP_NAME}"
echo "SHA256 (${ZIP_NAME}): $(if command -v sha256sum >/dev/null 2>&1; then sha256sum ${ZIP_NAME}; else shasum -a 256 ${ZIP_NAME}; fi | cut -d' ' -f1)"

# 创建 GitHub Release
echo "=========================="
echo "创建 GitHub Release..."

# 检查tag是否存在
if git rev-parse "v${VERSION}" >/dev/null 2>&1; then
    echo "警告: Tag v${VERSION} 已存在"
    read -p "是否继续? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# 创建 git tag
git tag -a "v${VERSION}" -m "Release v${VERSION}"
git push origin "v${VERSION}"

# 格式化日期时间
FORMATTED_DATE=$(date '+%Y-%m-%d %H:%M:%S %Z')

RELEASE_NOTES=$(cat << EOF
{
    "tag_name": "v${VERSION}",
    "name": "Release v${VERSION}",
    "body": "# Release v${VERSION}\n\n---\n\n## 更新内容\n\n${CUSTOM_MESSAGE}\n\n---\n\n## 构建信息\n\n- 发布时间：${FORMATTED_DATE}\n- SHA256 校验和：请参考压缩包中的 \`SHA256SUMS.txt\` 文件\n",
    "draft": false,
    "prerelease": false
}
EOF
)

# 使用 GitHub API 创建 release
RELEASE_RESPONSE=$(curl -H "Authorization: token ${GITHUB_TOKEN}" \
    -H "Accept: application/vnd.github.v3+json" \
    -X POST "https://api.github.com/repos/${GITHUB_REPO}/releases" \
    -d "${RELEASE_NOTES}")

# 修改这部分：使用 jq 来正确解析 JSON 响应
if command -v jq >/dev/null 2>&1; then
    RELEASE_ID=$(echo "${RELEASE_RESPONSE}" | jq -r .id)
else
    # 如果没有 jq，使用更可靠的 grep 和 sed 组合
    RELEASE_ID=$(echo "${RELEASE_RESPONSE}" | grep -o '"id": *[0-9]*' | head -n1 | grep -o '[0-9]*')
fi

if [ -z "${RELEASE_ID}" ] || [ "${RELEASE_ID}" = "null" ]; then
    echo "错误: 创建 Release 失败"
    echo "${RELEASE_RESPONSE}"
    exit 1
fi

# 上传 ZIP 文件
echo "上传发布包..."
curl -H "Authorization: token ${GITHUB_TOKEN}" \
    -H "Content-Type: application/zip" \
    -H "Accept: application/vnd.github.v3+json" \
    --data-binary @"${ZIP_NAME}" \
    "https://uploads.github.com/repos/${GITHUB_REPO}/releases/${RELEASE_ID}/assets?name=${ZIP_NAME}"

echo "=========================="
echo "发布完成！"
echo "请访问 https://github.com/${GITHUB_REPO}/releases/tag/v${VERSION} 查看"

# 在创建 Release 后添加以下代码
echo "=========================="
echo "同步 Release 到镜像仓库..."

# 检查镜像仓库是否为空
REPO_EMPTY=$(curl -s -H "Authorization: token ${GITHUB_TOKEN}" \
    -H "Accept: application/vnd.github.v3+json" \
    "https://api.github.com/repos/${MIRROR_REPO}/commits" | grep -c "Not Found")

if [ "$REPO_EMPTY" -eq 1 ]; then
    echo "镜像仓库为空，正在初始化..."
    
    # 创建临时目录
    TEMP_DIR=$(mktemp -d)
    cd "$TEMP_DIR"
    
    # 初始化仓库
    git init
    echo "# Update Mirror Repository" > README.md
    echo "This repository mirrors releases from ${GITHUB_REPO}" >> README.md
    git add README.md
    git commit -m "Initial commit"
    
    # 添加远程仓库
    git remote add origin "https://${GITHUB_TOKEN}@github.com/${MIRROR_REPO}.git"
    git branch -M main
    git push -u origin main
    
    # 返回原目录
    cd -
    rm -rf "$TEMP_DIR"
fi

# 继续创建 Release...
MIRROR_RELEASE_NOTES=$(cat << EOF
{
    "tag_name": "v${VERSION}",
    "name": "Release v${VERSION}",
    "body": "# Release v${VERSION}\n\n同步自 ${GITHUB_REPO}\n\n## 更新内容\n\n${CUSTOM_MESSAGE}\n\n---\n\n## 构建信息\n\n- 发布时间：${FORMATTED_DATE}\n- SHA256 校验和：请参考压缩包中的 \`SHA256SUMS.txt\` 文件\n",
    "draft": false,
    "prerelease": false
}
EOF
)

# 创建镜像仓库的 Release
MIRROR_RELEASE_RESPONSE=$(curl -H "Authorization: token ${GITHUB_TOKEN}" \
    -H "Accept: application/vnd.github.v3+json" \
    -X POST "https://api.github.com/repos/${MIRROR_REPO}/releases" \
    -d "${MIRROR_RELEASE_NOTES}")

# 解析镜像仓库的 Release ID
if command -v jq >/dev/null 2>&1; then
    MIRROR_RELEASE_ID=$(echo "${MIRROR_RELEASE_RESPONSE}" | jq -r .id)
else
    MIRROR_RELEASE_ID=$(echo "${MIRROR_RELEASE_RESPONSE}" | grep -o '"id": *[0-9]*' | head -n1 | grep -o '[0-9]*')
fi

if [ -n "${MIRROR_RELEASE_ID}" ] && [ "${MIRROR_RELEASE_ID}" != "null" ]; then
    # 上传 ZIP 文件到镜像仓库
    echo "上传发布包到镜像仓库..."
    curl -H "Authorization: token ${GITHUB_TOKEN}" \
        -H "Content-Type: application/zip" \
        -H "Accept: application/vnd.github.v3+json" \
        --data-binary @"${ZIP_NAME}" \
        "https://uploads.github.com/repos/${MIRROR_REPO}/releases/${MIRROR_RELEASE_ID}/assets?name=${ZIP_NAME}"
    
    echo "镜像仓库发布完成！"
    echo "请访问 https://github.com/${MIRROR_REPO}/releases/tag/v${VERSION} 查看"
else
    echo "警告: 镜像仓库 Release 创建失败"
    echo "${MIRROR_RELEASE_RESPONSE}"
fi