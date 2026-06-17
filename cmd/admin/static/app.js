const state = {
  view: "characters",
  health: null,
  characters: { page: 1, pageSize: 20, total: 0, items: [] },
  mail: { page: 1, pageSize: 20, hasMore: false, items: [] },
  items: { page: 1, pageSize: 40, total: 0, items: [], facetsLoaded: false, category: "all" },
  itemDetailRequest: 0,
  resources: null,
  selectedCharacter: null
};

const viewMeta = {
  characters: {
    title: "角色管理",
    subtitle: "查询角色、编辑常用属性，并快速跳转发货。"
  },
  operations: {
    title: "运营管理",
    subtitle: "查询账号资源，调整点券、金币、SP/TP/QP，并执行角色高级操作。"
  },
  mail: {
    title: "邮件发货",
    subtitle: "发送物品、金币或信件，并查看最近邮件记录。"
  },
  items: {
    title: "物品库",
    subtitle: "搜索 PVF 解析出的装备和道具，支持从结果直接填入发货表单。"
  },
  system: {
    title: "系统状态",
    subtitle: "查看数据库、PVF 物品缓存和 NPK 图标资源的连接状态。"
  }
};

const statLabels = {
  equipmentType: "装备类型",
  equipmentTypeTag: "装备标签",
  avatar: "时装",
  itemGroup: "装备子类",
  physicalAttack: "物理攻击",
  magicalAttack: "魔法攻击",
  separateAttack: "独立攻击",
  physicalDefense: "物理防御",
  magicalDefense: "魔法防御",
  strength: "力量",
  intelligence: "智力",
  vitality: "体力",
  spirit: "精神",
  hpMax: "HP 最大值",
  mpMax: "MP 最大值",
  hpRegenSpeed: "HP 恢复",
  mpRegenSpeed: "MP 恢复",
  attackSpeed: "攻击速度",
  castSpeed: "施放速度",
  moveSpeed: "移动速度",
  physicalCriticalHit: "物理暴击",
  magicalCriticalHit: "魔法暴击",
  hitRate: "命中率",
  dodge: "回避率",
  hitRecovery: "硬直",
  jumpPower: "跳跃力",
  inventoryLimit: "负重",
  antiEvil: "抗魔值",
  fireElement: "火属性",
  waterElement: "水属性",
  lightElement: "光属性",
  darkElement: "暗属性",
  fireAttack: "火属性强化",
  waterAttack: "水属性强化",
  lightAttack: "光属性强化",
  darkAttack: "暗属性强化",
  allElementalAttack: "所有属性强化",
  fireResistance: "火抗性",
  waterResistance: "水抗性",
  lightResistance: "光抗性",
  darkResistance: "暗抗性",
  allElementalResistance: "所有属性抗性",
  blindResistance: "失明抗性",
  lightningResistance: "感电抗性",
  burnResistance: "灼伤抗性",
  freezeResistance: "冰冻抗性",
  holdResistance: "束缚抗性",
  sleepResistance: "睡眠抗性",
  bleedingResistance: "出血抗性",
  confuseResistance: "混乱抗性",
  curseResistance: "诅咒抗性",
  stoneResistance: "石化抗性",
  allActiveStatusResistance: "异常状态抗性",
  grade: "品级",
  weight: "重量",
  durability: "耐久",
  price: "价格",
  repairPrice: "修理价格",
  value: "价值",
  roomListMoveSpeedRate: "城镇移动",
  stackableType: "道具类型"
};

const itemCategories = [
  { key: "all", label: "全部物品", filters: { type: "all" } },
  { key: "equipment_all", label: "全部装备", filters: { type: "equipment" } },
  { key: "stackable_all", label: "全部道具", filters: { type: "stackable" } },
  { key: "weapon", label: "武器", filters: { type: "equipment", equipmentType: "武器" } },
  { key: "titleName", label: "称号", filters: { type: "equipment", equipmentType: "称号" } },
  { key: "coat", label: "上衣", filters: { type: "equipment", equipmentType: "上衣" } },
  { key: "shoulder", label: "护肩", filters: { type: "equipment", equipmentType: "护肩" } },
  { key: "pants", label: "裤子", filters: { type: "equipment", equipmentType: "裤子" } },
  { key: "shoes", label: "鞋子", filters: { type: "equipment", equipmentType: "鞋子" } },
  { key: "waist", label: "腰带", filters: { type: "equipment", equipmentType: "腰带" } },
  { key: "amulet", label: "项链", filters: { type: "equipment", equipmentType: "项链" } },
  { key: "wrist", label: "手镯", filters: { type: "equipment", equipmentType: "手镯" } },
  { key: "ring", label: "戒指", filters: { type: "equipment", equipmentType: "戒指" } },
  { key: "support", label: "辅助装备", filters: { type: "equipment", equipmentType: "辅助装备" } },
  { key: "magicStone", label: "魔法石", filters: { type: "equipment", equipmentType: "魔法石" } },
  { key: "creature_equipment", label: "宠物", filters: { type: "equipment", equipmentType: "宠物" } },
  { key: "artifact_red", label: "宠物装备-红色", filters: { type: "equipment", equipmentType: "宠物装备-红色" } },
  { key: "artifact_green", label: "宠物装备-绿色", filters: { type: "equipment", equipmentType: "宠物装备-绿色" } },
  { key: "artifact_blue", label: "宠物装备-蓝色", filters: { type: "equipment", equipmentType: "宠物装备-蓝色" } },
  { key: "avatar", label: "时装", filters: { type: "equipment", avatar: "true" } },
  { key: "waste", label: "消耗品", filters: { type: "stackable", stackableType: "消耗品" } },
  { key: "material", label: "材料", filters: { type: "stackable", stackableType: "材料" } },
  { key: "recipe", label: "设计图", filters: { type: "stackable", stackableType: "设计图" } },
  { key: "material_expert_job", label: "副职业", filters: { type: "stackable", stackableType: "副职业" } },
  { key: "quest", label: "任务道具", filters: { type: "stackable", stackableType: "任务道具" } },
  { key: "booster", label: "礼盒", filters: { type: "stackable", stackableType: "礼盒" } },
  { key: "feed", label: "饲料", filters: { type: "stackable", stackableType: "饲料" } },
  { key: "creature_stackable", label: "宠物道具", filters: { type: "stackable", stackableType: "宠物" } },
  { key: "throwItem", label: "投掷物", filters: { type: "stackable", stackableType: "投掷物" } },
  { key: "legacy", label: "罐子", filters: { type: "stackable", stackableType: "罐子" } },
  { key: "etc", label: "杂物", filters: { type: "stackable", stackableType: "杂物" } }
];

