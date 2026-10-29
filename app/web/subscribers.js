// "قائمة المشتركين" — a copy of ADV's list: same columns, status colors, filters and menus (rules in adv-docs/ADV-SUBSCRIBERS-LIST.md).
"use strict";
const SUB_COLS = [
  ["chk", ""], ["n", "#"], ["username", "اسم المستخدم"], ["firstname", "الاسم الأول"], ["money", "رصيد نقدي (EGP)"], ["quota", "الكوتة المتبقية"],
  ["expiration", "وقت الانتهاء"], ["ip", "عنوان IP"], ["profile", "الباقة"], ["lastname", "الاسم الأخير"], ["mac", "عنوان MAC"], ["manager", "المدير"],
  ["mobile", "الهاتف"], ["created", "تم إنشائه في"], ["notes", "ملاحظات"], ["paid_total", "إجمالي الفواتير المدفوعة (EGP)"],
  ["unpaid_total", "إجمالي الفواتير الغير مدفوعة (EGP)"], ["online", ""], ["address", "العنوان"], ["used", "استهلاك الكوتة"], ["framed_ip", "IP ثابت"],
];
const SUB_HIDDEN_DEFAULT = ["lastname", "mac", "manager", "created", "paid_total", "unpaid_total", "used", "framed_ip"];
const SUB_SORT = { username: "name", money: "money", quota: "quota", expiration: "exp", created: "created" };
// ADV's status filter, same order and wording; "none" = ADV feature we don't have data for yet
const SUB_STATUS = [["", "كل المشتركين"], ["active", "المشتركين المتاحين"], ["expired", "المشتركين المنتهي اشتراكهم"], ["quota", "منتهي الكوتة"],
  ["none", "منتهي الكوتة اليومية"], ["expiring", "ينتهي وقتهم خلال 3 أيام"], ["today", "ينتهي وقتهم اليوم"], ["unavailable", "مشتركين غير متاحين"],
  ["offline", "المشتركين الغير متصلين"], ["online", "المشتركين المتصلين"], ["borrowed", "المشتركين المستلفين"], ["auto_sub", "تفعيل تلقائي من حساب المشترك"],
  ["auto_mgr", "تفعيل تلقائي من حساب المدير"], ["static", "لديه ايبي ثابت"], ["debtors", "مدينون"], ["special_speed", "لديه سرعة خاصة"], ["temp_speed", "لديه سرعة مؤقتة"],
  ["none", "اسم مختلف في الجلسات"], ["discount", "مشتركون لديهم خصم"], ["disabled", "الموقوفون مؤقتًا"], ["none", "الموجّه متصل"], ["none", "الموجّه غير متصل"],
  ["nosession", "غير متصل بالإنترنت"], ["nosession", "لا توجد جلسة إنترنت"], ["expired", "اشتراك الإنترنت منتهي"]];
// ADV row colors (fnRowCallback)
const ST_COLOR = { _exp_3: ["#80ff80", "ينتهي وقتهم خلال 3 أيام"], _exp_1: ["#ccffcc", "ينتهي وقتهم اليوم"], _exp_: ["#EAD303", "منتهي"],
  _exp_q: ["#f1a447", "منتهي الكوتة"], _dis_: ["#ff3b3be0", "معطل"] };
const ACT_MENU = [["CREDITS", "الرصيد"], ["REF_CARD", "شحن بالكارت"], ["TRIAL", "تجريبي"], ["ADD_TIME", "إضافة وقت"], ["ADD_ADVANCE", "إضافة سلفة"], ["SELF_BORROW_BY_ADMIN", "سلفة من حساب المشترك"],
  ["ADD_NEW_QUOTA", "إضافة كوتة"], ["multi", "تجديد متعدد"], ["RECH_CASH", "شحن بالرصيد النقدي"], ["ADD_CASH", "إضافة مبلغ نقدي"], ["WITHDRAW_CASH", "سحب مبلغ نقدي"], ["pay_with_wallet", "التفعيل برقم المعاملة"]];
const MORE_MENU = [["print", "طباعة اخر فاتورة"], ["print", "طباعة إيصال"], ["rename_user", "إعادة تسمية"], ["copy", "تكرار"], ["disconnect_user", "فصل الإتصال"], ["send_msg", "إرسال رسالة"], ["soon", "إنشاء عقد"],
  ["delete_user", "حذف"], ["soon", "استعادة الكوتة اليومية"], ["soon", "ضغط البيانات"], ["toggle", "تغيير الحالة"], ["mac", "تعديل عنوان الماك"], ["PART_PAY", "دفع جزئي"], ["disable", "إيقاف مؤقت"]];
