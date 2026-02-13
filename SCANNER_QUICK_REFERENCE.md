# 📝 Storage Scanner - Quick Reference

## 新增接口

```
POST /api/storages/{storageId}/scan
```

### 请求体

```json
{
  "prefix": ""  // 可选，扫描特定前缀
}
```

### 响应

```json
{
  "total": 100,      // 扫描到的总对象数
  "created": 95,     // 新创建的 Matter 数
  "skipped": 4,      // 已存在的（跳过）
  "failed": 1,       // 失败的
  "errors": [...]    // 错误信息列表
}
```

---

## file_path 快速参考

### 默认配置（按日期）
```yaml
file_path: "$NOW_DATE/$RAND_16KEY.$RAW_EXT"
# 结果：20210101/abc123def456.pdf
```

### 保留原始路径
```yaml
file_path: "$RAW_PATH$RAW_NAME"
# 结果：documents/resume.pdf
```

### 用户隔离
```yaml
file_path: "user/$UID/$NOW_YEAR/$RAW_NAME"
# 结果：user/10001/2021/resume.pdf
```

---

## 新增文件

1. **[internal/app/usecase/storage/scanner.go](internal/app/usecase/storage/scanner.go)**
   - StorageScanner 实现
   - 扫描逻辑、路径解析、目录创建

2. **API 更新：[internal/app/api/storage.go](internal/app/api/storage.go)**
   - 新增 `scanObjects()` 方法
   - POST `/storages/{id}/scan` 端点

3. **绑定结构：[internal/pkg/bind/storage.go](internal/pkg/bind/storage.go)**
   - ScanStorageRequest
   - ScanStorageResponse

---

## 使用示例

### cURL 示例

```bash
# 扫描整个存储
curl -X POST \
  http://localhost:8222/api/storages/1/scan \
  -H "Authorization: Bearer eyJhbGc..." \
  -H "Content-Type: application/json" \
  -d '{}'

# 扫描特定前缀
curl -X POST \
  http://localhost:8222/api/storages/1/scan \
  -H "Authorization: Bearer eyJhbGc..." \
  -H "Content-Type: application/json" \
  -d '{"prefix": "docs/"}'
```

### JavaScript 示例

```javascript
// 使用 axios
async function scanStorage(storageId, prefix = '') {
  try {
    const response = await axios.post(
      `/api/storages/${storageId}/scan`,
      { prefix },
      {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      }
    );
    
    console.log(`创建了 ${response.data.created} 个文件`);
    console.log(`跳过了 ${response.data.skipped} 个文件`);
    
    if (response.data.errors.length > 0) {
      console.warn('错误：', response.data.errors);
    }
  } catch (error) {
    console.error('扫描失败：', error);
  }
}

// 使用
await scanStorage(1, 'documents/');
```

---

## 相关代码

### StorageScanner 关键方法

```go
// 扫描存储对象
func (s *StorageScanner) ScanStorageObjects(
  ctx context.Context, 
  uid int64,        // 用户ID
  sid int64,        // 存储ID
  prefix string     // 扫描前缀
) (*ScanResult, error)

// 解析对象为 Matter
func (s *StorageScanner) parseObjectToMatter(
  uid, sid int64, 
  obj provider.Object, 
  storage *entity.Storage
) *entity.Matter

// 确保父目录存在
func (s *StorageScanner) ensureParentDir(
  ctx context.Context, 
  uid, sid int64, 
  parent string
) error
```

### Matter 构造

```go
// 通过 file_path 构建对象路径
func (m *Matter) BuildObject(rootPath string, filePath string)

// 示例：
matter := entity.NewMatter(uid, sid, "resume.pdf")
matter.Parent = "/documents/"
matter.BuildObject("s3-bucket", "$RAW_PATH$RAW_NAME")
// matter.Object = "s3-bucket/documents/resume.pdf"
```

---

## 数据流

```
云存储对象
  ↓
Provider.List(prefix)  // 列出对象
  ↓
parseObjectToMatter()   // 路径解析
  ↓
ensureParentDir()       // 创建目录
  ↓
matterRepo.Create()     // 存储到数据库
  ↓
返回统计结果
```

---

## 常见问题

**Q: 如何避免重复导入？**
A: 扫描器会检查 `PathExist()`，已存在的文件会被跳过

**Q: 支持哪些存储提供商？**
A: 所有 S3 兼容的存储（S3、OSS、COS、MinIO等）

**Q: 可以部分扫描吗？**
A: 可以，使用 `prefix` 参数分批导入大型存储

**Q: 权限要求？**
A: Admin 权限（OAuth2: admin）

---

## 测试

```bash
# 编译
make build

# 运行
./build/bin/zpan server

# 测试扫描接口
curl -X POST http://localhost:8222/api/storages/1/scan \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{}'
```

---

更多详情请参考 [STORAGE_SCANNER_GUIDE.md](STORAGE_SCANNER_GUIDE.md)