const advancedActions = {
  rename: { label: "改名", fields: ["name"] },
  level: { label: "改等级", fields: ["level"] },
  job: { label: "改职业", fields: ["job", "growType", "expertJob"] },
  move: { label: "移动角色", fields: ["moveUid"], confirm: true },
  delete: { label: "删除角色", fields: [], confirm: true },
  recover: { label: "恢复角色", fields: [], confirm: true },
  ban: { label: "封号", fields: ["banDays", "banReason"], confirm: true },
  unban: { label: "解封", fields: [], confirm: true }
};

const commonItemKeys = new Set([
  "id",
  "rarity",
  "rarityName",
  "name",
  "type",
  "usableJobs",
  "attachType",
  "minimumLevel",
  "description",
  "explain",
  "stackLimit",
  "icon",
  "iconUrl",
  "pvfPath",
  "pvfSource",
  "pvfFields",
  "pvfError",
  "equipmentTypeTag",
  "itemGroupTag",
  "stackableTypeTag"
]);

const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => Array.from(root.querySelectorAll(selector));

document.addEventListener("DOMContentLoaded", () => {
  bindNavigation();
  bindCharacters();
  bindOperations();
  bindMail();
  bindItems();
  bindDrawers();
  refreshHealth();
  loadCharacters();
});

function bindNavigation() {
  $$(".nav-btn").forEach((button) => {
    button.addEventListener("click", () => setView(button.dataset.view));
  });
}

function bindCharacters() {
  $("#character-filter").addEventListener("submit", (event) => {
    event.preventDefault();
    state.characters.page = 1;
    loadCharacters();
  });
  $("#reset-character-filter").addEventListener("click", () => {
    $("#character-filter").reset();
    state.characters.page = 1;
    loadCharacters();
  });
  $("#refresh-characters").addEventListener("click", loadCharacters);
  $("#characters-prev").addEventListener("click", () => {
    if (state.characters.page > 1) {
      state.characters.page -= 1;
      loadCharacters();
    }
  });
  $("#characters-next").addEventListener("click", () => {
    if (state.characters.page * state.characters.pageSize < state.characters.total) {
      state.characters.page += 1;
      loadCharacters();
    }
  });
  $("#characters-body").addEventListener("click", (event) => {
    const button = event.target.closest("button[data-action]");
    if (!button) return;
    const id = Number(button.dataset.id);
    const row = state.characters.items.find((item) => item.characNo === id);
    if (!row) return;
    if (button.dataset.action === "edit") {
      openCharacterDrawer(row);
    }
    if (button.dataset.action === "resources") {
      fillOperations(row);
    }
    if (button.dataset.action === "advanced") {
      fillAdvancedCharacter(row);
    }
    if (button.dataset.action === "mail") {
      $("#mail-form [name='receiveCharacNo']").value = String(row.characNo);
      setView("mail");
      showToast(`已填入收件角色 ${row.characName}`, "ok");
    }
  });
  $("#character-form").addEventListener("submit", saveCharacter);
}

function bindOperations() {
  updateAdvancedActionFields();
  $("#resource-query-form").addEventListener("submit", (event) => {
    event.preventDefault();
    loadResources();
  });
  $("#resource-patch-form").addEventListener("submit", patchResource);
  $("#refresh-operations").addEventListener("click", loadResources);
  $("#advanced-character-form [name='action']").addEventListener("change", updateAdvancedActionFields);
  $("#advanced-character-form").addEventListener("submit", saveAdvancedCharacter);
  $("#pvp-form").addEventListener("submit", savePVP);
}

function bindMail() {
  $("#mail-form").addEventListener("submit", sendMail);
  $("#clear-mail-form").addEventListener("click", () => {
    $("#mail-form").reset();
    $("#mail-form [name='count']").value = "1";
    $("#mail-form [name='gold']").value = "0";
    $("#mail-form [name='letterId']").value = "0";
    $("#mail-form [name='endurance']").value = "0";
    $("#mail-form [name='attachmentType']").value = "normal";
  });
  $("#mail-filter").addEventListener("submit", (event) => {
    event.preventDefault();
    state.mail.page = 1;
    loadMail();
  });
  $("#mail-prev").addEventListener("click", () => {
    if (state.mail.page > 1) {
      state.mail.page -= 1;
      loadMail();
    }
  });
  $("#mail-next").addEventListener("click", () => {
    if (state.mail.hasMore) {
      state.mail.page += 1;
      loadMail();
    }
  });
  $("#mail-body").addEventListener("click", (event) => {
    const button = event.target.closest("button[data-action='delete-mail']");
    if (!button) return;
    deleteMail(button.dataset.id);
  });
}

