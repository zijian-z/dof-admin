const state = {
  view: "characters",
  health: null,
  characters: { page: 1, pageSize: 20, total: 0, items: [] },
  mail: { page: 1, pageSize: 20, hasMore: false, items: [] },
  items: { page: 1, pageSize: 40, total: 0, items: [], facetsLoaded: false },
  selectedCharacter: null
};

const viewMeta = {
  characters: {
    title: "角色管理",
    subtitle: "查询角色、编辑常用属性，并快速跳转发货。"
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
  "icon"
]);

const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => Array.from(root.querySelectorAll(selector));

document.addEventListener("DOMContentLoaded", () => {
  bindNavigation();
  bindCharacters();
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
    if (button.dataset.action === "mail") {
      $("#mail-form [name='receiveCharacNo']").value = String(row.characNo);
      setView("mail");
      showToast(`已填入收件角色 ${row.characName}`, "ok");
    }
  });
  $("#character-form").addEventListener("submit", saveCharacter);
}

function bindMail() {
  $("#mail-form").addEventListener("submit", sendMail);
  $("#clear-mail-form").addEventListener("click", () => {
    $("#mail-form").reset();
    $("#mail-form [name='count']").value = "1";
    $("#mail-form [name='gold']").value = "0";
    $("#mail-form [name='letterId']").value = "0";
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
  $("#reset-create-limit").addEventListener("click", resetCreateLimit);
}

function bindItems() {
  $("#item-filter").addEventListener("submit", (event) => {
    event.preventDefault();
    state.items.page = 1;
    loadItems();
  });
  $("#reset-item-filter").addEventListener("click", () => {
    $("#item-filter").reset();
    state.items.page = 1;
    updateItemFilterAvailability();
    loadItems();
  });
  $("#refresh-items").addEventListener("click", loadItems);
  $("#item-type").addEventListener("change", updateItemFilterAvailability);
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
      ["装备数量", health.items.equipmentCount],
      ["道具数量", health.items.stackableCount],
      ["加载时间", health.items.loadedAt ? formatDate(health.items.loadedAt) : ""],
      ["错误", health.items.error || ""]
    ]),
    healthPanel("NPK 图标", health.npk.ready, [
      ["路径", health.npk.detail || "未配置"],
      ["状态", health.npk.ready ? "已加载" : "按需加载"],
      ["错误", health.npk.error || ""]
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
          <code title="${escapeHTML(String(value ?? ""))}">${escapeHTML(String(value ?? "")) || "-"}</code>
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
        <td>${item.mid}</td>
        <td>${item.job}</td>
        <td>${item.lev}</td>
        <td>${item.hp} / ${item.maxHp} · MP ${item.maxMp}</td>
        <td>${item.attackSpeed} / ${item.castSpeed} / ${item.moveSpeed}</td>
        <td>${item.fatigue}</td>
        <td>${formatDate(item.createTime)}</td>
        <td>
          <div class="row-actions">
            <button class="btn small" data-action="edit" data-id="${item.characNo}">编辑</button>
            <button class="btn small secondary" data-action="mail" data-id="${item.characNo}">发货</button>
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
    seal: form.elements.seal.checked
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
  body.innerHTML = `<tr><td colspan="7" class="empty">加载中</td></tr>`;
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
    body.innerHTML = `<tr><td colspan="7" class="empty">${escapeHTML(error.message)}</td></tr>`;
  }
}

function renderMail() {
  const body = $("#mail-body");
  if (state.mail.items.length === 0) {
    body.innerHTML = `<tr><td colspan="7" class="empty">没有邮件记录</td></tr>`;
  } else {
    body.innerHTML = state.mail.items.map((item) => `
      <tr>
        <td>${item.postalId}</td>
        <td>${formatDate(item.occTime)}</td>
        <td title="${escapeHTML(item.sendCharacName)}">${escapeHTML(item.sendCharacName)}</td>
        <td>${escapeHTML(item.receiveCharacNo)}</td>
        <td>${item.itemId}${item.upgrade ? ` +${item.upgrade}` : ""}${item.seperateUpgrade ? ` / 锻${item.seperateUpgrade}` : ""}</td>
        <td>${item.count}</td>
        <td>${item.gold}</td>
      </tr>
    `).join("");
  }
  $("#mail-page").textContent = `第 ${state.mail.page} 页`;
  $("#mail-prev").disabled = state.mail.page <= 1;
  $("#mail-next").disabled = !state.mail.hasMore;
}

async function resetCreateLimit() {
  if (!confirm("确定清空角色创建限制表吗？")) return;
  try {
    await api("/api/reset-create-limit", { method: "POST", body: "{}" });
    showToast("角色创建限制已重置", "ok");
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
      facetsLoaded: true
    };
    populateItemFacets(data.facets);
    renderItems();
    refreshHealth();
  } catch (error) {
    body.innerHTML = `<tr><td colspan="8" class="empty">${escapeHTML(error.message)}</td></tr>`;
    showToast(error.message, "err");
  }
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
            <span class="subtle">ID ${item.id}${item.explainPreview ? ` · ${escapeHTML(item.explainPreview)}` : ""}</span>
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
  try {
    const item = await api(`/api/items/${encodeURIComponent(type)}/${encodeURIComponent(id)}`);
    renderItemDetail(item);
    openDrawer("#item-drawer");
  } catch (error) {
    showToast(error.message, "err");
  }
}

function renderItemDetail(item) {
  $("#item-drawer-title").textContent = item.name || "物品详情";
  $("#item-drawer-subtitle").textContent = `ID ${item.id} · ${typeName(item.type)}`;
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
    </div>
    <div class="row-actions">
      <button class="btn primary" type="button" data-detail-mail="${item.id}">填入发货</button>
    </div>
    ${item.description ? `<div><h3>描述</h3><div class="text-block">${escapeHTML(item.description)}</div></div>` : ""}
    ${item.explain ? `<div><h3>说明</h3><div class="text-block">${escapeHTML(item.explain)}</div></div>` : ""}
    ${stats ? `<div><h3>属性</h3><div class="stat-grid">${stats}</div></div>` : ""}
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
  if (Array.from(select.options).some((option) => option.value === previous)) {
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
