# LuCICodex - OpenWrt 自然语言助手

<p align="center">
  <img src="assets/logo.png" alt="LuciCodex Logo" width="240">
</p>

**用简单的中文命令控制你的 OpenWrt 路由器**

作者: AZ <Aezi.zhu@icloud.com>

<p align="center">
  <a href="#"><img alt="Build" src="https://img.shields.io/badge/build-passing-brightgreen"></a>
  <a href="#license"><img alt="License" src="https://img.shields.io/badge/license-Dual-blue"></a>
  <a href="#"><img alt="Go Version" src="https://img.shields.io/badge/Go-1.21+-1f425f"></a>
  <a href="#"><img alt="OpenWrt" src="https://img.shields.io/badge/OpenWrt-supported-00a0e9"></a>
  <a href="https://github.com/aezizhu/LuciCodex/actions/workflows/build.yml"><img alt="CI" src="https://github.com/aezizhu/LuciCodex/actions/workflows/build.yml/badge.svg"></a>
</p>

**[English](README.md)** | **[中文](README.zh-CN.md)** | **[Español](README.es.md)**

---

## 什么是 LuCICodex？

**LuCICodex** 是一个智能助手，让你可以用自然语言管理 OpenWrt 路由器，而不需要记忆复杂的命令。只需用简单的中文告诉 LuCICodex 你想做什么，它就会将你的请求转换成安全的、经过审核的命令，供你在执行前查看。

**例如：** 不需要记住 `uci set wireless.radio0.disabled=0 && uci commit wireless && wifi reload`，只需说：*"打开 WiFi"*

---

## 目录