function bindItems() {
  renderItemCategoryBrowser();
  $("#item-filter").addEventListener("submit", (event) => {
    event.preventDefault();
    state.items.page = 1;
    syncItemCategoryFromFilters();
    renderItemCategoryBrowser();
    loadItems();
  });
  $("#reset-item-filter").addEventListener("click", () => {
    $("#item-filter").reset();
    state.items.page = 1;
    state.items.category = "all";
    updateItemFilterAvailability();
    renderItemCategoryBrowser();
    loadItems();
  });
  $("#refresh-items").addEventListener("click", reloadItems);
  $("#item-type").addEventListener("change", () => {
    updateItemFilterAvailability();
    syncItemCategoryFromFilters();
    renderItemCategoryBrowser();
  });
  ["#equipment-type-select", "#item-group-select", "#item-filter [name='avatar']"].forEach((selector) => {
    $(selector).addEventListener("change", () => {
      if ($(selector).value) $("#item-type").value = "equipment";
      updateItemFilterAvailability();
      syncItemCategoryFromFilters();
      renderItemCategoryBrowser();
    });
  });
  $("#stackable-type-select").addEventListener("change", () => {
    if ($("#stackable-type-select").value) $("#item-type").value = "stackable";
    updateItemFilterAvailability();
    syncItemCategoryFromFilters();
    renderItemCategoryBrowser();
  });
  $("#item-category-browser").addEventListener("click", (event) => {
    const button = event.target.closest("button[data-category]");
    if (!button) return;
    applyItemCategory(button.dataset.category);
  });
  $("#items-prev").addEventListener("click", () => {
    if (state.items.page > 1) {
      state.items.page -= 1;
      loadItems();
    }
  });
  $("#items-next").addEventListener("click", () => {
    if (state.items.page * state.items.pageSize < state.items.total) {
      state.items.page += 1;
      loadItems();
    }
  });
  $("#items-body").addEventListener("click", (event) => {
    const button = event.target.closest("button[data-action]");
    if (!button) return;
    const id = Number(button.dataset.id);
    const type = button.dataset.type;
    if (button.dataset.action === "detail") {
      openItemDrawer(type, id);
    }
    if (button.dataset.action === "mail-item") {
      fillMailItem(id);
    }
  });
  $("#refresh-health").addEventListener("click", refreshHealth);
  updateItemFilterAvailability();
}

function bindDrawers() {
  $$("#drawer-backdrop, [data-close-drawer]").forEach((element) => {
    element.addEventListener("click", closeDrawers);
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape") closeDrawers();
  });
}

function setView(view) {
  state.view = view;
  $$(".nav-btn").forEach((button) => button.classList.toggle("active", button.dataset.view === view));
  $$(".view").forEach((section) => section.classList.toggle("active", section.id === `view-${view}`));
  $("#view-title").textContent = viewMeta[view].title;
  $("#view-subtitle").textContent = viewMeta[view].subtitle;

  if (view === "characters" && state.characters.items.length === 0) loadCharacters();
  if (view === "operations" && !state.resources) loadResources();
  if (view === "mail" && state.mail.items.length === 0) loadMail();
  if (view === "items" && state.items.items.length === 0) loadItems();
  if (view === "system") refreshHealth();
}

async function api(path, options = {}) {
  const response = await fetch(path, {
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
    ...options
  });
  const contentType = response.headers.get("content-type") || "";
  const payload = contentType.includes("application/json") ? await response.json() : await response.text();
  if (!response.ok) {
    const message = payload && payload.error ? payload.error : String(payload || response.statusText);
    throw new Error(message);
  }
  return payload;
}

async function refreshHealth() {
  try {
    const health = await api("/api/health");
    state.health = health;
    renderHealthStrip(health);
    renderHealthDetails(health);
  } catch (error) {
    $("#status-strip").innerHTML = `<span class="status-pill err">${escapeHTML(error.message)}</span>`;
  }
}

function renderHealthStrip(health) {
  const dbClass = health.database.ready ? "ok" : "warn";
  const itemClass = health.items.loaded ? "ok" : health.items.configured ? "warn" : "warn";
  const npkClass = health.npk.ready ? "ok" : health.npk.configured ? "warn" : "muted";
  $("#status-strip").innerHTML = [
    `<span class="status-pill ${dbClass}">数据库 ${health.database.ready ? "已连接" : "未就绪"}</span>`,
    `<span class="status-pill ${itemClass}">PVF ${health.items.loaded ? "已加载" : "未加载"}</span>`,
    `<span class="status-pill ${npkClass}">NPK ${health.npk.ready ? "已加载" : "可选"}</span>`
  ].join("");
}

function renderHealthDetails(health) {
  const target = $("#health-details");
  if (!target) return;
  target.innerHTML = [
    healthPanel("数据库", health.database.ready, [
      ["配置", health.database.configured ? "已配置" : "未配置"],
      ["状态", health.database.ready ? "已连接" : "未就绪"],
      ["说明", health.database.error || health.database.detail || ""]
    ]),
    healthPanel("PVF 物品", health.items.loaded, [
      ["路径", health.items.pvfPath || "未配置"],
      ["编码", health.items.pvfCharset || ""],
      ["装备数量", health.items.equipmentCount],
      ["道具数量", health.items.stackableCount],
      ["加载时间", health.items.loadedAt ? formatDate(health.items.loadedAt) : ""],
      ["错误", health.items.error || ""]
    ]),
    healthPanel("NPK 图标", health.npk.ready, [
      ["路径", health.npk.detail || "未配置"],
      ["状态", health.npk.ready ? "已加载" : "按需加载"],
      ["错误", health.npk.error || ""]
    ]),
    healthPanel("构建信息", true, [
      ["版本", health.build?.version || "dev"],
      ["Commit", health.build?.commit || "none"],
      ["构建时间", health.build?.buildTime || "unknown"],
      ["服务器时间", health.serverTime ? formatDate(health.serverTime) : ""]
    ])
  ].join("");
}

function healthPanel(title, ok, rows) {
  return `
    <section class="panel health-row">
      <div class="section-head">
        <h3>${escapeHTML(title)}</h3>
        <span class="tag ${ok ? "ok" : "warn"}">${ok ? "可用" : "未就绪"}</span>
      </div>
      ${rows.map(([label, value]) => `
        <div>
          <span class="subtle">${escapeHTML(String(label))}</span>
          <code class="${String(label).toLowerCase() === "commit" ? "breakable" : ""}" title="${escapeHTML(String(value ?? ""))}">${escapeHTML(String(value ?? "")) || "-"}</code>
        </div>
      `).join("")}
    </section>
  `;
}

async function loadCharacters() {
  const body = $("#characters-body");
  body.innerHTML = `<tr><td colspan="9" class="empty">加载中</td></tr>`;
  try {
    const form = $("#character-filter");
    const params = formParams(form);
    params.set("page", String(state.characters.page));
    const data = await api(`/api/characters?${params.toString()}`);
    state.characters = {
      page: data.page,
      pageSize: data.pageSize,
      total: data.total,
      items: data.items || []
    };
    renderCharacters();
    refreshHealth();
  } catch (error) {
    body.innerHTML = `<tr><td colspan="9" class="empty">${escapeHTML(error.message)}</td></tr>`;
    showToast(error.message, "err");
  }
}

