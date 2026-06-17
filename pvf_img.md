# PVF 装备读取与图片解析原理

本文梳理本项目「通过 PVF 读取装备 → 获取装备对应图片」的完整实现链路，作为 `go-parser` 重写/移植的参考。原始实现位于 Java 侧（`src/` 业务层 + `parser/` 解析库）。

## 整体架构

项目分两层：

- **底层解析库** `parser/dnf_parser/`（包名 `com.xiaoyouma.dnf.parser.*`）：纯二进制解析器
  - `dnf-parser-pvf`：解析 PVF 脚本
  - `dnf-parser-npk`：解析 NPK 图片包
- **业务层** `src/main/java/com/aiyi/game/dnfserver/`（包名 `com.aiyi.game.dnfserver.*`）：调用解析库，把数据转成实体并通过 HTTP 接口暴露。

两类数据源：

| 数据源 | 内容 |
| --- | --- |
| `data/Script.pvf` | 脚本包（装备属性、名称、**图标路径**） |
| `data/ImagePacks2/*.npk` | 图片资源包（实际贴图像素） |

> 关键点：**PVF 里不存图片，只存「图标在哪个 .img、第几帧」这个指针**；真正的像素在 NPK 里。两者通过装备脚本中的 `[icon]` 字段串联起来。

---

## 一、通过 PVF 读取装备

### 1. 初始化 PVF

`PvfManager.init()`（`@PostConstruct`）加载 `data/Script.pvf`，编码使用 **Big5**：

```java
File file = new File("data/Script.pvf");
PvfCoder.initialize(file.getAbsolutePath(), Charset.forName("Big5"));
```

底层 `new Pvf(path, charset)` 构造函数一次性完成 4 件事：

1. 整个文件读入 `ByteBuffer`（小端序 LITTLE_ENDIAN）；
2. `new PvfHeader()` 解析文件头（GUID/版本/目录树），并对目录树做 **CRC 解密**；
3. `loadTree()` 遍历目录树，把每个文件条目（`number/path/length/crc32/offset`）建成 `treeDict`；
4. `loadStringTable()` 加载 `stringtable.bin`（字符串表）、`loadNString()` 加载 `n_string.lst`（名称引用表）。

### 2. 读取装备列表

`PvfManager.getEquipmentList()` 的调用链：

```java
JSONObject script = pvf.getScript("equipment/equipment.lst");
for (String key : script.keySet()) {
    String str = script.getStr(key);
    JSONObject equipmentScript = pvf.getScript("equipment/" + str);
    Equipment equipment = Equipment.forScript(equipmentScript);
    equipment.setId(Integer.parseInt(key));
}
```

流程：

1. `getScript("equipment/equipment.lst")` → `LstParser` 解析出 `{装备id: "xxx.equ文件名"}` 映射表；
2. 遍历每个 id，再 `getScript("equipment/xxx.equ")` 读取单件装备脚本；
3. `Equipment.forScript()` 解析装备专属字段（攻防、四维、属强、抗性等），父类 `Item.parseForScript()` 解析通用字段（名称、稀有度、可用职业、说明，**以及图标 `[icon]`**）；
4. 过滤掉「未命名 / 找不到代码」的无效项。

道具走的是 `getStackableList()` → `stackable/stackable.lst`，逻辑完全一致。

### 3. `getScript` 内部解析过程

`Pvf.getScript(path)`：

- 用 `getTreeFile()` 在 `treeDict` 中按路径找到文件条目；
- `getTreeContent()` 按 `offset/length` 切出字节，并用 `crcDecrypt(content, crc32)` 解密；
- 根据后缀路由到不同解析器（`ScriptType` 枚举），最终组装成层级 `JSONObject`（段落形如 `[xxx]...[/xxx]`）：

| 后缀 | 解析器 | 说明 |
| --- | --- | --- |
| `.lst` | `LstParser` | id → 文件路径映射 |
| `.bin` | `BinParser` | 二进制表（如 stringtable.bin） |
| `.str` | `StrParser` | 字符串 |
| `.ani` | `AniParser` | 动画 |
| `.ui` | `UIParser` | UI |
| 其他（`.equ/.stk` 等） | `DefaultParser` | 按 unitType 1-10 读 int/float/字符串/stringtable 引用 |