function subHidden() { try { return JSON.parse(localStorage.getItem("rm_sub_hidden2")) || SUB_HIDDEN_DEFAULT; } catch (_) { return SUB_HIDDEN_DEFAULT; } }
function subSaveHidden(h) { try { localStorage.setItem("rm_sub_hidden2", JSON.stringify(h)); } catch (_) {} }
function bytesTxt(mb) {
  const b = Number(mb || 0) * 1048576;
  if (b <= 0) return "0 B";
  const u = ["B", "KB", "MB", "GB", "TB"]; let i = 0, v = b;
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++; }
  return `${v.toFixed(2)} ${u[i]}`;
}
function advDate(s) {
  if (!s) return "";
  const d = new Date(s.replace(" ", "T")), p = n => String(n).padStart(2, "0");
  return `${p(d.getDate())}-${p(d.getMonth() + 1)}-${d.getFullYear()} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

PAGES.subscribers = async () => {
  const qs = new URLSearchParams(location.hash.split("?")[1] || "");
  const st = { status: qs.get("status") || "", q: "", profile_id: 0, manager_id: 0, sub: 0, page: 1, size: +(localStorage.getItem("rm_sub_size") || 25),
    sort: "", date_by: "created", from: "", to: "" };
  let rows = [], total = 0, all = 0;
  const sel = new Set();
  const ddMenu = (id, items) => `<div class="dd-menu" id="${id}">${items.map(([k, t]) => `<a data-act="${k}" class="${k === "soon" ? "soon" : ""}">${t}</a>`).join("")}</div>`;
  C(`<div class="adv-card">
    <div class="adv-head"><span><i class="fa fa-user-plus"></i> قائمة المشتركين <small id="foundTxt"></small></span><button class="icon-btn" id="reload" title="تحديث"><i class="fa fa-refresh"></i></button></div>
    <div class="adv-body">
      <div class="adv-row">
        <div class="adv-f"><label>البحث بواسطة</label><div class="adv-inline"><button class="adv-btn-ico" id="go1"><i class="fa fa-search"></i></button>
          <select id="dateBy"><option value="created">تاريخ الإنشاء</option><option value="expiration">تاريخ الانتهاء</option></select></div></div>
        <div class="adv-f"><label>من</label><div class="adv-inline"><span class="adv-cal"><i class="fa fa-calendar"></i></span><input type="date" id="dFrom"></div></div>
        <div class="adv-f"><label>الى</label><div class="adv-inline"><span class="adv-cal"><i class="fa fa-calendar"></i></span><input type="date" id="dTo"></div></div>
        <div class="adv-f adv-mgr"><label>المدير</label><div class="adv-inline"><select id="fm">${options(MANAGERS, 0, "username", ME.admin.username + " (" + (ME.admin.role === "admin" ? "Administrator" : ME.admin.role) + ")")}</select></div></div>
      </div>
      <div class="adv-row">
        <div class="adv-f"><select id="fst">${SUB_STATUS.map(([k, t], i) => `<option value="${i}" ${k && k === st.status ? "selected" : ""}>${t}</option>`).join("")}</select></div>
        <div class="adv-f"><select id="fp">${options(PROFILES, 0, "name", "كل العروض")}</select></div>
        <div class="adv-actions">
          <button class="b-blue" id="aAdd"><i class="fa fa-plus"></i> إضافة</button>
          <button class="b-light" id="a360"><i class="fa fa-eye"></i> °360 عرض</button>
          <span class="dd"><button class="b-gray" data-dd="dAct"><i class="fa fa-bolt"></i> تفعيل <i class="fa fa-caret-down"></i></button>${ddMenu("dAct", ACT_MENU)}</span>
          <button class="btn-hoki" id="aProf"><i class="fa fa-exchange"></i> تغيير العرض</button>
          <button class="btn-hoki" data-act="RECH_CASH">تفعيل نقدي</button><button class="btn-hoki" data-act="trans_money">تحويل رصيد</button><button class="btn-hoki" data-act="ADD_NEW_QUOTA_U">إضافة كوتة من رصيد المشترك</button>
          <span class="dd"><button class="b-gray" data-dd="dMore">المزيد <i class="fa fa-caret-down"></i></button>${ddMenu("dMore", MORE_MENU)}</span>
        </div>
      </div>
      <div class="adv-row adv-search">
        <div class="adv-f adv-grow"><label>بحث عام</label><div class="adv-inline"><span class="adv-cal"><i class="fa fa-filter"></i></span><input id="q"></div>
          <button class="b-blue sm" id="go2"><i class="fa fa-search"></i> بحث</button></div>
        <div class="adv-f adv-toggle"><label>موزعين النظام</label><label class="switch"><input type="checkbox" id="fsub"><span>إظهار</span></label></div>
      </div>
      <div class="adv-tools">
        <div><label>إظهار <select id="psize">${[10, 25, 50, 100, 250, 500, 1000, 2000, 5000, 10000].map(n => `<option value="${n}" ${n === st.size ? "selected" : ""}>${n} في الصفحة</option>`).join("")}</select> نتائج</label></div>
        <div class="adv-tbtns"><button id="tCsv">CSV</button><button id="tXls">EXCEL</button><button id="tPrint">طباعة</button>
          <span class="dd"><button data-dd="dCols">عرض/إخفاء</button><div class="dd-menu" id="dCols"></div></span></div>
      </div>
      <div class="adv-tools"><div></div><div><label>بحث: <input id="localQ"></label></div></div>
      <div class="tablewrap"><table class="adv-table" id="advT"></table></div>
      <div class="adv-tools"><div id="infoTxt" class="muted"></div><div class="adv-pager" id="pg"></div></div>
    </div></div>`);

  const hidden = () => subHidden();
  const visCols = () => SUB_COLS.filter(([k]) => !hidden().includes(k));
  const drawCols = () => {
    $("#dCols").innerHTML = SUB_COLS.filter(([k, t]) => k !== "chk" && k !== "n" && t).map(([k, t]) =>
      `<label class="dd-chk"><input type="checkbox" data-col="${k}" ${hidden().includes(k) ? "" : "checked"}> ${t}</label>`).join("");
    $("#dCols").querySelectorAll("input").forEach(i => i.onchange = () => {
      const h = new Set(hidden()); i.checked ? h.delete(i.dataset.col) : h.add(i.dataset.col); subSaveHidden([...h]); draw();
    });
  };
  const nCell = (s, n) => {
    let color = "", title = "";
    if (s.st === "_act_") [color, title] = +s.online ? ["#809fff", "متصل"] : ["#2dd22d", "نشط"];
    else if (ST_COLOR[s.st]) [color, title] = ST_COLOR[s.st];
    let bg = color;
    if (+s.borrowed) {
      const base = color || "#ffffff", b = s.st === "_exp_q" ? "#8e44ad" : "#f7a522";
      bg = `linear-gradient(to left, ${base} 0%, ${base} 52%, ${b} 100%)`; title = (title ? title + " & " : "") + "مستلف";
    }
    return `<td class="c-n" style="background:${bg}" title="${esc(title)}">${n}</td>`;
  };
  const cell = (k, s, i) => {
    switch (k) {
      case "chk": return `<td class="c-chk"><input type="checkbox" data-sel="${s.id}" ${sel.has(+s.id) ? "checked" : ""}></td>`;
      case "n": return nCell(s, (st.page - 1) * st.size + i + 1);
      case "username": {
        const hl = +s.online && s.st !== "_act_" ? ' style="background:#809fff;color:#fff" title="متصل"' : "";
        const badges = (+s.discount > 0 ? '<span class="mini green">خصم %</span>' : "");
        return `<td><a class="u-pill" data-open="${s.id}"${hl}>${esc(s.username)}</a>${badges}</td>`;
      }
      case "money": return `<td>${Number(s.money || 0).toFixed(2)}</td>`;
      case "quota": {
        if (+s.profile_quota <= 0) return "<td>غير محدود</td>";
        const t = bytesTxt(Number(s.quota_mb || 0) - Number(s.used_mb || 0));
        return `<td${t === "0 B" ? ' style="background:#efa51e"' : ""}>${t}</td>`;
      }
      case "used": return `<td>${bytesTxt(s.used_mb)}</td>`;
      case "expiration": {
        const exp = !s.expiration || new Date(s.expiration.replace(" ", "T")) < new Date();
        return `<td${exp && s.expiration ? ' style="background:#efa51e"' : ""}>${s.expiration ? advDate(s.expiration) : "---"}</td>`;
      }
      case "created": return `<td>${advDate(s.created)}</td>`;
      case "ip": return `<td>${s.ip ? `<b>${esc(s.ip)}</b><div class="nas">NAS: ${esc(s.nas_name || s.nas || "")}</div>` : "---"}</td>`;
      case "online": return `<td>${+s.online ? 1 : 0}</td>`;
      case "paid_total": case "unpaid_total": return `<td>${Number(s[k] || 0).toFixed(2)}</td>`;
      case "firstname": return `<td>${esc(s.firstname || "")}</td>`;
      case "notes": case "address": return `<td class="c-txt">${esc(String(s[k] ?? "").replace(/\\n|\n/g, " "))}</td>`;
      default: return `<td>${esc(s[k] ?? "")}</td>`;
    }
  };
  const draw = () => {
    const cols = visCols();
    $("#advT").innerHTML = `<thead><tr>${cols.map(([k, t]) => k === "chk" ? '<th class="c-chk"><input type="checkbox" id="selAll"></th>'
      : `<th ${SUB_SORT[k] ? `class="sortable" data-sort="${SUB_SORT[k]}"` : ""}>${t}${SUB_SORT[k] ? ` <i class="fa fa-sort${st.sort === SUB_SORT[k] ? "-asc" : ""}"></i>` : ""}</th>`).join("")}</tr></thead>
      <tbody>${rows.length ? rows.map((s, i) => `<tr data-id="${s.id}" data-u="${esc(s.username)}">${cols.map(([k]) => cell(k, s, i)).join("")}</tr>`).join("") : `<tr><td colspan="${cols.length}" class="empty">لا توجد بيانات</td></tr>`}</tbody>`;
    $("#foundTxt").textContent = `( تم إيجاد ${fmt(total)} من ${fmt(all)} مشتركين )`;
    const from = total ? (st.page - 1) * st.size + 1 : 0;
    $("#infoTxt").textContent = `إظهار ${fmt(from)} إلى ${fmt(Math.min(total, st.page * st.size))} من أصل ${fmt(total)} مُدخل`;
    $("#selAll").onchange = e => { rows.forEach(s => e.target.checked ? sel.add(+s.id) : sel.delete(+s.id)); draw(); };
    document.querySelectorAll("[data-sel]").forEach(c => c.onchange = () => c.checked ? sel.add(+c.dataset.sel) : sel.delete(+c.dataset.sel));
    document.querySelectorAll("[data-open]").forEach(c => c.onclick = () => { location.hash = "user?id=" + c.dataset.open; });
    document.querySelectorAll("#advT tbody tr[data-id]").forEach(tr => tr.oncontextmenu = e => showContextMenu(e, +tr.dataset.id, tr.dataset.u, () => { sel.clear(); load(); }));
    document.querySelectorAll("th[data-sort]").forEach(th => th.onclick = () => { st.sort = st.sort === th.dataset.sort && th.dataset.sort === "exp" ? "exp_desc" : th.dataset.sort; load(); });
    drawPager();
  };
  const drawPager = () => {
    const pages = Math.max(1, Math.ceil(total / st.size)), p = st.page;
    const nums = new Set([1, pages, p - 1, p, p + 1, p + 2, p + 3].filter(n => n >= 1 && n <= pages));
    let html = `<a class="${p <= 1 ? "dis" : ""}" data-p="${p - 1}">السابقة</a>`, last = 0;
    [...nums].sort((a, b) => a - b).forEach(n => { if (n - last > 1) html += "<span>…</span>"; html += `<a class="${n === p ? "cur" : ""}" data-p="${n}">${n}</a>`; last = n; });
    html += `<a class="${p >= pages ? "dis" : ""}" data-p="${p + 1}">التالية</a>`;
    $("#pg").innerHTML = html;
    $("#pg").querySelectorAll("a[data-p]").forEach(a => a.onclick = () => { const n = +a.dataset.p; if (n >= 1 && n <= pages && n !== p) { st.page = n; load(); } });
  };
  const params = extra => new URLSearchParams({ status: st.status, q: st.q, profile_id: st.profile_id, manager_id: st.manager_id, sub: st.sub, page: st.page, size: st.size,
    sort: st.sort, date_by: st.from || st.to ? st.date_by : "", from: st.from, to: st.to, ...extra });
  let seq = 0;
  const load = async () => {
    const my = ++seq;
    $("#advT").innerHTML = '<tbody><tr><td class="empty"><i class="fa fa-spinner fa-spin"></i> جاري التحميل...</td></tr></tbody>';
    const r = await api("subscribers?" + params());
    if (my !== seq) return; // a newer search already started
    rows = r.rows; total = r.total; all = r.all; draw();
  };
  const apply = () => { st.page = 1; st.q = $("#q").value.trim(); st.from = $("#dFrom").value; st.to = $("#dTo").value; st.date_by = $("#dateBy").value; load(); };

  // filters
  $("#fst").onchange = e => { st.status = SUB_STATUS[+e.target.value][0]; apply(); };
  $("#fp").onchange = e => { st.profile_id = +e.target.value; apply(); };
  $("#fm").onchange = e => { st.manager_id = +e.target.value; apply(); };
  $("#fsub").onchange = e => { st.sub = e.target.checked ? 1 : 0; apply(); };
  ["go1", "go2"].forEach(id => $("#" + id).onclick = apply);
  let tq; const live = v => { clearTimeout(tq); tq = setTimeout(() => { $("#q").value = v; $("#localQ").value = v; apply(); }, 350); };
  $("#q").oninput = e => live(e.target.value);
  $("#localQ").oninput = e => live(e.target.value);
  $("#q").onkeydown = e => { if (e.key === "Enter") { clearTimeout(tq); apply(); } };
  ["dFrom", "dTo"].forEach(id => $("#" + id).onchange = apply);
  $("#psize").onchange = e => { st.size = +e.target.value; try { localStorage.setItem("rm_sub_size", st.size); } catch (_) {} apply(); };
  $("#reload").onclick = load;

  // dropdowns
  document.querySelectorAll("[data-dd]").forEach(b => b.onclick = e => {
    e.stopPropagation(); const m = $("#" + b.dataset.dd); const open = m.classList.contains("open");
    document.querySelectorAll(".dd-menu.open").forEach(x => x.classList.remove("open")); if (!open) m.classList.add("open");
  });
  if (!window.__ddClose) { window.__ddClose = true; document.addEventListener("click", e => { if (!e.target.closest(".dd-menu")) document.querySelectorAll(".dd-menu.open").forEach(x => x.classList.remove("open")); }); }

  // actions
  const need = () => { if (!sel.size) { toast("من فضلك اختر مشترك أو أكثر من الجدول", true); return false; } return true; };
  const bulk = async (action, extra = {}) => {
    const r = await act(() => api("subscribers/bulk", { method: "POST", body: { Action: action, IDs: [...sel], ...extra } }));
    toast(`تم على ${r.done} مشترك`); sel.clear(); load();
  };
  const renewEach = async paid => {
    let ok = 0;
    for (const id of sel) { try { await api(`subscribers/${id}/renew`, { method: "POST", body: { ProfileID: 0, Discount: 0, Paid: paid, Notes: paid ? "" : "سلفة" } }); ok++; } catch (e) { toast(e.message, true); break; } }
    if (ok) toast(`تم تجديد ${ok} مشترك`); sel.clear(); load();
  };
  const doAct = async k => {
    document.querySelectorAll(".dd-menu.open").forEach(x => x.classList.remove("open"));
    if (k === "soon") return toast("الخاصية دي هتتعمل في المرحلة الجاية", true);
    if (!need()) return;
    if (USER_OPS[k]) { // ADV modal for one subscriber
      if (sel.size > 1 && k === "CREDITS") return confirm(`تجديد ${sel.size} مشترك من الرصيد؟`) && renewEach(true);
      if (sel.size > 1) return toast("اختار مشترك واحد للعملية دي", true);
      return USER_OPS[k]([...sel][0], () => { sel.clear(); load(); });
    }
    switch (k) {
      case "renew": case "multi": if (confirm(`تجديد ${sel.size} مشترك من الرصيد؟`)) renewEach(true); break;
      case "borrow": if (confirm(`إضافة سلفة (تجديد غير مدفوع) لـ ${sel.size} مشترك؟`)) renewEach(false); break;
      case "extend": { const d = prompt("عدد الأيام (1 - 60):", "1"); if (d) bulk("extend", { Days: +d }); break; }
      case "toggle": if (confirm(`تغيير حالة ${sel.size} مشترك؟`)) bulk("toggle"); break;
      case "disable": if (confirm(`إيقاف مؤقت لـ ${sel.size} مشترك؟`)) bulk("disable"); break;
      case "mac": { const v = prompt("عنوان الماك الجديد (اتركه فارغاً لإزالة التثبيت):", ""); if (v !== null) bulk(v.trim() ? "mac" : "unmac", { Value: v }); break; }
      case "print": printLastInvoice([...sel][0]); break;
      case "copy": location.hash = "user?copy=" + [...sel][0]; break;
    }
  };
  document.querySelectorAll("[data-act]").forEach(a => a.onclick = () => doAct(a.dataset.act));
  $("#aAdd").onclick = () => { location.hash = "user?new=1"; };
  $("#a360").onclick = () => { if (need()) openSubscriber([...sel][0], load); };
  $("#aProf").onclick = () => {
    if (!need()) return;
    if (sel.size === 1) return USER_OPS.change_profile([...sel][0], () => { sel.clear(); load(); });
    const m = modal(`<h3>تغيير العرض لـ ${sel.size} مشترك</h3><form class="form" id="cp"><label class="full">العرض الجديد<select name="p">${options(PROFILES, 0)}</select></label>
      <div class="actions full"><button class="green">تغيير</button></div></form>`);
    $("#cp", m).onsubmit = e => { e.preventDefault(); closeModal(); bulk("profile", { ProfileID: +formData(e.target).p }); };
  };
  const exportCSV = () => { location.href = "/api/subscribers?" + params({ format: "csv" }); };
  $("#tCsv").onclick = exportCSV; $("#tXls").onclick = exportCSV;
  $("#tPrint").onclick = () => window.print();

  drawCols();
  if (qs.get("add") === "1") location.hash = "user?new=1";
  load();
};

async function printLastInvoice(id) {
  const d = await api("subscribers/" + id), s = d.subscriber, inv = d.invoices[0];
  if (!inv) return toast("مفيش فواتير للمشترك ده", true);
  const w = window.open("", "_blank", "width=420,height=600");
  const tpl = (await api("settings").catch(() => ({ settings: [] }))).settings.find(x => x.k === "adv_set_invoice_tpl")?.v;
  if (tpl) { // the owner's own receipt layout (settings ← فاتورة المشترك); values are escaped
    const v = { company: ME.company || "Stream-Net", user: s.username, name: `${s.firstname || ""} ${s.lastname || ""}`.trim(), prof: s.profile || "", amount: fmt(inv.amount - inv.discount),
      from: inv.period_from || "", to: inv.period_to || "", date: inv.created, by: inv.created_by, mobile: s.mobile || "", notes: inv.notes || "" };
    w.document.write(`<html dir="rtl"><head><meta charset="utf-8"><title>إيصال</title></head><body>${tpl.replace(/\{(\w+)\}/g, (m, k) => k in v ? esc(v[k]) : m)}<script>print()<\/script></body></html>`);
    return;
  }
  w.document.write(`<html dir="rtl"><head><meta charset="utf-8"><title>إيصال</title><style>body{font-family:Tahoma;padding:16px}td{padding:4px 8px}h2{text-align:center}</style></head><body>
    <h2>${esc(ME.company || "Stream-Net")}</h2><table><tr><td>المشترك</td><td>${esc(s.username)}</td></tr><tr><td>الاسم</td><td>${esc(s.firstname)} ${esc(s.lastname)}</td></tr>
    <tr><td>الباقة</td><td>${esc(s.profile || "")}</td></tr><tr><td>المبلغ</td><td>${fmt(inv.amount - inv.discount)}</td></tr><tr><td>من</td><td>${esc(inv.period_from || "")}</td></tr>
    <tr><td>إلى</td><td>${esc(inv.period_to || "")}</td></tr><tr><td>التاريخ</td><td>${esc(inv.created)}</td></tr><tr><td>بواسطة</td><td>${esc(inv.created_by)}</td></tr></table>
    <script>print()<\/script></body></html>`);
}