function renderCharacters() {
  const rows = state.characters.items;
  const body = $("#characters-body");
  if (rows.length === 0) {
    body.innerHTML = `<tr><td colspan="9" class="empty">没有匹配角色</td></tr>`;
  } else {
    body.innerHTML = rows.map((item) => `
      <tr>
        <td>
          <div class="main-cell">
            <strong title="${escapeHTML(item.characName)}">${escapeHTML(item.characName)}</strong>
            <span class="subtle">ID ${item.characNo}</span>
          </div>
        </td>
        <td>
          <div class="main-cell">
            <strong title="${escapeHTML(item.accountName || "")}">${escapeHTML(item.accountName || "-")}</strong>
            <span class="subtle">UID ${item.mid}</span>
          </div>
        </td>
        <td>${item.job}</td>
        <td>${item.lev}</td>
        <td>${item.hp} / ${item.maxHp} · MP ${item.maxMp}</td>
        <td>${item.attackSpeed} / ${item.castSpeed} / ${item.moveSpeed}</td>
        <td>${item.fatigue}</td>
        <td>${formatDate(item.createTime)}</td>
        <td>
          <div class="row-actions">
            <button class="btn small" data-action="edit" data-id="${item.characNo}">编辑</button>
            <button class="btn small secondary" data-action="resources" data-id="${item.characNo}">资源</button>
            <button class="btn small secondary" data-action="mail" data-id="${item.characNo}">发货</button>
            <button class="btn small secondary" data-action="advanced" data-id="${item.characNo}">高级</button>
          </div>
        </td>
      </tr>
    `).join("");
  }
  const maxPage = Math.max(1, Math.ceil(state.characters.total / state.characters.pageSize));
  $("#characters-page").textContent = `第 ${state.characters.page} / ${maxPage} 页 · 共 ${state.characters.total} 条`;
  $("#characters-prev").disabled = state.characters.page <= 1;
  $("#characters-next").disabled = state.characters.page >= maxPage;
}

function fillOperations(character) {
  $("#resource-query-form [name='account']").value = character.accountName || "";
  $("#resource-query-form [name='characName']").value = character.characName || "";
  $("#resource-query-form [name='uid']").value = String(character.mid);
  $("#resource-query-form [name='characNo']").value = String(character.characNo);
  const advancedForm = $("#advanced-character-form");
  advancedForm.elements.characNo.value = String(character.characNo);
  advancedForm.elements.action.value = "rename";
  advancedForm.elements.name.value = character.characName || "";
  updateAdvancedActionFields();
  $("#pvp-form [name='characNo']").value = String(character.characNo);
  setView("operations");
  loadResources();
}

function fillAdvancedCharacter(character) {
  const form = $("#advanced-character-form");
  form.elements.characNo.value = String(character.characNo);
  form.elements.action.value = "rename";
  form.elements.name.value = character.characName || "";
  form.elements.level.value = character.lev ?? "";
  form.elements.job.value = character.job ?? "";
  form.elements.growType.value = character.growType ?? "";
  form.elements.expertJob.value = character.expertJob ?? "";
  $("#resource-query-form [name='account']").value = character.accountName || "";
  $("#resource-query-form [name='characName']").value = character.characName || "";
  $("#resource-query-form [name='uid']").value = String(character.mid);
  $("#resource-query-form [name='characNo']").value = String(character.characNo);
  $("#pvp-form [name='characNo']").value = String(character.characNo);
  updateAdvancedActionFields();
  setView("operations");
  loadResources();
}

async function loadResources() {
  const form = $("#resource-query-form");
  const params = formParams(form);
  const summary = $("#resource-summary");
  if (!params.has("account") && !params.has("characName") && !params.has("uid") && !params.has("characNo")) {
    summary.innerHTML = `<div class="muted-panel">输入账号名、角色名、账号 UID 或角色 ID 后查询。</div>`;
    return;
  }
  summary.innerHTML = `<div class="muted-panel">资源加载中</div>`;
  try {
    const data = await api(`/api/resources?${params.toString()}`);
    state.resources = data;
    if (data.uid) {
      form.elements.uid.value = String(data.uid);
    }
    if (data.accountName) {
      form.elements.account.value = data.accountName;
    }
    if (data.characNo) {
      form.elements.characNo.value = String(data.characNo);
      form.elements.characName.value = data.characName || form.elements.characName.value;
      $("#advanced-character-form [name='characNo']").value = String(data.characNo);
      if (data.characName) $("#advanced-character-form [name='name']").value = data.characName;
      $("#pvp-form [name='characNo']").value = String(data.characNo);
    }
    renderResources(data);
  } catch (error) {
    summary.innerHTML = `<div class="muted-panel">${escapeHTML(error.message)}</div>`;
    showToast(error.message, "err");
  }
}

function renderResources(data) {
  const rows = [
    ["账号名", data.accountName || "-"],
    ["账号 UID", data.uid],
    ["角色名", data.characName || "-"],
    ["角色 ID", data.characNo || "-"],
    ["D币 / 点券", data.cera],
    ["D点 / 代币", data.ceraPoint],
    ["账号金库金币", data.accountMoney],
    ["角色金币", data.characMoney],
    ["时装硬币", data.avatarCoin],
    ["SP", `${data.sp} / ${data.sp2}`],
    ["TP", `${data.tp} / ${data.tp2}`],
    ["QP", data.qp],
    ["pay_coin", data.payCoin],
    ["建角限制", data.createLimitCount],
    ["封禁", data.banned ? `是${data.banEndTime ? ` 至 ${formatDate(data.banEndTime)}` : ""}` : "否"],
    ["封禁原因", data.banReason || "-"],
    ["PVP", `段位 ${data.pvpGrade} · 胜场 ${data.pvpWin} · 胜点 ${data.pvpPoint}`]
  ];
  $("#resource-summary").innerHTML = rows.map(([label, value]) => `
    <div class="fact">
      <span>${escapeHTML(label)}</span>
      <strong>${escapeHTML(formatValue(value))}</strong>
    </div>
  `).join("");
  const pvp = $("#pvp-form");
  pvp.elements.grade.value = data.pvpGrade ?? 0;
  pvp.elements.win.value = data.pvpWin ?? 0;
  pvp.elements.point.value = data.pvpPoint ?? 0;
  pvp.elements.winPoint.value = data.pvpWinPoint ?? 0;
}