### 4. 关键：图标字段从哪来

装备脚本里有 `[icon]` 段，`Item.parseForScript()` 把它解析成 `ItemIcon`：

```java
if (script.containsKey("[icon]") && !script.getJSONArray("[icon]").isEmpty()){
    JSONArray arrays = JSON.parseArray(script.getStr("[icon]"));
    ItemIcon itemIcon = new ItemIcon();
    itemIcon.setPath(arrays.getString(0).toLowerCase());
    if (arrays.size() >= 2){
        itemIcon.setIndex(arrays.getIntValue(1));
    }
    this.icon = itemIcon;
}
```

`ItemIcon` 只有两个字段：

- `path` —— 图标所在的 **.img 文件名**（如 `equipment/weapon/xxx.img`）
- `index` —— 这张 .img 里的**第几帧贴图**

这就是连接 PVF 与 NPK 的桥梁。

---

## 二、根据图标信息获取具体图片

### 1. 初始化 NPK

`NpkManager.init()` 加载 `data/ImagePacks2` 目录：

```java
File file = new File("data/ImagePacks2");
NpkCoder.initialize(file.getAbsolutePath());
```

`NpkCoder.initialize()` 并行扫描所有 `.npk`，建立两张全局表：

- `NPK_IMG_NAME_TABLE`：img名 → 所在 npk 文件名
- `NPK_IMG_INDEX_TABLE`：img名 → 在 npk 中的 `offset/length` 索引

### 2. HTTP 取图接口

前端拿到装备的 `icon.path` 和 `icon.index` 后，请求 `GET api/v1/pvf/img?path=xxx&index=n`：

```java
@GetMapping("img")
public void img(String path, int index, HttpServletResponse response) {
    byte[] imageBytes = npkManager.getImageBytes(path, index);
    response.setContentType("image/png");
    response.getOutputStream().write(imageBytes);
}
```

### 3. 取图核心链路

`NpkManager.getImageBytes(path, index)`：

```java
NpkTexture[] textures = NpkCoder.loadImg(path).getTextures();
return textures[index].toPngBytes();
```

详细步骤：

1. **定位 .img**：`NpkCoder.loadImg(path)` 用 `NPK_IMG_NAME_TABLE` 找到 .img 在哪个 .npk，用 `NPK_IMG_INDEX_TABLE` 拿到 offset，`seek` 过去读取这个 .img（魔数 `Neople Image File` / `Neople Img File`，读版本号、贴图数量等）。
2. **解析贴图索引**：按版本路由到 `V1Handle` / `V2Handle`（`HandleFactory`）。以 `V2Handle.readStream()` 为例，逐帧读取每张贴图的元信息：颜色格式（`ColorBit`）、压缩方式（`CompressMode`）、宽高、坐标、帧域、数据长度。其中 `indexType == 0x11` 是**链接贴图**（复用另一帧的数据，节省空间）。
3. **取第 index 帧 → 转 PNG**：`textures[index].toPngBytes()`：
   - `getBgraData()` → `V2Handle.convertData()`：若是 ZLIB 压缩先 `unZlib` 解压，再 `ColorHelper.readBgraBytes()` 把不同颜色格式统一转成 BGRA8888；
   - `ColorHelper` 支持 `ARGB_8888` / `ARGB_1555` / `ARGB_4444` 三种格式转换，全透明像素（`a == 0`）清零；
   - 最后用 `BufferedImage`(TYPE_INT_ARGB) 逐像素填充，`ImageIO.write(image, "png", ...)` 输出 PNG 字节流。

```java
public byte[] toPngBytes() {
    byte[] bgraData = getBgraData();
    BufferedImage image = new BufferedImage(width, height, BufferedImage.TYPE_INT_ARGB);
    // 逐像素：image.setRGB(x, y, (a << 24) | (r << 16) | (g << 8) | b);
    ImageIO.write(image, "png", outputStream);
    return outputStream.toByteArray();
}
```

---

## 三、完整链路总结