- [为什么使用 LuCICodex？](#为什么使用-lucicodex)
- [快速开始](#快速开始)
  - [系统要求](#系统要求)
  - [在 OpenWrt 上安装](#在-openwrt-上安装)
  - [获取 API 密钥](#获取-api-密钥)
- [在路由器上使用 LuCICodex](#在路由器上使用-lucicodex)
  - [方法 1：网页界面（推荐）](#方法-1网页界面推荐)
  - [方法 2：命令行（SSH）](#方法-2命令行ssh)
- [配置指南](#配置指南)
  - [选择 AI 服务商](#选择-ai-服务商)
  - [通过网页界面配置](#通过网页界面配置)
  - [通过命令行配置](#通过命令行配置)
- [常见用例](#常见用例)
- [安全特性](#安全特性)
- [故障排除](#故障排除)
- [高级用法](#高级用法)
- [许可证](#许可证)
- [支持](#支持)

---

## 为什么使用 LuCICodex？

### 适合家庭用户
- **无需记忆命令**：用简单的中文管理路由器
- **默认安全**：所有命令在执行前都会被审查
- **简单的网页界面**：无需 SSH 连接到路由器
- **边学边用**：查看 LuciCodex 生成的实际命令

### 适合高级用户
- **更快的管理**：自然语言比查找语法更快
- **基于策略的安全性**：自定义允许的命令
- **多个 AI 服务商**：可选择 Gemini、OpenAI 或 Anthropic
- **审计日志**：完整记录所有操作

---

## 快速开始

### 系统要求

安装 LuciCodex 前，你需要：

1. **OpenWrt 路由器**（建议版本 21.02 或更高）
2. 路由器有**互联网连接**
3. 至少 **10MB 可用存储**空间
4. 从以下服务商之一获取 **API 密钥**：
   - Google Gemini（推荐新手使用 - 有免费额度，默认 `gemini-3`）
   - OpenAI（GPT-5.1/GPT-4o）
   - Anthropic（Claude 4.5）

### 在 OpenWrt 上安装

#### 步骤 1：下载软件包

通过 SSH 连接到路由器，下载适合你架构的 LuCICodex 软件包：

```bash
# 适用于 MIPS 路由器（最常见）
cd /tmp
wget https://github.com/aezizhu/LuciCodex/releases/latest/download/lucicodex-mips.ipk

# 适用于 ARM 路由器
wget https://github.com/aezizhu/LuciCodex/releases/latest/download/lucicodex-arm.ipk

# 适用于 ARM64 (aarch64) 路由器 - 标准 OpenWrt
wget https://github.com/aezizhu/LuciCodex/releases/latest/download/lucicodex-arm64.ipk

# 适用于 x86_64 路由器
wget https://github.com/aezizhu/LuciCodex/releases/latest/download/lucicodex-amd64.ipk
```

#### 步骤 2：安装软件包

```bash
opkg update
opkg install /tmp/lucicodex-*.ipk
```

#### 步骤 3：安装网页界面（可选但推荐）

```bash
opkg install luci-app-lucicodex
```

#### 步骤 4：验证安装

```bash
lucicodex -version
```

你应该看到：`LuciCodex version 0.4.10`

### 获取 API 密钥

#### 选项 1：Google Gemini（推荐新手）

1. 访问 https://makersuite.google.com/app/apikey
2. 点击 "Create API Key"
3. 复制你的 API 密钥（以 `AIza...` 开头）
4. **免费额度**：每分钟 60 次请求

#### 选项 2：OpenAI

1. 访问 https://platform.openai.com/api-keys
2. 点击 "Create new secret key"
3. 复制你的 API 密钥（以 `sk-...` 开头）
4. **注意**：需要绑定支付方式

#### 选项 3：Anthropic

1. 访问 https://console.anthropic.com/settings/keys
2. 点击 "Create Key"
3. 复制你的 API 密钥（以 `sk-ant-...` 开头）
4. **注意**：需要绑定支付方式

---

## 在路由器上使用 LuCICodex

### 方法 1：网页界面（推荐）

这是使用 LuciCodex 最简单的方式，特别是如果你不熟悉命令行。

#### 步骤 1：访问网页界面

1. 打开路由器的网页界面（通常是 http://192.168.1.1）
2. 用管理员账号登录
3. 导航到 **系统 → LuCICodex**

#### 步骤 2：配置 API 密钥

1. 点击 **配置** 标签
2. 选择你的 AI 服务商（Gemini、OpenAI 或 Anthropic）
3. 在相应字段中输入你的 API 密钥
4. 点击 **保存并应用**

#### 步骤 3：使用助手

1. 点击 **运行** 标签
2. 用中文输入你的请求，例如：
   - "显示当前网络配置"
   - "重启 WiFi"
   - "为我的 Web 服务器开放 8080 端口"
3. 点击 **生成计划**
4. 查看 LuciCodex 建议的命令
5. 如果命令正确，点击 **执行命令**

**就这样！** 你现在可以用自然语言控制路由器了。

### 方法 2：命令行（SSH）

如果你喜欢使用 SSH 或想自动化任务，可以从命令行使用 LuciCodex。

#### 步骤 1：配置 API 密钥

```bash
# 设置 Gemini API 密钥
uci set lucicodex.@api[0].provider='gemini'
uci set lucicodex.@api[0].key='你的-API-密钥'
uci commit lucicodex
```

#### 步骤 2：试运行测试

```bash
lucicodex "显示 WiFi 状态"
```

这将显示 LuciCodex 会运行的命令，但不会实际执行。

#### 步骤 3：执行命令

如果命令看起来正确，使用批准模式运行：

```bash
lucicodex -approve "重启 WiFi"
```

或使用交互模式逐个确认命令：

```bash
lucicodex -confirm-each "更新软件包列表并安装 htop"
```

---

## 配置指南

### 选择 AI 服务商

LuciCodex 支持多个 AI 服务商。选择方法如下：

| 服务商 | 适合 | 费用 | 速度 | 所需 API 密钥 |
|--------|------|------|------|---------------|
| **Gemini** | 新手、家庭用户 | 有免费额度 | 快 | GEMINI_API_KEY 或 lucicodex.@api[0].key |
| **OpenAI** | 高级用户、复杂任务 | 按使用付费 | 非常快 | OPENAI_API_KEY 或 lucicodex.@api[0].openai_key |
| **Anthropic** | 注重隐私的用户 | 按使用付费 | 快 | ANTHROPIC_API_KEY 或 lucicodex.@api[0].anthropic_key |
| **Gemini CLI** | 离线/本地使用 | 免费（本地） | 不定 | 外部 gemini 可执行文件路径 |

**注意：** 每个服务商需要自己专用的 API 密钥。你只需配置正在使用的服务商的密钥。

### 通过网页界面配置

1. 前往 **系统 → LuCICodex → 配置**
2. 配置以下设置：

**API 设置：**
- **服务商**：选择你的 AI 服务商
- **API 密钥**：输入你的密钥（安全存储）
- **模型**：保持空白使用默认值，或指定（如 `gemini-3`、`gpt-5.1`、`claude-4.5`）
- **端点**：除非使用自定义端点，否则保持默认

**安全设置：**
- **默认试运行**：保持启用（推荐） - 在运行前显示命令
- **逐个确认命令**：启用以获得额外安全性
- **命令超时**：每个命令的等待时间（默认：30 秒）
- **最大命令数**：每次请求的最大命令数（默认：10）
- **日志文件**：保存执行日志的位置（默认：`/tmp/lucicodex.log`）

3. 点击 **保存并应用**

### 通过命令行配置

所有设置都使用 OpenWrt 的 UCI 系统存储在 `/etc/config/lucicodex`：

```bash
# 配置 Gemini
uci set lucicodex.@api[0].provider='gemini'
uci set lucicodex.@api[0].key='你的-GEMINI-密钥'
uci set lucicodex.@api[0].model='gemini-3'

# 配置 OpenAI
uci set lucicodex.@api[0].provider='openai'
uci set lucicodex.@api[0].openai_key='你的-OPENAI-密钥'
uci set lucicodex.@api[0].model='gpt-5.1'

# 配置 Anthropic
uci set lucicodex.@api[0].provider='anthropic'
uci set lucicodex.@api[0].anthropic_key='你的-ANTHROPIC-密钥'
uci set lucicodex.@api[0].model='claude-4.5'

# 安全设置
uci set lucicodex.@settings[0].dry_run='1'          # 1=启用, 0=禁用
uci set lucicodex.@settings[0].confirm_each='0'     # 1=逐个确认, 0=一次确认
uci set lucicodex.@settings[0].timeout='30'         # 秒
uci set lucicodex.@settings[0].max_commands='10'    # 每次请求的最大命令数

# 应用更改
uci commit lucicodex
```

---

## 常见用例

### 网络管理

```bash
# 检查网络状态
lucicodex "显示所有网络接口及其状态"

# 重启网络
lucicodex -approve "重启网络"

# 配置静态 IP
lucicodex "将 lan 接口设置为静态 IP 192.168.1.1"
```

### WiFi 管理

```bash
# 检查 WiFi 状态
lucicodex "显示 WiFi 状态"

# 更改 WiFi 密码
lucicodex "将 WiFi 密码更改为 MyNewPassword123"

# 启用/禁用 WiFi
lucicodex -approve "关闭 WiFi"
lucicodex -approve "打开 WiFi"

# 重启 WiFi
lucicodex -approve "重启 WiFi"
```

### 防火墙管理

```bash
# 检查防火墙规则
lucicodex "显示当前防火墙规则"

# 开放端口
lucicodex "为来自 lan 的 tcp 流量开放 8080 端口"

# 阻止 IP
lucicodex "阻止 IP 地址 192.168.1.100"
```

### 软件包管理

```bash
# 更新软件包列表
lucicodex "更新软件包列表"

# 安装软件包
lucicodex "安装 htop 软件包"

# 列出已安装的软件包
lucicodex "显示所有已安装的软件包"
```

### 系统监控

```bash
# 检查系统状态
lucicodex "显示系统信息和运行时间"

# 检查内存使用
lucicodex "显示内存使用情况"

# 检查磁盘空间
lucicodex "显示磁盘空间使用情况"

# 查看系统日志
lucicodex "显示系统日志的最后 20 行"
```

### 诊断

```bash
# Ping 测试
lucicodex "ping google.com 5 次"

# DNS 测试
lucicodex "检查 DNS 是否正常工作"

# 检查互联网连接
lucicodex "测试互联网连接"
```

---

## 安全特性

LuCICodex 以安全为最高优先级设计：

### 1. 试运行模式（默认）
默认情况下，LuciCodex 会显示它将要做什么，而不实际执行。你必须明确批准执行。

### 2. 命令审查
每个命令在执行前都会显示给你。你可以准确看到将在系统上运行什么。

### 3. 策略引擎
LuCICodex 内置了关于允许哪些命令的规则：

**默认允许：**
- `uci`（配置）
- `ubus`（系统总线）
- `fw4`（防火墙）
- `opkg`（软件包管理器）
- `ip`、`ifconfig`（网络信息）
- `cat`、`grep`、`tail`（读取文件）
- `logread`、`dmesg`（日志）

**默认阻止：**
- `rm -rf /`（危险删除）
- `mkfs`（文件系统格式化）
- `dd`（磁盘操作）
- Fork 炸弹和其他恶意模式

### 4. 无 Shell 执行
LuCICodex 从不使用 shell 扩展或管道。命令以精确参数直接执行，防止注入攻击。

### 5. 执行锁定
一次只能运行一个 LuciCodex 命令，防止冲突和竞态条件。CLI 使用 `/var/lock/lucicodex.lock`（或备用的 `/tmp/lucicodex.lock`）锁文件来确保独占执行。

### 6. 超时
每个命令都有超时（默认 30 秒）以防止挂起。

### 7. 审计日志
所有命令及其结果都记录到 `/tmp/lucicodex.log` 供审查。

---

## 故障排除

### "API key not configured"（API 密钥未配置）

**解决方案：** 确保你已设置 API 密钥：

```bash
# 通过 UCI
uci set lucicodex.@api[0].key='你的-密钥'
uci commit lucicodex

# 或通过环境变量
export GEMINI_API_KEY='你的-密钥'
```

### "execution in progress"（正在执行）

**解决方案：** 另一个 LuciCodex 命令正在运行。等待完成，或删除陈旧的锁文件：

```bash
rm /var/lock/lucicodex.lock
# 或如果使用备用位置：
rm /tmp/lucicodex.lock
```

### "command not found: lucicodex"（未找到命令：lucicodex）

**解决方案：** 确保 lucicodex 已安装并在 PATH 中：

```bash
which lucicodex
# 应显示：/usr/bin/lucicodex

# 如果未找到，重新安装：
opkg update
opkg install lucicodex
```

### 命令未执行

**解决方案：** 确保你不在试运行模式：

```bash
# 使用 -approve 标志
lucicodex -approve "你的命令"

# 或在配置中禁用试运行
uci set lucicodex.@settings[0].dry_run='0'
uci commit lucicodex
```

### "prompt too long (max 4096 chars)"（提示过长）

**解决方案：** 你的请求太长。将其分解为更小的请求或更简洁。

### 网页界面未显示

**解决方案：** 确保 luci-app-lucicodex 已安装：

```bash
opkg update
opkg install luci-app-lucicodex
/etc/init.d/uhttpd restart
```

然后清除浏览器缓存并重新加载。

---

## 高级用法

### 交互模式（REPL）

启动交互式会话，与 LuciCodex 对话：

```bash
lucicodex -interactive
```

### JSON 输出

获取结构化输出用于脚本编写：

```bash
lucicodex -json "显示网络状态" | jq .
```

### 自定义配置文件

使用自定义配置文件而不是 UCI：

```bash
lucicodex -config /etc/lucicodex/custom-config.json "你的命令"
```

### 环境变量

使用环境变量覆盖设置：

```bash
export GEMINI_API_KEY='你的密钥'
export LUCICODEX_PROVIDER='gemini'
export LUCICODEX_MODEL='gemini-3'
lucicodex "你的命令"
```

### 命令行标志

```bash
lucicodex -help
```

可用标志：
- `-approve`：无需确认自动批准计划
- `-dry-run`：仅显示计划，不执行（默认：true）
- `-confirm-each`：逐个确认每个命令
- `-json`：以 JSON 格式输出
- `-interactive`：启动交互式 REPL 模式
- `-timeout=30`：设置命令超时（秒）
- `-max-commands=10`：设置每次请求的最大命令数
- `-model=name`：覆盖模型名称
- `-config=path`：使用自定义配置文件
- `-log-file=path`：设置日志文件路径
- `-facts=true`：在提示中包含环境信息（默认：true）
- `-join-args`：将所有参数连接成单个提示（实验性）
- `-version`：显示版本

**关于提示处理的注意事项：** 默认情况下，LuciCodex 仅使用第一个参数作为提示。如果需要在不使用引号的情况下传递多词提示，请使用 `-join-args` 标志：

```bash
# 默认行为（推荐）
lucicodex "显示 WiFi 状态"

# 使用 -join-args 标志（实验性）
lucicodex -join-args 显示 WiFi 状态
```

### 自定义策略

编辑 `/etc/config/lucicodex` 或配置文件中的白名单和黑名单：

```json
{
  "allowlist": [
    "^uci(\\s|$)",
    "^自定义命令(\\s|$)"
  ],
  "denylist": [
    "^危险命令(\\s|$)"
  ]
}
```

---

## 许可证

**双重许可：**

- **个人/非商业使用免费** - 在家用路由器上免费使用 LuciCodex
- **商业使用需要许可** - 联系 aezi.zhu@icloud.com 获取商业许可

详见 [LICENSE](LICENSE) 文件。

---

## 支持

### 获取帮助

- **文档**：你正在阅读！
- **问题**：https://github.com/aezizhu/LuciCodex/issues
- **讨论**：https://github.com/aezizhu/LuciCodex/discussions

### 商业支持

商业许可、企业支持或定制开发：
- 邮箱：Aezi.zhu@icloud.com
- 主题请包含 "LuciCodex Commercial License"

### 贡献

欢迎贡献！提交拉取请求前请阅读我们的贡献指南。

---

## 关于本项目

**LuciCodex** 旨在让每个人都能管理 OpenWrt 路由器，而不仅仅是网络专家。通过结合现代 AI 的强大功能和严格的安全控制，LuciCodex 让你可以用自然语言管理路由器，同时保持安全性和透明度。

该项目首先专注于 OpenWrt，采用服务商无关设计和强大的安全默认设置。每个命令都经过审核，每个操作都被记录，你始终掌控一切。

---

**用 ❤️ 为 OpenWrt 社区打造**