async function patchResource(event) {
  event.preventDefault();
  const query = $("#resource-query-form");
  const form = event.currentTarget;
  const payload = {
    account: query.elements.account.value.trim(),
    characName: query.elements.characName.value.trim(),
    uid: numberOrZero(query.elements.uid.value),
    characNo: numberOrZero(query.elements.characNo.value),
    target: form.elements.target.value,
    mode: form.elements.mode.value,
    value: numberOrZero(form.elements.value.value)
  };
  try {
    const data = await api("/api/resources", {
      method: "POST",
      body: JSON.stringify(payload)
    });
    state.resources = data;
    renderResources(data);
    showToast("资源已调整", "ok");
  } catch (error) {
    showToast(error.message, "err");
  }
}

function updateAdvancedActionFields() {
  const form = $("#advanced-character-form");
  if (!form) return;
  const action = form.elements.action.value;
  const config = advancedActions[action] || advancedActions.rename;
  const enabled = new Set(config.fields || []);
  $$("[data-advanced-field]").forEach((field) => {
    const name = field.dataset.advancedField;
    const visible = enabled.has(name);
    field.classList.toggle("hidden-field", !visible);
    const input = field.querySelector("input, select, textarea");
    if (input) input.disabled = !visible;
  });
  const button = $("#advanced-save-button");
  button.textContent = `保存${config.label}`;
  button.classList.toggle("danger", Boolean(config.confirm && (action === "delete" || action === "ban")));
  button.classList.toggle("primary", !(config.confirm && (action === "delete" || action === "ban")));
}

async function saveAdvancedCharacter(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const action = form.elements.action.value;
  const config = advancedActions[action] || advancedActions.rename;
  const characNo = numberOrZero(form.elements.characNo.value);
  if (!characNo) {
    showToast("角色 ID 不能为空", "err");
    return;
  }
  let payload = {};
  if (action === "rename") {
    payload = { name: form.elements.name.value.trim() };
    if (!payload.name) return showToast("新角色名不能为空", "err");
  }
  if (action === "level") {
    payload = { level: numberOrZero(form.elements.level.value) };
    if (!payload.level) return showToast("等级不能为空", "err");
  }
  if (action === "job") {
    payload = {};
    ["job", "growType", "expertJob"].forEach((key) => {
      const raw = form.elements[key].value.trim();
      if (raw !== "") payload[key] = numberOrZero(raw);
    });
    if (Object.keys(payload).length === 0) return showToast("至少填写一个职业字段", "err");
  }
  if (action === "move") {
    payload = { uid: numberOrZero(form.elements.moveUid.value) };
    if (!payload.uid) return showToast("目标账号 UID 不能为空", "err");
  }
  if (action === "ban") {
    payload = {
      days: numberOrZero(form.elements.banDays.value) || 365,
      reason: form.elements.banReason.value.trim()
    };
  }
  if (config.confirm && !confirm(`确定执行 ${config.label} 吗？`)) return;
  try {
    await api(`/api/characters/${characNo}/${action}`, {
      method: "POST",
      body: JSON.stringify(payload)
    });
    showToast(`${config.label}已保存`, "ok");
    loadCharacters();
    loadResources();
  } catch (error) {
    showToast(error.message, "err");
  }
}

async function savePVP(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const characNo = numberOrZero(form.elements.characNo.value);
  if (!characNo) return showToast("角色 ID 不能为空", "err");
  const payload = {
    grade: numberOrZero(form.elements.grade.value),
    win: numberOrZero(form.elements.win.value),
    point: numberOrZero(form.elements.point.value),
    winPoint: numberOrZero(form.elements.winPoint.value)
  };
  try {
    await api(`/api/characters/${characNo}/pvp`, {
      method: "POST",
      body: JSON.stringify(payload)
    });
    showToast("PVP 数据已保存", "ok");
    loadResources();
  } catch (error) {
    showToast(error.message, "err");
  }
}

function openCharacterDrawer(character) {
  state.selectedCharacter = character;
  $("#character-drawer-title").textContent = `编辑 ${character.characName}`;
  $("#character-drawer-subtitle").textContent = `角色 ID ${character.characNo} · 等级 ${character.lev}`;
  const form = $("#character-form");
  [
    "attackSpeed",
    "castSpeed",
    "moveSpeed",
    "fatigue",
    "maxHp",
    "maxMp",
    "phyAttack",
    "phyDefense",
    "magAttack",
    "magDefense",
    "hitRecovery",
    "jump"
  ].forEach((key) => {
    form.elements[key].value = character[key] ?? "";
  });
  openDrawer("#character-drawer");
}

async function saveCharacter(event) {
  event.preventDefault();
  if (!state.selectedCharacter) return;
  const form = event.currentTarget;
  const payload = {};
  Array.from(new FormData(form).entries()).forEach(([key, value]) => {
    payload[key] = numberOrZero(value);
  });
  try {
    await api(`/api/characters/${state.selectedCharacter.characNo}`, {
      method: "PUT",
      body: JSON.stringify(payload)
    });
    showToast("角色属性已保存", "ok");
    closeDrawers();
    loadCharacters();
  } catch (error) {
    showToast(error.message, "err");
  }
}

async function sendMail(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const attachmentType = form.elements.attachmentType.value;
  const payload = {
    receiveCharacNo: form.elements.receiveCharacNo.value.trim(),
    sendCharacName: form.elements.sendCharacName.value.trim(),
    itemId: numberOrZero(form.elements.itemId.value),
    count: numberOrZero(form.elements.count.value) || 1,
    upgrade: numberOrZero(form.elements.upgrade.value),
    seperateUpgrade: numberOrZero(form.elements.seperateUpgrade.value),
    amplifyOption: numberOrZero(form.elements.amplifyOption.value),
    amplifyValue: numberOrZero(form.elements.amplifyValue.value),
    gold: numberOrZero(form.elements.gold.value),
    letterId: numberOrZero(form.elements.letterId.value),
    endurance: numberOrZero(form.elements.endurance.value),
    message: form.elements.message.value.trim(),
    seal: form.elements.seal.checked,
    avatar: attachmentType === "avatar",
    creature: attachmentType === "creature"
  };
  try {
    const result = await api("/api/mail", {
      method: "POST",
      body: JSON.stringify(payload)
    });
    showToast(`邮件已发送，postal_id=${result.postalId}`, "ok");
    loadMail();
  } catch (error) {
    showToast(error.message, "err");
  }
}

