// RAD-Manager panel: one page, hash routes, same look as the old ADV panel.
"use strict";
const $ = (s, el = document) => el.querySelector(s);
const esc = v => String(v ?? "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
const fmt = n => Number(n || 0).toLocaleString("en-US", { maximumFractionDigits: 2 });
let ME = null, PROFILES = [], MANAGERS = [];

// CSV that Excel opens with Arabic intact (BOM)
function downloadCSV(name, header, rows) {
  const cell = v => `"${String(v ?? "").replace(/"/g, '""')}"`;
  const csv = "﻿" + [header.map(cell).join(",")].concat(rows.map(r => r.map(cell).join(","))).join("\n");
  const a = document.createElement("a"); a.href = URL.createObjectURL(new Blob([csv], { type: "text/csv" })); a.download = name + ".csv"; a.click();
}

async function api(path, opt = {}) {
  const o = { method: opt.method || "GET", headers: { "X-Requested-With": "rm", "X-Page": (location.hash.slice(1) || "dashboard").split("?")[0] }, credentials: "same-origin" };
  if (opt.body !== undefined) { o.body = JSON.stringify(opt.body); o.headers["Content-Type"] = "application/json"; }
  const r = await fetch("/api/" + path, o);
  let j = null; try { j = await r.json(); } catch (_) {}
  if (r.status === 401 && path !== "login") { showLogin(); throw new Error("سجل دخول"); }
  if (!r.ok) throw new Error((j && j.error) || ("HTTP " + r.status));
  return j;
}
function toast(msg, bad) { const t = $("#toast"); t.textContent = msg; t.className = "toast" + (bad ? " bad" : ""); clearTimeout(t._h); t._h = setTimeout(() => t.classList.add("hidden"), 4000); }
function modal(html) { $("#modalBody").innerHTML = html; $("#modal").classList.remove("hidden"); return $("#modalBody"); }
function closeModal() { $("#modal").classList.add("hidden"); }
document.addEventListener("click", e => { if (e.target.matches("[data-close]") || e.target.id === "modal") closeModal(); });
async function act(fn, okMsg) { try { const r = await fn(); if (okMsg) toast(okMsg); return r; } catch (e) { toast(e.message, true); throw e; } }
function formData(form) { const o = {}; new FormData(form).forEach((v, k) => { o[k] = v; }); return o; }
const statusTag = s => {
  if (+s.enabled === 0) return '<span class="tag gray">موقوف</span>';
  if (!s.expiration) return '<span class="tag gray">بدون تاريخ</span>';
  return new Date(s.expiration.replace(" ", "T")) < new Date() ? '<span class="tag red">منتهي</span>' : '<span class="tag green">فعال</span>';
};
const onlineTag = s => +s.online ? '<span class="tag blue">متصل</span>' : "";

// ---------- menu (same order as the old panel)
const MENU = [ // same order and sub-menus as ADV's sidebar
  ["dashboard", "fa-dashboard", "لوحة التحكم"],
  ["subscribers", "fa-users", "مشتركين", false, [
    ["subscribers?view=overview", "fa-th-large", "نظرة عامة"], ["subscribers", "fa-list", "قائمة المشتركين"], ["user?new=1", "fa-user-plus", "إضافة مشترك"],
    ["online", "fa-lightbulb-o", "المشتركين المتصلين"], ["appusers", "fa-mobile", "مستخدمو التطبيق المتصلون"], ["cardusers", "fa-credit-card", "مشتركين الكارت"],
    ["onlinecards", "fa-credit-card", "الكروت المتصلة"]]],
  ["profiles", "fa-gift", "العروض", false, [["profiles?view=overview", "fa-th-large", "نظرة عامة"], ["profiles", "fa-list", "قائمة العروض"],
    ["profile", "fa-plus", "إضافة عرض"], ["ippool", "fa-sitemap", "IP Pool"]]],
  ["tickets", "fa-exclamation-circle", "الشكاوى", false, [["tickets?view=overview", "fa-th-large", "نظرة عامة"], ["tickets", "fa-list", "قائمة الشكاوى"]]],
  ["managers", "fa-university", "المدراء", false, [["managers?view=overview", "fa-th-large", "نظرة عامة"], ["managers", "fa-list", "قائمة المدراء"],
    ["onlinemgr", "fa-circle", "المدراء المتصلين"], ["managers?add=1", "fa-plus", "إضافة مدير"], ["pricing", "fa-tags", "اسعار العروض"], ["perms", "fa-key", "صلاحيات الموزعين"]]],
  ["cards", "fa-credit-card", "كروت الشحن", false, [["cards", "fa-credit-card", "كروت الشحن"], ["cardtpl", "fa-paint-brush", "تصميم الكروت والطباعة"]]],
  ["nas", "fa-paper-plane", "NAS"],
  ["cafe", "fa-coffee", "كروت الكافيهات"],
  ["vpn", "fa-lock", "VPN الكافيهات"],
  ["ports", "fa-random", "توجيه البورتات"],
  ["reports", "fa-pie-chart", "تقارير"],
  ["smart", "fa-magic", "المزايا الذكية"],
  ["bandwidth", "fa-tachometer", "مراقبة الباندويث"],
  ["speed", "fa-bolt", "التحكم في السرعات"],
  ["charge", "fa-money", "لوحة الشحن"],
  ["cash", "fa-money", "معاملات الكاش"],
  ["tools", "fa-wrench", "الادوات", false, [["backup", "fa-database", "نسخ إحتياطية & استرجاعها"], ["sync", "fa-google", "Google Drive / المزامنة مع ADV"], ["genadj", "fa-edit", "تعديلات عامة"],
    ["trash", "fa-trash", "سلة المحذوفات"], ["messages", "fa-envelope", "سجل الرسائل"], ["maintenance", "fa-cogs", "صيانة النظام"], ["invoices", "fa-file-text-o", "الفواتير"], ["audit", "fa-history", "سجل العمليات"], ["admins", "fa-user-secret", "مستخدمين اللوحة"]]],
  ["settings", "fa-cog", "الإعدادات"],
];
// ADV permission number each menu entry needs, for distributor logins that carry their ADV group (ME.admin.perms)
const MENU_PERM = { subscribers: 1, "subscribers?view=overview": 1, "user?new=1": 2, online: 6, appusers: 6, cardusers: 9, onlinecards: 12, profiles: 15, ippool: 15, profile: 16,
  tickets: 136, managers: 20, onlinemgr: 20, "managers?add=1": 21, pricing: 30, perms: 33, cards: 37, nas: 42, ports: 42, reports: 47, smart: 0, bandwidth: 0, charge: 100 };
// Bootstrap Icons + a colour per section (look taken from Haitham's system); anything not listed keeps its Font Awesome icon
const MENU_BI = {
  dashboard: ["bi-grid-1x2-fill", "mi-indigo"], subscribers: ["bi-people-fill", "mi-indigo"], profiles: ["bi-tags-fill", "mi-amber"], tickets: ["bi-exclamation-octagon-fill", "mi-rose"],
  managers: ["bi-shop-window", "mi-sky"], cards: ["bi-credit-card-2-front-fill", "mi-amber"], nas: ["bi-hdd-network-fill", "mi-green"], ports: ["bi-shuffle", "mi-green"],
  reports: ["bi-file-earmark-bar-graph-fill", "mi-indigo"], smart: ["bi-stars", "mi-violet"], bandwidth: ["bi-speedometer2", "mi-sky"], speed: ["bi-lightning-charge-fill", "mi-amber"],
  charge: ["bi-wallet2", "mi-green"], tools: ["bi-tools", "mi-slate"], settings: ["bi-gear-wide-connected", "mi-slate"], vpn: ["bi-shield-lock-fill", "mi-sky"], cafe: ["bi-cup-hot-fill", "mi-amber"], cash: ["bi-cash-coin", "mi-green"],
  "subscribers?view=overview": ["bi-grid"], "subscribers": ["bi-people-fill"], "user?new=1": ["bi-person-plus-fill"], online: ["bi-broadcast"], appusers: ["bi-phone"],
  cardusers: ["bi-person-badge"], onlinecards: ["bi-credit-card"], "profiles?view=overview": ["bi-grid"], profile: ["bi-plus-circle"], ippool: ["bi-diagram-3"],
  "tickets?view=overview": ["bi-grid"], "managers?view=overview": ["bi-grid"], onlinemgr: ["bi-circle-fill"], "managers?add=1": ["bi-plus-circle"], pricing: ["bi-tags"],
  perms: ["bi-key-fill"], backup: ["bi-cloud-arrow-down-fill"], sync: ["bi-arrow-left-right"], genadj: ["bi-pencil-square"], trash: ["bi-trash3"],
  messages: ["bi-envelope-fill"], maintenance: ["bi-wrench-adjustable"], invoices: ["bi-receipt"], audit: ["bi-clock-history"], admins: ["bi-person-lock"],
  portal: ["bi-person-workspace"], cardtpl: ["bi-palette-fill"], cafe: ["bi-cup-hot-fill"], radiuslog: ["bi-hdd-network"], traffic: ["bi-graph-up-arrow"],
};
const menuIcon = (k, fa, sub) => {
  const b = MENU_BI[k] || (sub ? MENU_BI[k.split("?")[0]] : null);
  return b ? `<i class="bi ${b[0]} ${b[1] || ""}"></i>` : `<i class="fa ${fa}"></i>`;
};
const can = p => !ME.admin.perms || p === undefined || (p > 0 && ME.admin.perms.includes(p));
function drawMenu() {
  const adminOnly = ["tools", "settings", "speed", "vpn", "cafe", "cash"];
  const adminKids = ["cardtpl"];
  const ok = k => can(MENU_PERM[k] ?? MENU_PERM[k.split("?")[0]]);
  $("#menu").innerHTML = MENU.filter(([k]) => (ME.admin.role === "admin" || !adminOnly.includes(k)) && ok(k)).map(([k, ic, t, soon, kids]) =>
    `<a href="#${k}" data-k="${k}" class="${soon ? "soon" : ""}">${menuIcon(k, ic)}${t}${kids ? '<i class="fa fa-angle-down caret"></i>' : ""}</a>` +
    (kids ? `<div class="sub" data-parent="${k}">${kids.filter(([h]) => ok(h) && (ME.admin.role === "admin" || !adminKids.includes(h))).map(([h, i2, t2]) => `<a href="#${h}" data-h="${h}">${menuIcon(h, i2, true)}${t2}</a>`).join("")}</div>` : "")).join("");
}

// ---------- login
function showLogin() { $("#app").classList.add("hidden"); $("#login").classList.remove("hidden"); }
$("#loginForm").addEventListener("submit", async e => {
  e.preventDefault(); $("#loginErr").textContent = "";
  try { await api("login", { method: "POST", body: formData(e.target) }); boot(); }
  catch (err) { $("#loginErr").textContent = err.message; }
});
$("#logout").onclick = async () => { await api("logout", { method: "POST" }); showLogin(); };
$("#burger").onclick = () => $("#app").classList.toggle("menu-open");

async function boot() {
  try { ME = await api("me"); } catch (_) { showLogin(); return; }
  $("#login").classList.add("hidden"); $("#app").classList.remove("hidden");
  $("#who").textContent = ME.admin.full_name || ME.admin.username;
  $("#brand").textContent = ME.company || "Stream-Net";
  const mb = $("#modeBadge"); mb.className = "mode " + ME.mode;
  mb.textContent = ME.mode === "shadow" ? "وضع المراقبة (Shadow) — ADV هو الشغال" : "وضع التشغيل (Live)";
  if (ME.sim) { mb.className = "mode shadow"; mb.textContent = "🧪 لوحة التجربة — نسخة، مش بتكلم أي راوتر ولا ADV"; }
  drawMenu();
  [PROFILES, MANAGERS] = await Promise.all([api("profiles").catch(() => []), api("managers").catch(() => [])]);
  route();
}
window.addEventListener("hashchange", route);
function route() {
  const k = (location.hash.slice(1) || "dashboard").split("?")[0];
  const full = location.hash.slice(1) || "dashboard";
  const owns = s => s.dataset.parent === k || [...s.querySelectorAll("a")].some(a => a.dataset.h.split("?")[0] === k);
  document.querySelectorAll("#menu .sub").forEach(s => s.classList.toggle("open", owns(s)));
  document.querySelectorAll("#menu > a").forEach(a => { const sub = a.nextElementSibling; a.classList.toggle("active", a.dataset.k === k || !!(sub && sub.classList.contains("sub") && owns(sub))); });
  document.querySelectorAll("#menu .sub a").forEach(a => a.classList.toggle("cur", a.dataset.h === full));
  $("#app").classList.remove("menu-open");
  (PAGES[k] || PAGES.soon)().catch(e => { $("#content").innerHTML = `<div class="panel"><div class="empty">${esc(e.message)}</div></div>`; });
}
const C = html => { $("#content").innerHTML = html; };
const options = (list, sel, label = "name", none = "—") => `<option value="0">${none}</option>` + list.map(x => `<option value="${x.id}" ${+sel === +x.id ? "selected" : ""}>${esc(x[label])}</option>`).join("");

// ---------- pages
const PAGES = {};
PAGES.soon = async () => C('<div class="panel"><div class="empty"><i class="fa fa-wrench fa-2x"></i><br><br>الصفحة دي هتتعمل في المرحلة الجاية</div></div>');

PAGES.dashboard = async () => {
  const d = await api("dashboard");
  const box = (cls, num, lbl, ico, link) => `<div class="box ${cls}"><div class="in"><div><div class="num big">${num}</div><div class="lbl">${lbl}</div></div><i class="fa ${ico} ico"></i></div><a class="more" href="#${link || ""}"><i class="fa fa-arrow-circle-left"></i> تفاصيل أخرى</a></div>`;
  const inv = (title, x) => `<div class="inv"><div class="head"><i class="fa fa-refresh"></i> ${title}</div><div class="body"><i class="fa fa-money ico"></i><dl>
      <dt>إجمالي الفواتير المدفوعة</dt><dd>${fmt(x.paid)}</dd><dt>إجمالي الفواتير الغير مدفوعة</dt><dd>${fmt(x.unpaid)}</dd>
      <dt>محصلة الفواتير (العدد)</dt><dd>${fmt(x.count)}</dd><dt>محصلة الفواتير اليومية</dt><dd>${fmt(x.today)}</dd><dt>محصلة الفواتير الشهرية</dt><dd>${fmt(x.month)}</dd></dl></div></div>`;
  const online = d.mode === "shadow" ? d.adv_online : d.online;
  if (d.mode === "shadow") { // mirror mode: money boxes show ADV's own numbers (read-only metrics from the last sync)
    try {
      const m = (await api("sync")).adv_metrics || {}, qs = m.quick_stats || {}, cs = m.get_cards_sales_stats || {};
      d.inv_subscribers = { paid: qs.today_sales_total || 0, unpaid: qs.unpaid_total || 0, count: qs.unpaid_invoices_count || 0, today: qs.today_sales_total || 0, month: d.inv_subscribers.month };
      d.inv_cards = { paid: (cs.year || {}).total || 0, unpaid: 0, count: (cs.year || {}).count || 0, today: (cs.day || {}).total || 0, month: (cs.month || {}).total || 0 };
      if ((cs.year || {}).count) d.cards = cs.year.count;
    } catch (_) {}
  }
  C(`${d.mode === "shadow" ? `<div class="alert"><b><i class="fa fa-warning"></i> وضع المراقبة!</b>النظام ده بيقرا من ADV بس ومش بيكلم أي سيرفر. آخر مزامنة: ${esc(d.last_sync || "لسه")} — الحالة: ${esc(d.sync_state)}</div>` : ""}
  <div class="grid g2">${box("blue", fmt(online), "المشتركين المتصلين", "fa-lightbulb-o", "online")}${box("blue", fmt(d.subscribers), "مشتركين", "fa-users", "subscribers")}</div>
  <div class="grid g3">${box("orange", fmt(d.expiring), `مشتركين ينتهي اشتراكهم خلال ${d.expiring_days} ايام`, "fa-meh-o", "subscribers?status=expiring")}
    ${box("amber", fmt(d.expired), "المشتركين المنتهي اشتراكهم", "fa-frown-o", "subscribers?status=expired")}
    ${box("green", fmt(d.active), "المشتركين المتاحين", "fa-smile-o", "subscribers?status=active")}</div>
  <div class="grid g2">${box("purple", "EGP " + fmt(d.received), "إجمالي قيم الإستلام", "fa-database", "invoices")}${box("green", typeof d.balance === "number" ? fmt(d.balance) : esc(d.balance), "الرصيد", "fa-money", "managers")}</div>
  <div class="grid g2">${box("blue", fmt(d.cards_online), "الكروت المتصلة", "fa-credit-card", "cards")}${box("blue", fmt(d.cards), "كروت", "fa-credit-card", "cards")}</div>
  <div class="grid g3">${inv("إجمالي الفواتير (الكروت)", d.inv_cards)}${inv("إجمالي الفواتير (المشتركين)", d.inv_subscribers)}${inv("إجمالي الفواتير (المدراء)", d.inv_managers)}</div>
  <div class="grid g4">${box(d.radius_ok ? "blue" : "red", d.radius_ok ? "شغال" : "واقف", "خدمة الراديوس (FreeRADIUS)", "fa-signal", "settings")}
    ${box("blue", esc(d.uptime), "مدة تشغيل النظام", "fa-clock-o")}${box("blue", esc(d.now), "الوقت داخل النظام", "fa-clock-o")}${box("blue", esc(d.version), "إصدار النظام", "fa-star")}</div>`);
};

function subForm(s) {
  s = s || {};
  return `<form class="form" id="sf">
    <label>اسم المستخدم<input name="username" value="${esc(s.username)}" ${s.id ? "readonly" : "required"}></label>
    <label>الموبايل<input name="mobile" value="${esc(s.mobile)}"></label>
    <label>الاسم الأول<input name="firstname" value="${esc(s.firstname)}"></label>
    <label>الاسم الأخير<input name="lastname" value="${esc(s.lastname)}"></label>
    <label>الباقة<select name="profile_id">${options(PROFILES, s.profile_id)}</select></label>
    <label>المدير<select name="manager_id">${options(MANAGERS, s.manager_id, "username")}</select></label>
    <label>تاريخ الانتهاء<input name="expiration" type="datetime-local" value="${esc((s.expiration || "").replace(" ", "T").slice(0, 16))}"></label>
    <label>عدد الأجهزة في نفس الوقت<input name="simul" type="number" min="1" value="${esc(s.simul || 1)}"></label>
    <label>MAC<input name="mac" value="${esc(s.mac)}" dir="ltr"></label>
    <label>IP ثابت<input name="framed_ip" value="${esc(s.framed_ip)}" dir="ltr"></label>
    <label class="full">العنوان<input name="address" value="${esc(s.address)}"></label>
    <label class="full">ملاحظات<textarea name="notes" rows="2">${esc(s.notes)}</textarea></label>
    <div class="actions full"><button class="green">حفظ</button><button type="button" class="gray" data-close>إلغاء</button></div></form>`;
}
function editSubscriber(s, after) {
  const m = modal(`<h3>${s ? "تعديل " + esc(s.username) : "مشترك جديد"}</h3>` + subForm(s));
  $("#sf", m).onsubmit = async e => {
    e.preventDefault();
    const f = formData(e.target);
    const body = { Username: f.username, Firstname: f.firstname, Lastname: f.lastname, Mobile: f.mobile, Address: f.address, Notes: f.notes, Mac: f.mac,
      FramedIP: f.framed_ip, Expiration: f.expiration ? f.expiration.replace("T", " ") : "", ProfileID: +f.profile_id, ManagerID: +f.manager_id, Simul: +f.simul };
    await act(() => api(s ? "subscribers/" + s.id : "subscribers", { method: s ? "PUT" : "POST", body }), "اتحفظ");
    closeModal(); after && after();
  };
}
async function openSubscriber(id, after) {
  const d = await api("subscribers/" + id), s = d.subscriber;
  const m = modal(`<h3><i class="fa fa-user"></i> ${esc(s.username)} ${statusTag(s)} ${onlineTag(s)}</h3>
    <div class="kv"><div>الاسم</div><div>${esc(s.firstname)} ${esc(s.lastname)}</div><div>الموبايل</div><div>${esc(s.mobile)}</div>
      <div>الباقة</div><div>${esc(s.profile || "—")}</div><div>المدير</div><div>${esc(s.manager || "—")}</div><div>ينتهي</div><div>${esc(s.expiration || "—")}</div>
      <div>الرصيد</div><div>${fmt(s.money)}</div><div>MAC</div><div dir="ltr">${esc(s.mac || "—")}</div><div>المصدر</div><div>${s.source === "adv" ? "منسوخ من ADV" : "محلي"}</div>
      <div>العنوان</div><div>${esc(s.address)}</div><div>ملاحظات</div><div>${esc(s.notes)}</div></div>
    <div class="actions"><button id="bRenew" class="green"><i class="fa fa-refresh"></i> تجديد</button><button id="bExt" class="orange"><i class="fa fa-clock-o"></i> تمديد أيام</button>
      <button id="bEdit"><i class="fa fa-edit"></i> تعديل</button><button id="bTog" class="${+s.enabled ? "red" : "green"}">${+s.enabled ? "إيقاف" : "تفعيل"}</button></div>
    <h3 style="margin-top:18px">الفواتير</h3><div class="tablewrap">${table(d.invoices, [["created", "التاريخ"], ["operation", "العملية"], ["amount", "المبلغ"], ["discount", "خصم"], ["paid", "مدفوع", v => +v ? "✔" : "✘"], ["period_to", "لحد"], ["created_by", "بواسطة"]])}</div>
    <h3>الجلسات</h3><div class="tablewrap">${table(d.adv_online.length ? d.adv_online : d.sessions, d.adv_online.length ? [["nas", "NAS"], ["ip", "IP"], ["mac", "MAC"], ["since", "من"]] :
      [["acctstarttime", "بداية"], ["acctstoptime", "نهاية"], ["nasipaddress", "NAS"], ["framedipaddress", "IP"], ["callingstationid", "MAC"]])}</div>
    <h3>إعدادات الراديوس</h3><div class="tablewrap">${table(d.radius, [["t", "النوع"], ["attribute", "الخاصية"], ["op", ""], ["value", "القيمة"]])}</div>
    <h3>سجل العمليات</h3><div class="tablewrap">${table(d.log, [["ts", "الوقت"], ["admin", "بواسطة"], ["action", "العملية"]])}</div>`);
  const reload = () => { closeModal(); after && after(); };
  $("#bEdit", m).onclick = () => editSubscriber(s, after);
  $("#bTog", m).onclick = () => act(() => api(`subscribers/${s.id}/toggle`, { method: "POST" }), "تم").then(reload);
  $("#bExt", m).onclick = () => {
    const days = prompt("عدد الأيام (من 1 لـ 60):", "1"); if (!days) return;
    act(() => api(`subscribers/${s.id}/extend`, { method: "POST", body: { Days: +days, Reason: "تمديد يدوي" } }), "اتمدد").then(reload);
  };
  $("#bRenew", m).onclick = () => {
    const mm = modal(`<h3>تجديد ${esc(s.username)}</h3><form class="form" id="rf"><label>الباقة<select name="p">${options(PROFILES, s.profile_id)}</select></label>
      <label>خصم<input name="d" type="number" value="0" min="0"></label><label class="full"><span><input type="checkbox" name="paid" checked style="width:auto"> مدفوع (يتخصم من رصيد المدير)</span></label>
      <label class="full">ملاحظات<input name="n"></label><div class="actions full"><button class="green">تجديد</button></div></form>`);
    $("#rf", mm).onsubmit = async e => {
      e.preventDefault(); const f = formData(e.target);
      const r = await act(() => api(`subscribers/${s.id}/renew`, { method: "POST", body: { ProfileID: +f.p, Discount: +f.d, Paid: !!f.paid, Notes: f.n } }));
      toast("اتجدد لحد " + r.expiration); reload();
    };
  };
}
function table(rows, cols) {
  if (!rows || !rows.length) return '<div class="empty">مفيش بيانات</div>';
  return `<table><tr>${cols.map(c => `<th>${c[1]}</th>`).join("")}</tr>${rows.map(r => `<tr>${cols.map(c => `<td>${c[2] ? c[2](r[c[0]], r) : esc(r[c[0]] ?? "")}</td>`).join("")}</tr>`).join("")}</table>`;
}

PAGES.profiles = async () => {
  PROFILES = await api("profiles");
  C(`<div class="panel"><h2><i class="fa fa-tags"></i> العروض (الباقات) <span class="spacer"></span><button class="green" id="addP"><i class="fa fa-plus"></i> باقة جديدة</button></h2>
    <div class="tablewrap">${table(PROFILES, [["name", "الاسم"], ["price", "السعر", v => fmt(v)], ["down_speed", "تحميل"], ["up_speed", "رفع"], ["rate_limit", "Rate-Limit"], ["days", "أيام"],
      ["quota_mb", "كوتة MB"], ["pool", "Pool"], ["users", "المشتركين"], ["id", "", (v) => `<button class="sm" data-p="${v}">تعديل</button>`]])}</div></div>`);
  const edit = p => {
    p = p || {};
    const m = modal(`<h3>${p.id ? "تعديل باقة" : "باقة جديدة"}</h3><form class="form" id="pf">
      <label>الاسم<input name="Name" value="${esc(p.name)}" required></label><label>السعر<input name="Price" type="number" step="0.01" value="${esc(p.price || 0)}"></label>
      <label>سرعة التحميل (مثلا 10M)<input name="DownSpeed" value="${esc(p.down_speed)}" dir="ltr"></label><label>سرعة الرفع (مثلا 2M)<input name="UpSpeed" value="${esc(p.up_speed)}" dir="ltr"></label>
      <label class="full">Rate-Limit كامل (اختياري، بيغطي على السرعات)<input name="RateLimit" value="${esc(p.rate_limit)}" dir="ltr"></label>
      <label>عدد الأيام<input name="Days" type="number" value="${esc(p.days || 30)}"></label><label>كوتة MB (0 = مفتوح)<input name="QuotaMB" type="number" value="${esc(p.quota_mb || 0)}"></label>
      <label>Pool<input name="Pool" value="${esc(p.pool)}" dir="ltr"></label><label>أجهزة في نفس الوقت<input name="Simul" type="number" value="${esc(p.simul || 1)}"></label>
      <div class="actions full"><button class="green">حفظ</button></div></form>`);
    $("#pf", m).onsubmit = async e => {
      e.preventDefault(); const f = formData(e.target);
      ["Price", "Days", "QuotaMB", "Simul"].forEach(k => f[k] = +f[k]);
      await act(() => api(p.id ? "profiles/" + p.id : "profiles", { method: p.id ? "PUT" : "POST", body: f }), "اتحفظت");
      closeModal(); PAGES.profiles();
    };
  };
  $("#addP").onclick = () => edit();
  document.querySelectorAll("[data-p]").forEach(b => b.onclick = () => edit(PROFILES.find(x => +x.id === +b.dataset.p)));
};

PAGES.managers = async () => {
  MANAGERS = await api("managers");
  C(`<div class="panel"><h2><i class="fa fa-university"></i> المدراء <span class="spacer"></span><button class="green" id="addM"><i class="fa fa-plus"></i> مدير جديد</button></h2>
    <div class="tablewrap">${table(MANAGERS, [["username", "اليوزر"], ["full_name", "الاسم"], ["phone", "الموبايل"], ["balance", "الرصيد", (v, r) => +r.unlimited ? "غير محدود" : fmt(v)],
      ["users", "المشتركين"], ["enabled", "الحالة", v => +v ? '<span class="tag green">فعال</span>' : '<span class="tag gray">موقوف</span>'],
      ["id", "", v => `<button class="sm green" data-dep="${v}">شحن</button> <button class="sm" data-led="${v}">الحساب</button> <button class="sm gray" data-ed="${v}">تعديل</button>`]])}</div></div>`);
  const edit = x => {
    x = x || { enabled: 1 };
    const m = modal(`<h3>${x.id ? "تعديل مدير" : "مدير جديد"}</h3><form class="form" id="mf"><label>اليوزر<input name="Username" value="${esc(x.username)}" required></label>
      <label>الاسم<input name="FullName" value="${esc(x.full_name)}"></label><label>الموبايل<input name="Phone" value="${esc(x.phone)}"></label>
      <label>المدير الأعلى<select name="ParentID">${options(MANAGERS.filter(z => z.id !== x.id), x.parent_id, "username")}</select></label>
      <label class="full"><span><input type="checkbox" name="Unlimited" ${+x.unlimited ? "checked" : ""} style="width:auto"> رصيد غير محدود</span></label>
      <label class="full"><span><input type="checkbox" name="Enabled" ${+x.enabled ? "checked" : ""} style="width:auto"> فعال</span></label>
      <div class="actions full"><button class="green">حفظ</button></div></form>`);
    $("#mf", m).onsubmit = async e => {
      e.preventDefault(); const f = formData(e.target);
      await act(() => api(x.id ? "managers/" + x.id : "managers", { method: x.id ? "PUT" : "POST", body: { ...f, ParentID: +f.ParentID, Unlimited: !!f.Unlimited, Enabled: !!f.Enabled } }), "اتحفظ");
      closeModal(); PAGES.managers();
    };
  };
  $("#addM").onclick = () => edit();
  document.querySelectorAll("[data-ed]").forEach(b => b.onclick = () => edit(MANAGERS.find(x => +x.id === +b.dataset.ed)));
  document.querySelectorAll("[data-dep]").forEach(b => b.onclick = async () => {
    const amt = prompt("المبلغ (بالسالب للسحب):"); if (!amt) return;
    const reason = prompt("السبب:", "شحن رصيد") || "";
    await act(() => api(`managers/${b.dataset.dep}/balance`, { method: "POST", body: { Amount: +amt, Reason: reason } }), "تم"); PAGES.managers();
  });
  document.querySelectorAll("[data-led]").forEach(b => b.onclick = async () => {
    const rows = await api(`managers/${b.dataset.led}/balance`);
    modal(`<h3>كشف حساب</h3><div class="tablewrap">${table(rows, [["created", "الوقت"], ["delta", "المبلغ"], ["balance_after", "الرصيد بعدها"], ["reason", "السبب"], ["created_by", "بواسطة"]])}</div>`);
  });
};

PAGES.invoices = async () => {
  C(`<div class="panel"><h2><i class="fa fa-money"></i> الفواتير والشحن</h2><div class="toolbar"><select id="ik"><option value="">الكل</option><option value="subscriber">المشتركين</option>
    <option value="manager">المدراء</option><option value="card">الكروت</option></select><select id="ip"><option value="">مدفوع وغير مدفوع</option><option value="1">مدفوع</option><option value="0">غير مدفوع</option></select>
    <input type="date" id="if"><input type="date" id="it"></div><div class="tablewrap" id="itbl"></div></div>`);
  const load = async () => {
    const rows = await api("invoices?" + new URLSearchParams({ kind: $("#ik").value, paid: $("#ip").value, from: $("#if").value, to: $("#it").value }));
    $("#itbl").innerHTML = table(rows, [["created", "التاريخ"], ["kind", "النوع"], ["username", "المشترك"], ["manager", "المدير"], ["profile", "الباقة"], ["operation", "العملية"],
      ["amount", "المبلغ", v => fmt(v)], ["discount", "خصم"], ["paid", "مدفوع", v => +v ? '<span class="tag green">مدفوع</span>' : '<span class="tag red">غير مدفوع</span>'], ["created_by", "بواسطة"]]);
  };
  ["ik", "ip", "if", "it"].forEach(i => $("#" + i).onchange = load); load();
};

PAGES.cards = async () => {
  C(`<div class="panel"><h2><i class="fa fa-credit-card"></i> كروت الشحن <span class="spacer"></span><button class="green" id="gen"><i class="fa fa-plus"></i> توليد كروت</button></h2>
    <div class="pbody" id="cst"></div><div class="toolbar"><select id="ck"><option value="">كل الأنواع</option><option value="recharge">كروت شحن</option><option value="user">كروت مستخدمين</option></select>
    <select id="cs"><option value="">كل الحالات</option><option value="new">جديد</option><option value="used">مستخدم</option></select><input id="cq" placeholder="بحث بالكود / السلسلة"></div><div class="tablewrap" id="ctbl"></div></div>`);
  const load = async () => {
    const r = await api("cards?" + new URLSearchParams({ kind: $("#ck").value, status: $("#cs").value, q: $("#cq").value }));
    $("#cst").innerHTML = r.stats.map(s => `<span class="tag ${s.status === "used" ? "gray" : "green"}">${s.kind === "user" ? "مستخدمين" : "شحن"} / ${s.status === "used" ? "مستخدم" : "جديد"}: ${fmt(s.n)}</span>`).join(" ") || "";
    $("#ctbl").innerHTML = table(r.rows, [["code", "الكود"], ["series", "السلسلة"], ["kind", "النوع"], ["profile", "الباقة"], ["price", "السعر"], ["status", "الحالة"], ["manager", "المدير"], ["expires", "صالح لحد"], ["created", "اتعمل"]]);
  };
  ["ck", "cs"].forEach(i => $("#" + i).onchange = load); let t; $("#cq").oninput = () => { clearTimeout(t); t = setTimeout(load, 350); };
  $("#gen").onclick = () => {
    const m = modal(`<h3>توليد كروت</h3><form class="form" id="gf"><label>النوع<select name="Kind"><option value="recharge">كروت شحن</option><option value="user">كروت مستخدمين</option></select></label>
      <label>العدد<input name="Count" type="number" min="1" max="1000" value="10"></label><label>الباقة<select name="ProfileID">${options(PROFILES, 0)}</select></label>
      <label>السعر<input name="Price" type="number" step="0.01" value="0"></label><label>بادئة<input name="Prefix" dir="ltr"></label><label>طول الكود<input name="Length" type="number" value="10"></label>
      <label>المدير<select name="ManagerID">${options(MANAGERS, 0, "username")}</select></label><label>اسم السلسلة<input name="Series"></label>
      <div class="actions full"><button class="green">توليد</button></div></form>`);
    $("#gf", m).onsubmit = async e => {
      e.preventDefault(); const f = formData(e.target); ["Count", "ProfileID", "Length", "ManagerID", "Price"].forEach(k => f[k] = +f[k]);
      const r = await act(() => api("cards/generate", { method: "POST", body: f }));
      modal(`<h3>اتعمل ${r.codes.length} كارت — السلسلة ${esc(r.series)}</h3><pre>${esc(r.codes.join("\n"))}</pre>`); load();
    };
  };
  load();
};

PAGES.nas = async () => {
  const rows = await api("nas");
  C(`<div class="panel"><h2><i class="fa fa-paper-plane"></i> أجهزة NAS (السيرفرات اللي بتكلم الراديوس) <span class="spacer"></span><button class="green" id="addN"><i class="fa fa-plus"></i> إضافة</button></h2>
    <div class="pbody muted">في وضع المراقبة مفيش سيرفر بيكلم النظام ده. إضافة NAS هنا مش بتغيّر أي حاجة على الميكروتيك.</div>
    <div class="tablewrap">${table(rows, [["nasname", "IP"], ["shortname", "الاسم"], ["type", "النوع"], ["description", "وصف"], ["id", "", v => `<button class="sm red" data-del="${v}">حذف</button>`]])}</div></div>`);
  $("#addN").onclick = () => {
    const m = modal(`<h3>إضافة NAS</h3><form class="form" id="nf"><label>IP<input name="Nasname" dir="ltr" required></label><label>الاسم<input name="Shortname"></label>
      <label>Secret<input name="Secret" type="password" required></label><label>وصف<input name="Description"></label><div class="actions full"><button class="green">حفظ</button></div></form>`);
    $("#nf", m).onsubmit = async e => { e.preventDefault(); await act(() => api("nas", { method: "POST", body: formData(e.target) }), "اتضاف"); closeModal(); PAGES.nas(); };
  };
  document.querySelectorAll("[data-del]").forEach(b => b.onclick = async () => { if (confirm("حذف الجهاز ده؟")) { await act(() => api("nas/" + b.dataset.del, { method: "DELETE" }), "اتحذف"); PAGES.nas(); } });
};

PAGES.online = async () => {
  const r = await api("online");
  C(`<div class="panel"><h2><i class="fa fa-tachometer"></i> المتصلين الآن</h2>
    <div class="pbody"><b>على الراديوس الجديد:</b> ${fmt(r.own.length)} — <b>على ADV (آخر مزامنة):</b> ${fmt(r.adv.length)}</div>
    <div class="tablewrap">${r.own.length ? table(r.own, [["username", "اليوزر"], ["nasipaddress", "NAS"], ["framedipaddress", "IP"], ["callingstationid", "MAC"], ["acctstarttime", "من"]])
      : table(r.adv, [["username", "اليوزر"], ["nas", "NAS"], ["ip", "IP"], ["mac", "MAC"], ["since", "من"]])}</div></div>`);
};

PAGES.tickets = async () => {
  const rows = await api("tickets");
  C(`<div class="panel"><h2><i class="fa fa-exclamation-circle"></i> الشكاوى <span class="spacer"></span><button class="green" id="addT"><i class="fa fa-plus"></i> شكوى جديدة</button></h2>
    <div class="tablewrap">${table(rows, [["id", "#"], ["subject", "الموضوع"], ["username", "المشترك"], ["status", "الحالة"], ["created", "التاريخ"], ["created_by", "بواسطة"],
      ["id", "", v => `<button class="sm" data-t="${v}">فتح</button>`]])}</div></div>`);
  $("#addT").onclick = () => {
    const m = modal(`<h3>شكوى جديدة</h3><form class="form" id="tf"><label>يوزر المشترك<input name="Username"></label><label>الموضوع<input name="Subject" required></label>
      <label class="full">التفاصيل<textarea name="Body" rows="4"></textarea></label><div class="actions full"><button class="green">حفظ</button></div></form>`);
    $("#tf", m).onsubmit = async e => { e.preventDefault(); await act(() => api("tickets", { method: "POST", body: formData(e.target) }), "اتسجلت"); closeModal(); PAGES.tickets(); };
  };
  document.querySelectorAll("[data-t]").forEach(b => b.onclick = async () => {
    const d = await api("tickets/" + b.dataset.t), t = d.ticket[0];
    const m = modal(`<h3>${esc(t.subject)} <span class="tag gray">${esc(t.status)}</span></h3>${d.replies.map(x => `<div class="panel"><div class="pbody"><b>${esc(x.created_by)}</b> <span class="muted">${esc(x.created)}</span><br>${esc(x.body)}</div></div>`).join("")}
      <form class="form" id="rf"><label class="full">رد<textarea name="Body" rows="3"></textarea></label><label>الحالة<select name="Status"><option value="">بدون تغيير</option><option value="open">مفتوحة</option><option value="pending">معلقة</option><option value="closed">مقفولة</option></select></label>
      <div class="actions full"><button class="green">حفظ</button></div></form>`);
    $("#rf", m).onsubmit = async e => { e.preventDefault(); await act(() => api(`tickets/${t.id}/reply`, { method: "POST", body: formData(e.target) }), "اتحفظ"); closeModal(); PAGES.tickets(); };
  });
};

PAGES.reports = async () => {
  C(`<div class="panel"><h2><i class="fa fa-pie-chart"></i> تقارير المبيعات</h2><div class="toolbar"><input type="date" id="rf"><input type="date" id="rt">
    <select id="rg"><option value="day">باليوم</option><option value="month">بالشهر</option><option value="manager">بالمدير</option><option value="profile">بالباقة</option><option value="kind">بالنوع</option></select></div>
    <div class="tablewrap" id="rtbl"></div></div>`);
  const load = async () => {
    const r = await api("reports/sales?" + new URLSearchParams({ from: $("#rf").value, to: $("#rt").value, group: $("#rg").value }));
    if (!$("#rf").value) { $("#rf").value = r.from; $("#rt").value = r.to; }
    const tot = r.rows.reduce((a, x) => (a.n += +x.n, a.p += +x.paid, a.u += +x.unpaid, a), { n: 0, p: 0, u: 0 });
    $("#rtbl").innerHTML = table(r.rows.concat([{ k: "الإجمالي", n: tot.n, paid: tot.p, unpaid: tot.u }]), [["k", "البند"], ["n", "عدد الفواتير"], ["paid", "مدفوع", v => fmt(v)], ["unpaid", "غير مدفوع", v => fmt(v)]]);
  };
  ["rf", "rt", "rg"].forEach(i => $("#" + i).onchange = load); load();
};

PAGES.sync = async () => {
  const d = await api("sync");
  const L = d.local;
  C(`<div class="panel"><h2><i class="fa fa-refresh"></i> المزامنة مع ADV (قراءة بس) <span class="spacer"></span>${ME.admin.role === "admin" ? '<button class="green" id="runS"><i class="fa fa-refresh"></i> زامن دلوقتي</button>' : ""}</h2>
    <div class="pbody">${d.cred_error ? `<div class="alert" style="background:var(--red)">${esc(d.cred_error)}</div>` : ""}
    <div class="kv"><div>الحالة</div><div>${esc(d.state)}</div><div>آخر مزامنة</div><div>${esc(d.last || "لسه")}</div>
      <div>المشتركين عندنا</div><div>${fmt(L.subscribers)} (منهم من ADV: ${fmt(L.from_adv)})</div><div>الباقات</div><div>${fmt(L.profiles)}</div><div>المدراء</div><div>${fmt(L.managers)}</div>
      <div>الكروت</div><div>${fmt(L.cards)}</div><div>المتصلين على ADV</div><div>${fmt(L.adv_online)}</div><div>مشتركين من غير باقة</div><div>${fmt(L.no_profile)}</div>
      <div>مشتركين من غير تاريخ انتهاء</div><div>${fmt(L.no_expiration)}</div><div>جاهزين على الراديوس الجديد</div><div>${fmt(L.radius_users)}</div></div>
    <h3>أرقام ADV نفسه (للمقارنة)</h3><pre>${esc(JSON.stringify(d.adv_metrics, null, 2))}</pre>
    <h3>سجل المزامنة</h3><div class="tablewrap">${table(d.logs, [["ts", "الوقت"], ["kind", "الجزء"], ["ok", "", v => +v ? "✔" : "✘"], ["fetched", "اتقرا"], ["changed", "اتغير"], ["message", "رسالة"]])}</div></div></div>`);
  const b = $("#runS"); if (b) b.onclick = async () => { b.disabled = true; b.textContent = "شغالة..."; try { await act(() => api("sync/run", { method: "POST" }), "خلصت"); } finally { PAGES.sync(); } };
  if (ME.admin.role === "admin") { verifyBox(); quotaCmpBox(); archiveBox(); }
};

// مطابقة مع ADV: random subscribers read live from ADV and compared field by field (runs every 6 hours by itself)

// مقارنة عدّ الجيجات: ADV's quota deduction vs our own count (download / upload / total) over the same window
async function quotaCmpBox() {
  const st = await api("sync/quota-compare").catch(() => null);
  if (!st) return;
  const L = st.last || {};
  const mb = v => fmt(Math.round(v || 0)) + " ميجا";
  const pc = v => (v > 0 ? "+" : "") + (v || 0) + "%";
  const box = document.createElement("div");
  box.className = "panel";
  box.innerHTML = `<h2><i class="fa fa-balance-scale"></i> مقارنة عدّ الجيجات مع ADV <span class="spacer"></span>
    <button class="green" id="runQc" ${st.running ? "disabled" : ""}><i class="fa fa-play"></i> ${st.running ? esc(st.state || "شغالة...") : "قيس 60 مشترك لمدة ساعة"}</button></h2>
    <div class="pbody"><div class="kv"><div>آخر قياس</div><div>${esc(L.ts || "لسه")}${L.minutes ? ` (${L.minutes} دقيقة)` : ""}</div><div>مشتركين اتقاسوا</div><div>${fmt(L.users || 0)}${L.skipped ? ` (اتشال ${fmt(L.skipped)} اتجدّدوا في النص)` : ""}</div>
      <div>ADV خصم</div><div>${mb(L.adv_mb)}</div>
      <div>إحنا عدّينا تحميل</div><div>${mb(L.down_mb)} (الفرق ${pc(L.vs_down_pc)})</div>
      <div>إحنا عدّينا رفع</div><div>${mb(L.up_mb)} (الفرق ${pc(L.vs_up_pc)})</div>
      <div>إحنا عدّينا تحميل + رفع</div><div>${mb((L.down_mb || 0) + (L.up_mb || 0))} (الفرق ${pc(L.vs_total_pc)})</div></div>
    ${(L.rows || []).length ? `<div class="tablewrap">${table(L.rows, [["user", "المشترك"], ["adv_mb", "ADV خصم", mb], ["down_mb", "تحميل عندنا", mb], ["up_mb", "رفع عندنا", mb], ["diff_pc", "الفرق", pc]])}</div>` : ""}</div>`;
  $("#content").appendChild(box);
  $("#runQc").onclick = async () => { await act(() => api("sync/quota-compare", { method: "POST", body: { N: 60, Minutes: 60 } }), "بدأت — النتيجة بعد ساعة وشوية"); PAGES.sync(); };
}
async function verifyBox() {
  const st = await api("sync/verify").catch(() => null);
  if (!st) return;
  const L = st.last || {};
  const box = document.createElement("div");
  box.className = "panel";
  box.innerHTML = `<h2><i class="fa fa-check-square-o"></i> مطابقة مع ADV <span class="spacer"></span>
    <button class="green" id="runVer" ${st.running ? "disabled" : ""}><i class="fa fa-play"></i> ${st.running ? "شغالة..." : "طابق 100 مشترك دلوقتي"}</button></h2>
    <div class="pbody"><div class="kv"><div>آخر مطابقة</div><div>${esc(L.ts || "لسه")}</div><div>اتفحص</div><div>${fmt(L.checked || 0)} مشترك</div>
      <div>متطابق 100%</div><div>${fmt(L.identical || 0)}</div><div>فيه اختلاف</div><div>${fmt(L.different || 0)}${L.fixed ? ` (اتصلّح منهم ${fmt(L.fixed)})` : ""}</div></div>
    ${(L.diffs || []).length ? `<div class="tablewrap">${table(L.diffs, [["user", "المشترك"], ["field", "الحقل"], ["adv", "في ADV"], ["ours", "عندنا"]])}</div>` : ""}</div>`;
  $("#content").appendChild(box);
  $("#runVer").onclick = async () => { await act(() => api("sync/verify", { method: "POST", body: { N: 100 } }), "بدأت — النتيجة خلال 3 دقايق تقريبا"); PAGES.sync(); };
}

// أرشيف ADV: keep ADV's activity log + every subscriber's old sessions before migrating (about one ADV call per subscriber)
async function archiveBox() {
  const st = await api("sync/archive").catch(() => null);
  if (!st) return;
  const box = document.createElement("div");
  box.className = "panel";
  box.innerHTML = `<h2><i class="fa fa-archive"></i> أرشيف ADV (سجل العمليات + الجلسات القديمة) <span class="spacer"></span>
    <button class="orange" id="runArc" ${st.running ? "disabled" : ""}><i class="fa fa-download"></i> ${st.running ? "شغالة..." : "ابدأ الأرشفة"}</button></h2>
    <div class="pbody"><div class="kv"><div>الحالة</div><div>${esc(st.state || "لسه ما اتعملتش")}</div><div>عمليات محفوظة</div><div>${fmt(st.activity)}</div><div>جلسات محفوظة</div><div>${fmt(st.sessions)}</div></div>
    <p class="muted">بتقرا بس من ADV، طلب واحد لكل مشترك (حوالي ${fmt(5200)} طلب على نص ساعة تقريبا). سجل العمليات الجديد بيتحفظ لوحده كل 20 ثانية.</p></div>`;
  $("#content").appendChild(box);
  $("#runArc").onclick = async () => {
    const c = prompt("الأرشفة هتبعت آلاف الطلبات لـ ADV (قراءة بس). اكتب: نعم ابدأ");
    if (c === null) return;
    await act(() => api("sync/archive", { method: "POST", body: { Confirm: c.trim() } }), "بدأت — تابع الحالة هنا");
    PAGES.sync();
  };
}

PAGES.audit = async () => {
  const rows = await api("audit");
  C(`<div class="panel"><h2><i class="fa fa-history"></i> سجل العمليات (ما يتمسحش ولا يتعدل)</h2><div class="tablewrap">${table(rows, [["ts", "الوقت"], ["admin", "بواسطة"], ["action", "العملية"], ["target", "على"], ["details", "تفاصيل", v => `<span class="muted" dir="ltr">${esc(String(v || "").slice(0, 120))}</span>`], ["ip", "IP"]])}</div></div>`);
};

PAGES.admins = async () => {
  const rows = await api("admins");
  C(`<div class="panel"><h2><i class="fa fa-user-secret"></i> مستخدمين اللوحة <span class="spacer"></span><button class="green" id="addA"><i class="fa fa-plus"></i> مستخدم جديد</button></h2>
    <div class="tablewrap">${table(rows, [["username", "اليوزر"], ["full_name", "الاسم"], ["role", "الصلاحية", v => ({ admin: "مدير النظام", manager: "موزع", viewer: "مشاهدة بس" }[v] || v)],
      ["enabled", "الحالة", v => +v ? "فعال" : "موقوف"], ["id", "", v => `<button class="sm" data-a="${v}">تعديل</button>`]])}</div>
    <div class="pbody"><button class="gray" id="myPw">تغيير كلمة المرور بتاعتي</button></div></div>`);
  const edit = x => {
    x = x || { enabled: 1, role: "viewer" };
    const m = modal(`<h3>${x.id ? "تعديل" : "مستخدم جديد"}</h3><form class="form" id="af"><label>اليوزر<input name="Username" value="${esc(x.username)}" ${x.id ? "readonly" : "required"}></label>
      <label>الاسم<input name="FullName" value="${esc(x.full_name)}"></label><label>الصلاحية<select name="Role">${[["admin", "مدير النظام"], ["manager", "موزع"], ["viewer", "مشاهدة بس"]].map(([k, t]) => `<option value="${k}" ${x.role === k ? "selected" : ""}>${t}</option>`).join("")}</select></label>
      <label>مربوط بالمدير (للموزع)<select name="ManagerID">${options(MANAGERS, x.manager_id, "username")}</select></label>
      <label>كلمة مرور ${x.id ? "(سيبها فاضية لو مش هتغيرها)" : ""}<input name="Password" type="password" autocomplete="new-password"></label>
      <label><span><input type="checkbox" name="Enabled" ${+x.enabled ? "checked" : ""} style="width:auto"> فعال</span></label><div class="actions full"><button class="green">حفظ</button></div></form>`);
    $("#af", m).onsubmit = async e => {
      e.preventDefault(); const f = formData(e.target);
      await act(() => api(x.id ? "admins/" + x.id : "admins", { method: x.id ? "PUT" : "POST", body: { ...f, ManagerID: +f.ManagerID, Enabled: !!f.Enabled } }), "اتحفظ");
      closeModal(); PAGES.admins();
    };
  };
  $("#addA").onclick = () => edit();
  document.querySelectorAll("[data-a]").forEach(b => b.onclick = () => edit(rows.find(x => +x.id === +b.dataset.a)));
  $("#myPw").onclick = () => {
    const m = modal(`<h3>تغيير كلمة المرور</h3><form class="form" id="pw"><label>الحالية<input name="Current" type="password" required></label><label>الجديدة (8 حروف على الأقل)<input name="New" type="password" required minlength="8"></label>
      <div class="actions full"><button class="green">حفظ</button></div></form>`);
    $("#pw", m).onsubmit = async e => { e.preventDefault(); await act(() => api("me/password", { method: "POST", body: formData(e.target) }), "اتغيرت"); closeModal(); };
  };
};

PAGES.settings = async () => {
  const d = await api("settings"), S = Object.fromEntries(d.settings.map(x => [x.k, x.v]));
  const f = (k, label, help = "") => `<label>${label}<input name="${k}" value="${esc(S[k])}" dir="auto">${help ? `<small class="muted">${help}</small>` : ""}</label>`;
  C(`<div class="panel"><h2><i class="fa fa-cog"></i> الإعدادات</h2><div class="pbody"><form class="form" id="sf">
    ${f("company", "اسم الشركة")}${f("currency", "العملة")}
    ${f("expired_pool", "Pool المنتهي اشتراكهم", "زي END على السيرفرات")}${f("quota_pool", "Pool اللي خلصت باقتهم", "زي LIMIT")}
    ${f("expired_rate", "سرعة المنتهي (اختياري)", "مثلا 256k/256k")}${f("expiring_days", "ينتهي قريب = خلال كام يوم")}
    ${f("sync_minutes", "المزامنة مع ADV كل كام دقيقة")}${f("default_days", "أيام الباقة الافتراضية")}
    <label>ربط اليوزر بالـ MAC<select name="mac_bind"><option value="0" ${S.mac_bind !== "1" ? "selected" : ""}>مقفول</option><option value="1" ${S.mac_bind === "1" ? "selected" : ""}>مفعّل</option></select></label>
    <label>كلمة سر PPPoE الموحدة<input value="${d.default_password_set ? "متسجلة في ملف الإعدادات على السيرفر" : "مش متسجلة!"}" readonly></label>
    <div class="actions full"><button class="green">حفظ</button></div></form></div></div>
    <div class="panel"><h2><i class="fa fa-exchange"></i> وضع التشغيل</h2><div class="pbody">
      <p>الوضع الحالي: <b>${S.mode === "shadow" ? "مراقبة (Shadow) — ADV هو الشغال، والنظام ده بيقرا بس" : "تشغيل (Live)"}</b></p>
      <p class="muted">التحويل لـ Live بيفتح التجديد والفلوس هنا، ومش بيغيّر أي حاجة على السيرفرات. ربط السيرفرات بالراديوس الجديد خطوة منفصلة بموافقة المالك.</p>
      <button class="${S.mode === "shadow" ? "red" : "orange"}" id="modeBtn">${S.mode === "shadow" ? "حوّل لـ Live" : "رجّع لـ Shadow"}</button></div></div>`);
  $("#sf").onsubmit = async e => { e.preventDefault(); await act(() => api("settings", { method: "PUT", body: formData(e.target) }), "اتحفظت"); };
  $("#modeBtn").onclick = async () => {
    const to = S.mode === "shadow" ? "live" : "shadow";
    const c = prompt(`علشان تحوّل لـ ${to} اكتب: نعم انقل`); if (c !== "نعم انقل") return;
    await act(() => api("settings", { method: "PUT", body: { mode: to, confirm: c } }), "اتغير"); location.reload();
  };
};

boot();
