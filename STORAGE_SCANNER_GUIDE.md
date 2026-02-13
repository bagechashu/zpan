# ZPan 项目指南 - file_path 作用与存储扫描接口

## 📋 内容总览

1. **file_path 作用解释**
2. **新增存储扫描功能**
3. **API 使用示例**
4. **实现细节**

---

## 一、file_path 的作用与细节

### 1.1 核心概念

`file_path` 是存储器（Storage）配置中的一个参数，**定义了上传文件在对象存储（云存储）里的存储路径规则**。

### 1.2 工作流程

```
用户上传文件: "documents/resume.pdf"
    ↓
处理阶段：Matter.BuildObject(rootPath, filePath)
    ↓
变量替换：
  - $RAW_PATH → "documents/" (原始上传路径)
  - $RAW_NAME → "resume.pdf" (原始文件名)
  - $NOW_DATE → "20260213" (当前日期)
  - $RAND_16KEY → "mCUoR35rxyZabc12" (随机字符)
    ↓
最终对象路径: "rootPath/documents/resume.pdf" 或其他规则生成的路径
    ↓
存储到云存储（S3/OSS/COS等）
```

### 1.3 支持的变量

在 [internal/app/entity/matter_env.go](internal/app/entity/matter_env.go) 中定义了以下变量：

| 变量 | 说明 | 示例 |
|------|------|------|
| $UID | 用户ID | 10001 |
| $UUID | UUID码 | 6ba7b810-9dad-11d1-80b4-00c04fd430c8 |
| $RAW_PATH | 原始上传路径 | 文稿/简历 |
| $RAW_NAME | 原始文件名 | 张三-简历 |
| $RAW_EXT | 原始文件后缀 | pdf |
| $RAND_8KEY | 8位随机字符 | mCUoR35r |
| $RAND_16KEY | 16位随机字符 | e1CbDUNfyVP3sScJ |
| $NOW_DATE | 当前日期 | 20210101 |
| $NOW_YEAR | 当前年 | 2021 |
| $NOW_MONTH | 当前月 | 01 |
| $NOW_DAY | 当前日 | 01 |
| $NOW_HOUR | 当前时 | 12 |
| $NOW_MIN | 当前分 | 30 |
| $NOW_SEC | 当前秒 | 10 |
| $NOW_UNIX | 时间戳 | 1612631185 |

### 1.4 常用配置示例

#### 示例1：按日期组织（默认）

```yaml
file_path: "$NOW_DATE/$RAND_16KEY.$RAW_EXT"
```

**结果**：同一天上传的文件存在同一个文件夹
```
2021-01-01/abc123def456.pdf
2021-01-01/xyz789uvw012.jpg
2021-01-02/aaa111bbb222.txt
```

#### 示例2：保持原始路径（share.all_files场景）

```yaml
file_path: "$RAW_PATH$RAW_NAME"
```

**结果**：保留用户上传时的目录结构
```
documents/resume.pdf
documents/cover_letter.docx
projects/proposal.pptx
```

#### 示例3：用户隔离 + 日期

```yaml
file_path: "user/$UID/$NOW_YEAR/$NOW_MONTH/$RAW_NAME"
```

**结果**：不同用户的文件完全隔离
```
user/10001/2021/01/resume.pdf
user/10002/2021/01/resume.pdf
```

---

## 二、新增存储扫描功能

### 2.1 功能概述

当启用 `share.all_files = true` 时，有些场景下需要将对象存储中**已经存在的文件**导入到 ZPan 数据库中，使其显示在文件浏览器中。

新增的 **StorageScanner** 就是为了解决这个问题：

- ✅ 扫描云存储中的所有文件对象
- ✅ 根据 `file_path` 规则解析文件路径
- ✅ 自动创建缺失的目录结构
- ✅ 在数据库中创建 Matter 记录
- ✅ 避免重复导入

### 2.2 API 端点

**POST** `/api/storages/{id}/scan`

扫描指定存储中的对象并导入为 Matter 记录。

#### 请求示例

```bash
# 扫描整个存储
curl -X POST http://localhost:8222/api/storages/1/scan \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{}'

# 扫描特定前缀
curl -X POST http://localhost:8222/api/storages/1/scan \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "prefix": "documents/"
  }'
```

#### 响应示例

```json
{
  "total": 150,        // 扫描到的总对象数
  "created": 145,      // 成功创建的 Matter 数
  "skipped": 4,        // 跳过的（已存在）
  "failed": 1,         // 失败的
  "errors": [
    "failed to create matter docs/subdir/file.pdf: base dir not exist"
  ]
}
```