async function loadMail() {
  const body = $("#mail-body");
  body.innerHTML = `<tr><td colspan="9" class="empty">加载中</td></tr>`;
  try {
    const params = formParams($("#mail-filter"));
    params.set("page", String(state.mail.page));
    const data = await api(`/api/mail?${params.toString()}`);
    state.mail = {
      page: data.page,
      pageSize: data.pageSize,
      hasMore: data.hasMore,
      items: data.items || []
    };
    renderMail();
    refreshHealth();
  } catch (error) {
    body.innerHTML = `<tr><td colspan="9" class="empty">${escapeHTML(error.message)}</td></tr>`;
  }
}

function renderMail() {
  const body = $("#mail-body");
  if (state.mail.items.length === 0) {
    body.innerHTML = `<tr><td colspan="9" class="empty">没有邮件记录</td></tr>`;
  } else {
    body.innerHTML = state.mail.items.map((item) => `
      <tr>
        <td>${item.postalId}</td>
        <td>${formatDate(item.occTime)}</td>
        <td title="${escapeHTML(item.sendCharacName)}">${escapeHTML(item.sendCharacName)}</td>
        <td>${escapeHTML(item.receiveCharacNo)}</td>
        <td>${item.itemId}${item.upgrade ? ` +${item.upgrade}` : ""}${item.seperateUpgrade ? ` / 锻${item.seperateUpgrade}` : ""}</td>
        <td>${mailCountText(item)}</td>
        <td>${item.gold}</td>
        <td>${mailTypeName(item)}</td>
        <td><button class="btn small danger" data-action="delete-mail" data-id="${item.postalId}">删除</button></td>
      </tr>
    `).join("");
  }
  $("#mail-page").textContent = `第 ${state.mail.page} 页`;
  $("#mail-prev").disabled = state.mail.page <= 1;
  $("#mail-next").disabled = !state.mail.hasMore;
}

function mailTypeName(item) {
  if (item.avatar) return "时装";
  if (item.creature) return "宠物";
  if (item.letterId && !item.itemId) return "信件";
  return "普通";
}

function mailCountText(item) {
  if (item.avatar || item.creature) {
    return `附件 ${item.addInfo || "-"}`;
  }
  return item.count;
}

async function deleteMail(id) {
  if (!confirm(`确定删除邮件 ${id} 吗？`)) return;
  try {
    await api(`/api/mail/${encodeURIComponent(id)}`, { method: "DELETE" });
    showToast("邮件已删除", "ok");
    loadMail();
  } catch (error) {
    showToast(error.message, "err");
  }
}

async function loadItems() {
  const body = $("#items-body");
  body.innerHTML = `<tr><td colspan="8" class="empty">加载中，首次解析 PVF 可能需要一些时间</td></tr>`;
  try {
    const params = formParams($("#item-filter"));
    params.set("page", String(state.items.page));
    const data = await api(`/api/items?${params.toString()}`);
    state.items = {
      page: data.page,
      pageSize: data.pageSize,
      total: data.total,
      items: data.items || [],
      facetsLoaded: true,
      category: state.items.category || "all"
    };
    populateItemFacets(data.facets);
    renderItems();
    refreshHealth();
  } catch (error) {
    body.innerHTML = `<tr><td colspan="8" class="empty">${escapeHTML(error.message)}</td></tr>`;
    showToast(error.message, "err");
  }
}

async function reloadItems() {
  const body = $("#items-body");
  body.innerHTML = `<tr><td colspan="8" class="empty">正在重新解析 PVF</td></tr>`;
  try {
    await api("/api/items/reload", { method: "POST", body: "{}" });
    state.items.page = 1;
    await loadItems();
    showToast("物品缓存已刷新", "ok");
  } catch (error) {
    body.innerHTML = `<tr><td colspan="8" class="empty">${escapeHTML(error.message)}</td></tr>`;
    showToast(error.message, "err");
  }
}

function renderItemCategoryBrowser() {
  const target = $("#item-category-browser");
  if (!target) return;
  const groups = [
    ["总览", ["all", "equipment_all", "stackable_all"]],
    ["装备", ["weapon", "titleName", "coat", "shoulder", "pants", "shoes", "waist", "amulet", "wrist", "ring", "support", "magicStone", "creature_equipment", "artifact_red", "artifact_green", "artifact_blue", "avatar"]],
    ["道具", ["waste", "material", "recipe", "material_expert_job", "quest", "booster", "feed", "creature_stackable", "throwItem", "legacy", "etc"]]
  ];
  const byKey = new Map(itemCategories.map((item) => [item.key, item]));
  target.innerHTML = groups.map(([label, keys]) => `
    <div class="category-group-ui">
      <span>${escapeHTML(label)}</span>
      <div>
        ${keys.map((key) => {
          const item = byKey.get(key);
          if (!item) return "";
          const active = state.items.category === key ? "active" : "";
          return `<button class="category-chip ${active}" type="button" data-category="${escapeHTML(key)}">${escapeHTML(item.label)}</button>`;
        }).join("")}
      </div>
    </div>
  `).join("");
  const current = byKey.get(state.items.category || "all");
  $("#item-browser-current").textContent = current ? current.label : "自定义筛选";
}

function applyItemCategory(key) {
  const category = itemCategories.find((item) => item.key === key) || itemCategories[0];
  const form = $("#item-filter");
  form.reset();
  for (const [name, value] of Object.entries(category.filters)) {
    if (form.elements[name]) {
      ensureSelectOption(form.elements[name], value);
      form.elements[name].value = value;
    }
  }
  state.items.category = category.key;
  state.items.page = 1;
  updateItemFilterAvailability();
  renderItemCategoryBrowser();
  loadItems();
}

function syncItemCategoryFromFilters() {
  const form = $("#item-filter");
  const filters = {
    type: form.elements.type.value || "all",
    equipmentType: form.elements.equipmentType.value,
    itemGroup: form.elements.itemGroup.value,
    stackableType: form.elements.stackableType.value,
    avatar: form.elements.avatar.value
  };
  const matched = itemCategories.find((category) => categoryFiltersMatch(category.filters, filters));
  state.items.category = matched ? matched.key : "custom";
}

