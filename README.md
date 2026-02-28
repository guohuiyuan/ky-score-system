<div align="center">

# 🎓 考研分数统计与实时排行榜系统 (KY Score System)

基于 Go 语言和动态配置构建的高性能、极简、开箱即用的考研估分与志愿填报辅助系统。

[![Go Version](https://img.shields.io/github/go-mod/go-version/guohuiyuan/ky-score-system)](https://golang.org/)
[![License](https://img.shields.io/github/license/guohuiyuan/ky-score-system)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/guohuiyuan/ky-score-system)](https://hub.docker.com/r/guohuiyuan/ky-score-system)
[![Build Status](https://github.com/guohuiyuan/ky-score-system/actions/workflows/docker.yml/badge.svg)](https://github.com/guohuiyuan/ky-score-system/actions)

</div>

---

## 🌟 核心理念与特性

**KY Score System** 专为各大高校考研复试阶段的“民间分数线统计”而生。我们去除了所有冗余的操作逻辑，用最直观的方式呈现“成绩排名”和“方向筛选”，帮助考生更准确地定位自己的竞争环境。

### 为什么选择本项目？

- **🎨 优雅的现代化 UI**：全面重构的“哈工大蓝 (HIT Blue)”主题界面，响应式设计完美适配手机、平板及桌面端。
- **⚙️ 全动态表单配置**：考试科目、报考方向统统不需要改动代码！仅需修改 `config.json` 即可动态生成公共提交页面与筛选条件。
- **📊 实时智能排名**：抛弃了老旧的前端表格排序，采用纯后端动态计算与过滤，支持“最低分过滤 (min_score)”，筛选后名次自动从 1 开始实时重排。
- **🔒 安全与隐私保护**：
    - 管理员密码由 SQLite 安全托管，首次登录**强制修改密码**。
    - 考生录入成绩时自动分配**防篡改专属密钥**，后续更新成绩必须凭钥验证。
    - 准考证号+身份证号双重唯一性校验，内置 IP 频率监控防灌水。
- **🧑‍💼 高效后台核验**：支持可视化审核截图证明，支持状态搜索、批量通过/驳回、一键删除，更内置了 **Excel 数据的导入与导出**。
- **🐳 极简部署体验**：纯静态的跨平台单文件应用（内嵌 SQLite），也提供完善的 Docker 镜像，只需一条命令即可上线。

---

## 🛠️ 技术栈

本项目坚持轻量化与高性能：

- **后端**：Go 1.25 / [Gin](https://gin-gonic.com/) 高性能 Web 框架
- **持久层**：[GORM](https://gorm.io/) / 内置 SQLite 引擎（零外部数据库依赖）
- **前端**：Go HTML Templates / 原生 Bootstrap 5 / 原生 Vanilla JS，抛弃沉重的库（如 jQuery / Datatables）
- **CI / CD**：GitHub Actions + GoReleaser 自动化跨平台构建发布

---

## 🚀 快速上手 (Quick Start)

### 选项 A：使用 Docker 进行部署（推荐方案）

只需在服务器上准备一个 `docker-compose.yml`：

```yaml
services:
  ky-score-system:
    image: guohuiyuan/ky-score-system:latest
    container_name: ky-score-system
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./data:/home/appuser/data
    environment:
      - TZ=Asia/Shanghai
```

然后执行启动命令：

```bash
docker-compose up -d
```
> **提示**：首次启动后，系统会自动在 `./data` 目录中生成默认数据库 `score.db` 以及配置文件 `config.json`。默认的管理员账号为 `admin / admin123`，**请务必在登录后台后第一时间修改密码**。

### 选项 B：从源码手动运行

1. 克隆代码库：
   ```bash
   git clone https://github.com/guohuiyuan/ky-score-system.git
   cd ky-score-system
   ```
2. 启动服务（会自动下载依赖包并执行）：
   ```bash
   go run .
   ```
3. 在浏览器访问：[http://127.0.0.1:8080/ky/](http://127.0.0.1:8080/ky/)

---

## ⚙️ 配置文件说明 (`config.json`)

本系统极具弹性的核心就在于配置文件。在运行前，你可以参考项目中的 `data/config.example.json` 了解结构。

```json
{
  "exam_name": "哈工大计算机考研 (HIT CS)",
  "fields": [
    {"Key": "politics", "Label": "政治", "Type": "number"},
    {"Key": "english", "Label": "英语一", "Type": "number"},
    {"Key": "math", "Label": "数学一", "Type": "number"},
    {"Key": "subject_408", "Label": "408/854", "Type": "number"},
    {"Key": "undergrad", "Label": "本科层次", "Type": "select", "Options": ["985", "211", "双非"]},
    {
      "Key": "direction",
      "Label": "方向",
      "Type": "select",
      "Options": [
        "【本部】计算学部/未来技术学院-计算机科学与技术(学硕)",
        "【深圳】计算机方向"
      ],
      "Alias": {
        "【本部】计算学部/未来技术学院-计算机科学与技术(学硕)": "本部·计科学硕",
        "【深圳】计算机方向": "深圳·计科"
      }
    }
  ],
  "key_recovery_fields": ["politics", "english", "math", "subject_408"]
}
```

- **`fields` 数组**：配置了收集成绩的字段。支持 `number`（自动计算总分）与 `select`（生成前端下拉菜单，并在后台用作分类标签）。
- **`Alias` (别名系统)**：如果某个选项名字过于冗长（如“计算机科学与技术(学硕)”），通过配置别名，它可以以“本部·计科学硕”的形式清爽地在前端展示，但后端存储依旧保存全名。

---

## 📖 页面速览

- **公开页面**
  - **🏆 实时排行榜**：`/ky/` (主页，输入并递交成绩立即见榜)
  - **✍️ 提交/修改分数**：`/ky/submit`
  - **🔑 密钥验证**：`/ky/login` (仅持有有效防篡改密钥的用户才可以修改自己提交的数据)
- **管理后台**
  - **🛡️ 登录**：`/ky/admin/` (初始凭据 `admin / admin123`)
  - **📋 审核台**：`/ky/admin/dashboard`
  - **🔑 重置密码**：`/ky/admin/change-password`

---

## 💻 参与贡献 (Contributing)

我们非常欢迎任何形式的贡献——无论是一个发现 Bug 的 Issue，还是一个优化了 UI 或增加新功能的 Pull Request。

1. **Fork** 本仓库
2. 新建您的特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交您的修改记录 (`git commit -m 'feat: Add some AmazingFeature'`)
4. 推送代码分支 (`git push origin feature/AmazingFeature`)
5. 开启您的拉取请求 (**Pull Request**) 🚀

> 若提供 PR 代码变动，请通过了内部集成的 Github Actions (`golangci-lint` 及 编译测试)。

---

## 📄 开源许可证 (License)

本项目遵循 **AGPL-3.0 License** 开源协议 - 有关详细信息，请参阅随附的 [LICENSE](LICENSE) 文件。