### 2.3 权限要求

- **需要管理员权限** (OAuth2: admin)
- 建议存储主数据之前运行扫描，避免混乱

---

## 三、实现细节

### 3.1 关键文件

| 文件 | 功能 |
|------|------|
| [internal/app/usecase/storage/scanner.go](internal/app/usecase/storage/scanner.go) | StorageScanner 实现 |
| [internal/pkg/bind/storage.go](internal/pkg/bind/storage.go) | 请求/响应绑定结构 |
| [internal/app/api/storage.go](internal/app/api/storage.go) | API 端点实现 |
| [internal/app/wire_gen.go](internal/app/wire_gen.go) | 依赖注入配置 |

### 3.2 核心逻辑流程

```go
ScanStorageObjects(uid, sid, prefix)
  ├─ 1. 获取存储配置
  ├─ 2. 获取对应的 Provider（S3/OSS等）
  ├─ 3. 列出对象 provider.List(prefix)
  ├─ 4. 遍历每个对象：
  │  ├─ 解析对象路径 → 提取目录和文件名
  │  ├─ 检查是否已存在
  │  ├─ 确保父目录存在（自动创建）
  │  └─ 创建 Matter 记录
  └─ 5. 返回统计结果
```

### 3.3 目录自动创建

当发现父目录不存在时，扫描器会**递归创建**所有缺失的目录：

```
对象: "docs/2021/projects/proposal.pdf"

如果只有 "/" 存在，会依次创建：
  /docs/          ✓ 创建
  /docs/2021/     ✓ 创建
  /docs/2021/projects/  ✓ 创建
  (然后创建文件)
```

### 3.4 类型检测

扫描器根据文件扩展名自动检测 MIME 类型（PDF、图片、视频等），存储在 Matter 的 `Type` 字段。

---

## 四、使用建议

### 4.1 最佳实践

1. **在初始化阶段运行**
   ```yaml
   # 配置示例
   share:
     all_files: true    # 启用全局文件共享
   ```
   然后运行扫描接口导入现有文件

2. **配合特定的 file_path**
   ```yaml
   file_path: "$RAW_PATH$RAW_NAME"  # 保留原始路径
   ```

3. **权限控制**
   - 仅管理员可调用扫描接口
   - 建议在安装或迁移阶段使用

### 4.2 故障排查

| 问题 | 解决方案 |
|------|---------|
| `failed: base dir not exist` | 扫描器应该自动创建，如果仍然失败，检查目录名中是否有特殊字符 |
| `skipped` 数量过多 | 可能已经导入过，检查数据库中是否已有相同文件 |
| 性能缓慢 | 存储中文件过多时，可以使用 `prefix` 参数分批导入 |

---

## 五、示例代码

### 使用整个流程

```bash
# 1. 创建存储
POST /api/storages
{
  "name": "backup-oss",
  "bucket": "my-bucket",
  "provider": "OSS",
  "endpoint": "https://oss-cn-hangzhou.aliyuncs.com",
  "access_key": "xxx",
  "secret_key": "yyy",
  "file_path": "$RAW_PATH$RAW_NAME"
}

# 2. 扫描存储中的文件
POST /api/storages/1/scan
{
  "prefix": ""
}

# 3. 查看结果
GET /api/matters?sid=1
```

### 分批导入

```bash
# 按前缀批量导入，避免超时
POST /api/storages/1/scan {"prefix": "documents/"}
POST /api/storages/1/scan {"prefix": "media/"}
POST /api/storages/1/scan {"prefix": "archives/"}
```

---

## 六、架构图

```
┌─────────────────────────────────────────────┐
│          Cloud Storage (S3/OSS/COS)         │
│                                             │
│  file1.pdf                                  │
│  /documents/resume.pdf                      │
│  /projects/2021/proposal.pptx               │
└──────────────────────┬──────────────────────┘
                       │
                       │ List Objects
                       ↓
        ┌──────────────────────────┐
        │ StorageScanner.Scan()    │
        │ (解析路径、创建目录)      │
        └──────────────────────────┘
                       │
                       │ Create Matter
                       ↓
        ┌──────────────────────────┐
        │   zp_matter (数据库)     │
        │  - id, name, parent      │
        │  - object, type, size    │
        └──────────────────────────┘
                       │
                       │ Display
                       ↓
        ┌──────────────────────────┐
        │   File Explorer UI       │
        │   (用户可见文件)          │
        └──────────────────────────┘
```

---

**祝你使用愉快！** 🎉