function categoryFiltersMatch(categoryFilters, filters) {
  const keys = ["type", "equipmentType", "itemGroup", "stackableType", "avatar"];
  return keys.every((key) => (categoryFilters[key] || "") === (filters[key] || ""));
}

function ensureSelectOption(element, value) {
  if (!value || element.tagName !== "SELECT") return;
  if (Array.from(element.options).some((option) => option.value === value)) return;
  const option = document.createElement("option");
  option.value = value;
  option.textContent = value;
  element.appendChild(option);
}

function renderItems() {
  const body = $("#items-body");
  if (state.items.items.length === 0) {
    body.innerHTML = `<tr><td colspan="8" class="empty">没有匹配物品</td></tr>`;
  } else {
    body.innerHTML = state.items.items.map((item) => `
      <tr>
        <td>${renderIcon(item.icon)}</td>
        <td>
          <div class="main-cell">
            <strong title="${escapeHTML(item.name)}">${escapeHTML(item.name)}</strong>
            <span class="subtle">ID ${item.id}${item.pvfPath ? ` · ${escapeHTML(item.pvfPath)}` : ""}</span>
            ${item.icon?.path ? `<span class="subtle">IMG ${escapeHTML(item.icon.path)}#${item.icon.index || 0}</span>` : ""}
            ${item.explainPreview ? `<span class="subtle">${escapeHTML(item.explainPreview)}</span>` : ""}
          </div>
        </td>
        <td>${escapeHTML(item.equipmentType || item.stackableType || item.typeName)}${item.itemGroup ? `<div class="subtle">${escapeHTML(item.itemGroup)}</div>` : ""}</td>
        <td><span class="tag ${rarityClass(item.rarity)}">${escapeHTML(item.rarityName)}</span></td>
        <td>${item.minimumLevel}</td>
        <td>${item.stackLimit}</td>
        <td title="${escapeHTML(item.attachType)}">${escapeHTML(item.attachType)}</td>
        <td>
          <div class="row-actions">
            <button class="btn small" data-action="detail" data-type="${item.type}" data-id="${item.id}">详情</button>
            <button class="btn small secondary" data-action="mail-item" data-type="${item.type}" data-id="${item.id}">发货</button>
          </div>
        </td>
      </tr>
    `).join("");
  }
  const maxPage = Math.max(1, Math.ceil(state.items.total / state.items.pageSize));
  $("#items-page").textContent = `第 ${state.items.page} / ${maxPage} 页 · 共 ${state.items.total} 条`;
  $("#items-prev").disabled = state.items.page <= 1;
  $("#items-next").disabled = state.items.page >= maxPage;
  bindIconFallbacks();
}

async function openItemDrawer(type, id) {
  const requestId = ++state.itemDetailRequest;
  renderItemDetailLoading(type, id);
  openDrawer("#item-drawer");
  try {
    const item = await api(`/api/items/${encodeURIComponent(type)}/${encodeURIComponent(id)}`);
    if (requestId !== state.itemDetailRequest) return;
    renderItemDetail(item);
  } catch (error) {
    if (requestId !== state.itemDetailRequest) return;
    renderItemDetailError(type, id, error);
    showToast(error.message, "err");
  }
}

function renderItemDetailLoading(type, id) {
  $("#item-drawer-title").textContent = "物品详情";
  $("#item-drawer-subtitle").textContent = `ID ${id} · ${typeName(type)}`;
  $("#item-detail").innerHTML = `
    <div class="muted-panel detail-loading">
      <span class="loader"></span>
      <strong>正在读取物品详情</strong>
      <span>包含 PVF 原始文本和字段，可能需要等待片刻。</span>
    </div>
  `;
}

function renderItemDetailError(type, id, error) {
  $("#item-drawer-title").textContent = "物品详情";
  $("#item-drawer-subtitle").textContent = `ID ${id} · ${typeName(type)}`;
  $("#item-detail").innerHTML = `
    <div class="muted-panel detail-loading error">
      <strong>读取失败</strong>
      <span>${escapeHTML(error.message)}</span>
    </div>
  `;
}

function renderItemDetail(item) {
  $("#item-drawer-title").textContent = item.name || "物品详情";
  $("#item-drawer-subtitle").textContent = `ID ${item.id} · ${typeName(item.type)}`;
  const iconFacts = item.icon ? [
    ["图标 IMG", item.icon.path || ""],
    ["图标帧", item.icon.index ?? 0],
    ["图标接口", item.iconUrl || `/api/items/icon?path=${encodeURIComponent(item.icon.path || "")}&index=${encodeURIComponent(item.icon.index || 0)}`]
  ] : [];
  const stats = Object.entries(item)
    .filter(([key, value]) => !commonItemKeys.has(key) && hasValue(value))
    .map(([key, value]) => `
      <div class="stat">
        <span>${escapeHTML(statLabels[key] || camelLabel(key))}</span>
        <strong>${escapeHTML(formatValue(value))}</strong>
      </div>
    `)
    .join("");

  $("#item-detail").innerHTML = `
    <div class="item-detail-head">
      ${renderIcon(item.icon)}
      <div class="main-cell">
        <strong>${escapeHTML(item.name || "")}</strong>
        <span class="subtle">${escapeHTML(item.rarityName || "")} · ${escapeHTML(item.attachType || "")}</span>
      </div>
    </div>
    <div class="detail-facts">
      <div class="fact"><span>物品 ID</span><strong>${item.id}</strong></div>
      <div class="fact"><span>类型</span><strong>${escapeHTML(typeName(item.type))}</strong></div>
      <div class="fact"><span>等级</span><strong>${item.minimumLevel ?? 0}</strong></div>
      <div class="fact"><span>携带上限</span><strong>${item.stackLimit ?? 1}</strong></div>
      <div class="fact"><span>可用职业</span><strong>${escapeHTML(formatValue(item.usableJobs || []))}</strong></div>
      <div class="fact"><span>稀有度</span><strong>${escapeHTML(item.rarityName || "")}</strong></div>
      <div class="fact"><span>PVF 文件</span><strong>${escapeHTML(item.pvfPath || "-")}</strong></div>
      ${iconFacts.map(([label, value]) => `<div class="fact"><span>${escapeHTML(label)}</span><strong>${escapeHTML(formatValue(value) || "-")}</strong></div>`).join("")}
    </div>
    <div class="row-actions">
      <button class="btn primary" type="button" data-detail-mail="${item.id}">填入发货</button>
    </div>
    ${item.description ? `<div><h3>描述</h3><div class="text-block">${escapeHTML(item.description)}</div></div>` : ""}
    ${item.explain ? `<div><h3>说明</h3><div class="text-block">${escapeHTML(item.explain)}</div></div>` : ""}
    ${stats ? `<div><h3>属性</h3><div class="stat-grid">${stats}</div></div>` : ""}
    ${item.pvfError ? `<div><h3>PVF 读取错误</h3><div class="text-block">${escapeHTML(item.pvfError)}</div></div>` : ""}
    ${item.pvfSource ? `<div><h3>原始 PVF 源文本</h3><pre class="code-block">${escapeHTML(item.pvfSource)}</pre></div>` : ""}
    ${item.pvfFields ? `<div><h3>解析字段 JSON</h3><pre class="code-block">${escapeHTML(JSON.stringify(item.pvfFields, null, 2))}</pre></div>` : ""}
  `;
  $("#item-detail [data-detail-mail]").addEventListener("click", () => fillMailItem(item.id));
  bindIconFallbacks();
}

function fillMailItem(id) {
  $("#mail-form [name='itemId']").value = String(id);
  if (!$("#mail-form [name='count']").value) {
    $("#mail-form [name='count']").value = "1";
  }
  closeDrawers();
  setView("mail");
  $("#mail-form [name='receiveCharacNo']").focus();
  showToast(`已填入物品 ID ${id}`, "ok");
}

function populateItemFacets(facets) {
  if (!facets) return;
  fillSelect("#rarity-select", facets.rarities || [], (item) => String(item.value), (item) => item.label);
  fillSelect("#equipment-type-select", facets.equipmentTypes || []);
  fillSelect("#item-group-select", facets.itemGroups || []);
  fillSelect("#stackable-type-select", facets.stackableTypes || []);
  updateItemFilterAvailability();
}

function fillSelect(selector, values, getValue = (item) => item, getLabel = (item) => item) {
  const select = $(selector);
  const previous = select.value;
  const first = select.options[0];
  select.innerHTML = "";
  select.appendChild(first);
  values.forEach((item) => {
    const option = document.createElement("option");
    option.value = getValue(item);
    option.textContent = getLabel(item);
    select.appendChild(option);
  });
  if (previous && !Array.from(select.options).some((option) => option.value === previous)) {
    const option = document.createElement("option");
    option.value = previous;
    option.textContent = previous;
    select.appendChild(option);
  }
  if (previous) {
    select.value = previous;
  }
}

function updateItemFilterAvailability() {
  const type = $("#item-type").value;
  const equipmentDisabled = type === "stackable";
  const stackableDisabled = type === "equipment";
  $("#equipment-type-select").disabled = equipmentDisabled;
  $("#item-group-select").disabled = equipmentDisabled;
  $("#item-filter [name='avatar']").disabled = equipmentDisabled;
  $("#stackable-type-select").disabled = stackableDisabled;
  if (equipmentDisabled) {
    $("#equipment-type-select").value = "";
    $("#item-group-select").value = "";
    $("#item-filter [name='avatar']").value = "";
  }
  if (stackableDisabled) {
    $("#stackable-type-select").value = "";
  }
}

function openDrawer(selector) {
  $("#drawer-backdrop").classList.remove("hidden");
  $$(".drawer").forEach((drawer) => {
    drawer.classList.toggle("open", `#${drawer.id}` === selector);
    drawer.setAttribute("aria-hidden", drawer.classList.contains("open") ? "false" : "true");
  });
}