```text
启动:
  PvfManager.init()  → 加载 Script.pvf (建文件树/字符串表)
  NpkManager.init()  → 扫描 ImagePacks2/*.npk (建 img→npk 索引)

读装备:
  equipment.lst ──(id→.equ)──▶ xxx.equ
     ├─ Equipment.forScript()  → 攻防/四维/属强...
     └─ Item.parseForScript()  → 名称 + [icon] = {path, index}   ← 关键桥梁

取图 (GET api/v1/pvf/img?path=&index=):
  icon.path  ─▶ NpkCoder.loadImg() ─▶ 定位 .npk 中的 .img ─▶ 解析所有贴图帧
  icon.index ─▶ textures[index] ─▶ 解压(zlib)+颜色转换(BGRA) ─▶ PNG 字节流
```

一句话：**PVF 的 `[icon]` 字段提供 `(img路径, 帧下标)` 这个「坐标」，NPK 解析器据此从图片包里定位到对应贴图、解压解码后实时转成 PNG 返回。**

---

## 四、前端「PVF 管理」页面对应关系

参见 `doc/pvf.png`，后端能力在前端的最终呈现：

| 页面元素 | 对应接口 / 实现 |
| --- | --- |
| 顶部 `当前PVF / 装备 / 道具` 统计 | `GET api/v1/pvf/info`（数据来自 `PvfCache`，首次访问触发 `ensurePvfCache()`） |
| `上传新PVF` 按钮 | `POST api/v1/pvf/upload`（`accountVO.isAdmin()` 校验，仅 GM 可操作） |
| 左侧分类树（装备/武器/上衣… / 道具） | `search` 接口的 `type`(equipment/stackable) + `subType`(`EquipmentType`/`StackableType`) 过滤 |
| 中间列表 + 搜索框 + 分页 | `GET api/v1/pvf/search?keyword=&type=&subType=&page=&pageSize=`，返回 `ResultPage<Item>` |
| 列表小图标 / 详情卡物品图 | `GET api/v1/pvf/img?path=&index=`，由 `NpkCoder.loadImg(path).getTextures()[index].toPngBytes()` 实时解码 |
| 详情卡字段（ID/名称/类型/等级/稀有度/交易类型/携带上限/职业/usableJobs） | `Item` / `Equipment` 实体字段（由 `parseForScript` / `forScript` 解析得到） |

---

## 五、关键类速查表

### 业务层（`com.aiyi.game.dnfserver`）

| 类 | 职责 |
| --- | --- |
| `pvf/PvfManager` | 加载 Script.pvf；`getEquipmentList()` / `getStackableList()` |
| `pvf/NpkManager` | 加载 ImagePacks2；`getImageBytes(path, index)` |
| `pvf/PvfCache` | 缓存装备/道具列表与 PVF 文件大小 |
| `controller/PvfController` | HTTP 接口 `info / search / img / upload` |
| `entity/common/Item` | 物品基类，`parseForScript()` 解析通用字段含 `[icon]` |
| `entity/common/ItemIcon` | 图标实体：`path` + `index` |
| `entity/equipment/Equipment` | 装备实体，`forScript()` 解析装备专属字段 |
| `entity/stackable/Stackable` | 道具实体，`forScript()` |

### 解析库层（`com.xiaoyouma.dnf.parser`）

| 类 | 职责 |
| --- | --- |
| `pvf/coder/PvfCoder` | PVF 解析入口（`initialize` / `getPvf` / `getScript`） |
| `pvf/model/Pvf` | PVF 核心模型：文件树、字符串表、`getScript()` |
| `pvf/model/PvfHeader` | 文件头解析 + 目录树 CRC 解密 |
| `pvf/parser/*` | `LstParser` / `DefaultParser` / `BinParser` / `StrParser` / `AniParser` / `UIParser` |
| `pvf/util/PvfHelper` | `crcDecrypt()` CRC32 解密、`getScriptType()` 后缀判型 |
| `npk/coder/NpkCoder` | NPK 解析入口（`initialize` / `loadImg`） |
| `npk/model/NpkImg` / `NpkTexture` | img 与单帧贴图模型，`toPngBytes()` 输出 PNG |
| `npk/handle/V1Handle` / `V2Handle` | 不同 IMG 版本的贴图解析与解码 |
| `npk/util/ColorHelper` | ARGB_8888 / 1555 / 4444 → BGRA8888 颜色转换 |