function closeDrawers() {
  $("#drawer-backdrop").classList.add("hidden");
  $$(".drawer").forEach((drawer) => {
    drawer.classList.remove("open");
    drawer.setAttribute("aria-hidden", "true");
  });
}

function formParams(form) {
  const params = new URLSearchParams();
  Array.from(new FormData(form).entries()).forEach(([key, value]) => {
    const text = String(value).trim();
    if (text !== "") params.set(key, text);
  });
  return params;
}

function renderIcon(icon) {
  if (!icon || !icon.path) {
    return `<div class="icon-slot">无</div>`;
  }
  const src = `/api/items/icon?path=${encodeURIComponent(icon.path)}&index=${encodeURIComponent(icon.index || 0)}`;
  return `<div class="icon-slot"><img class="item-icon" src="${src}" alt=""></div>`;
}

function bindIconFallbacks() {
  $$(".item-icon").forEach((img) => {
    img.addEventListener("error", () => {
      const slot = img.closest(".icon-slot");
      if (slot) slot.textContent = "无";
    }, { once: true });
  });
}

function showToast(message, type = "ok") {
  const toast = document.createElement("div");
  toast.className = `toast ${type}`;
  toast.textContent = message;
  $("#toast-stack").appendChild(toast);
  setTimeout(() => toast.remove(), 4200);
}

function numberOrZero(value) {
  const number = Number(value);
  return Number.isFinite(number) ? number : 0;
}

function formatDate(value) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(date);
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function hasValue(value) {
  if (value === null || value === undefined) return false;
  if (Array.isArray(value)) return value.length > 0;
  if (typeof value === "string") return value.trim() !== "";
  return true;
}

function formatValue(value) {
  if (Array.isArray(value)) return value.join(" / ");
  if (typeof value === "boolean") return value ? "是" : "否";
  if (value && typeof value === "object") return JSON.stringify(value);
  return String(value ?? "");
}

function camelLabel(key) {
  return key.replace(/[A-Z]/g, (match) => ` ${match}`).replace(/^./, (match) => match.toUpperCase());
}

function typeName(type) {
  if (type === "equipment") return "装备";
  if (type === "stackable") return "道具";
  return type || "其他";
}

function rarityClass(rarity) {
  if (rarity >= 4) return "warn";
  if (rarity >= 2) return "ok";
  return "";
}
