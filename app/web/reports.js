// "تقارير" — ADV's report tree. Reports ADV's API exposes are read live from ADV (read-only) so the numbers match it exactly.
"use strict";
const REPORTS = [
  ["المدراء", [["الأحداث", "get_activity_log", ["from", "to"]], ["مديرين متصلين", "#onlinemgr"], ["فواتير المدراء", "report_manager_sales", ["manager", "from", "to"]], ["رصيد تجريبي"],
    ["المحصلة", "report_summary", ["from", "to"]], ["حالات تسجيل الدخول", "local:login_states", ["from", "to"]], ["مبيعات الكروت (حزم)", "get_cards_sales_stats", []], ["مبيعات الكروت (مديرين)"], ["مبيعات المدراء (النظام الجديد)", "local:manager_sales_local", ["from", "to"]], ["مبيعات الكروت (مشتركين)"],
    ["حركة رصيد المديرين", "get_manager_balance_log", ["manager"]]]],
  ["المشتركين", [["الأحداث", "get_activity_log", ["from", "to"]], ["فواتير المشتركين", "report_invoices", ["paid", "from", "to"]], ["عمليات شحن الكروت"], ["رصيد تجريبي"],
    ["المحصلة", "report_summary", ["from", "to"]]]],
  ["آخر", [["الجلسات", "get_user_sessions", ["user"]], ["الاستهلاك اليومي (من 12 بالليل لـ 12 بالليل)", "daily_usage", ["user", "from", "to"]], ["إجمالي استهلاك المشتركين", "report_user_usage", ["user", "from", "to"]], ["إحصائيات المتصلين", "quick_stats", []],
    ["عمليات تحويل الكروت"], ["ارقام الهواتف للكروت"], ["ارباح كروت الاتصال"], ["سجل اوامر الفصل والتحديث", "local:disconnects", ["from", "to"]], ["كروت الشحن المستخدمة"], ["معاملات الكاش", "local:cash", ["from", "to"]],
    ["تفاصيل معاملات الكاش", "local:cash_details", ["from", "to"]], ["حالات تسجيل الدخول", "local:login_states", ["from", "to"]], ["سجل تغييرات الماك", "local:mac_changes", ["from", "to"]],
    ["سجل تغيير العروض", "local:profile_changes", ["from", "to"]], ["تحويلات الرصيد", "local:balance_transfers", ["from", "to"]], ["محاولات الدخول الفاشلة", "local:login_failed", ["from", "to"]],
    ["فشل تحديث السرعة"], ["سجل الرسائل", "#messages"], ["تقرير العروض المؤقتة"], ["استهلاك الـ NAS", "report_nas_usage", ["nas", "from", "to"]]]],
  ["الرسوم البيانية", [["المدراء", "#charts?set=managers"], ["مشتركين", "#charts?set=subscribers"], ["كروت", "#charts?set=subscribers"], ["التفعيلات", "#charts?set=managers"], ["ارباح كروت الشحن"], ["ارباح كروت الاتصال"], ["العروض", "#charts?set=profiles"], ["NAS", "#charts?set=nas"], ["عرض النطاق الترددي", "#charts?set=subscribers"]]],
  ["الاستهلاك والراديوس", [["أعلى المستهلكين", "local:traffic_top", ["from", "to"]], ["الاستهلاك الشهري للشبكة", "local:traffic_monthly", ["from", "to"]], ["الاستهلاك اليومي للشبكة", "local:traffic_daily", ["from", "to"]],
    ["المديونيات (مشترك مشترك)", "local:debtors", []], ["جلسات الراديوس الجديد", "local:radius_sessions", ["from", "to"]], ["سجل الراديوس الخام", "local:radius_raw", ["from", "to"]]]],
  ["مالية", [["تقرير الأرباح والخسائر", "#profitloss"], ["تقادم الفواتير", "local:aging", []], ["سجل المراجعة المالي", "#audit"], ["لوحة المؤشرات", "get_dashboard_metrics", []], ["مبيعات النظام الجديد (محلي)", "local_sales", ["from", "to"]]]],
];
const LABELS = { down_gb: "تحميل (جيجا)", up_gb: "رفع (جيجا)", total_gb: "الإجمالي (جيجا)", users: "مشتركين", month: "الشهر", day: "اليوم", duration: "المدة", acctterminatecause: "سبب الفصل", mobile: "الموبايل", expiration: "الانتهاء", total: "الإجمالي", acctupdatetime: "آخر تحديث", acctinputoctets: "رفع (بايت)", acctoutputoctets: "تحميل (بايت)", id: "#", username: "اسم المستخدم", user_manager: "المدير", full_name: "الاسم", t_date: "الوقت", date_operation: "التاريخ", new_price: "المبلغ", old_price: "السعر القديم",
  paid_unpaid: "مدفوع", notes: "ملاحظات", discount: "خصم", tax_amount: "ضريبة", operation: "العملية", more_details: "التفاصيل", ip: "IP", acctstarttime: "بداية الجلسة",
  acctstoptime: "نهاية الجلسة", manager: "المدير", status: "الحالة", d0_30: "0-30 يوم", d31_60: "31-60 يوم", d61_90: "61-90 يوم", d90_plus: "أكتر من 90 يوم", count: "العدد", paid: "مدفوع", unpaid: "غير مدفوع", profile: "العرض", action: "العملية", framedipaddress: "عنوان IP", callingstationid: "MAC", nasipaddress: "NAS", acctinputoctets: "رفع", acctoutputoctets: "تحميل", count: "العدد", total: "الإجمالي", day: "اليوم", name: "الاسم", download: "تحميل", upload: "رفع" };

PAGES.reports = async () => {
  const today = new Date().toISOString().slice(0, 10), monthAgo = new Date(Date.now() - 30 * 864e5).toISOString().slice(0, 10);
  C(`<div class="rep-wrap"><div class="adv-card rep-tree">${REPORTS.map(([g, items], gi) => `<div class="rep-g"><b>${g}</b>${items.map(([t, ep], i) =>
      `<a data-r="${gi}:${i}" class="${ep ? "" : "soon"}">${t}</a>`).join("")}</div>`).join("")}</div>
    <div class="adv-card rep-main"><div class="adv-head"><span id="rTitle"><i class="fa fa-pie-chart"></i> تقارير</span></div><div class="adv-body" id="rBody">
      <div class="empty">اختار تقرير من القايمة. التقارير اللي لونها رمادي هتتعمل في المرحلة الجاية.</div></div></div></div>`);
  document.querySelectorAll("[data-r]").forEach(a => a.onclick = () => {
    const [gi, i] = a.dataset.r.split(":").map(Number), [title, ep, params = []] = REPORTS[gi][1][i];
    document.querySelectorAll("[data-r]").forEach(x => x.classList.toggle("cur", x === a));
    if (ep && ep.startsWith("#")) { location.hash = ep.slice(1); return; }
    if (!ep) return toast("التقرير ده هيتعمل في المرحلة الجاية", true);
    $("#rTitle").innerHTML = `<i class="fa fa-pie-chart"></i> ${esc(REPORTS[gi][0])} — ${esc(title)}`;
    const ctl = { from: `<label>من <input type="date" name="from" value="${monthAgo}"></label>`, to: `<label>الى <input type="date" name="to" value="${today}"></label>`,
      manager: `<label>المدير <select name="target_id">${MANAGERS.map(m => `<option value="${esc(m.adv_id || m.id)}">${esc(m.username)}</option>`).join("")}</select></label>`,
      paid: `<label>الحالة <select name="paid_status"><option value="">الكل</option><option value="1">مدفوع</option><option value="0">غير مدفوع</option></select></label>`,
      user: `<label>اسم المستخدم <input name="username" placeholder="اكتب اليوزر"></label>`,
      nas: `<label>NAS <input name="nas_id" placeholder="رقم الـ NAS"></label>` };
    $("#rBody").innerHTML = `<form class="adv-tools rep-f" id="rf">${params.map(p => ctl[p]).join("")}<button class="b-light"><i class="fa fa-search"></i> عرض</button>
      <button type="button" class="btn-plain" id="rCsv">CSV</button><button type="button" class="btn-plain" id="rPrint">طباعة</button></form><div id="rOut"></div>`;
    let last = [];
    $("#rf").onsubmit = async e => {
      e.preventDefault();
      const f = formData(e.target);
      if (f.username && ep !== "daily_usage") { // ADV wants its own user id
        const r = await api("subscribers?size=1&q=" + encodeURIComponent(f.username)); const s = r.rows[0];
        if (!s) return toast("المشترك مش موجود", true);
        const d = await api("subscribers/" + s.id); f.user_id = d.subscriber.adv_id; delete f.username;
      }
      $("#rOut").innerHTML = '<div class="empty"><i class="fa fa-spinner fa-spin"></i> جاري التحميل من ADV...</div>';
      try {
        let out;
        if (ep === "local_sales") out = await api("reports/sales?" + new URLSearchParams({ from: f.from, to: f.to, group: "day" }));
        else if (ep === "daily_usage") { out = await api("reports/daily-usage?" + new URLSearchParams({ from: f.from, to: f.to, user: f.username || "" })); out.data.forEach(r => { ["download", "upload", "total"].forEach(k => r[k] = bts(r[k])); }); }
        else if (ep.startsWith("local:")) out = await api("reports/local?" + new URLSearchParams({ name: ep.slice(6), from: f.from || "", to: f.to || "" }));
        else { // ADV reads dates as unix seconds
          const g = { ...f };
          if (g.from) g.from = Math.floor(new Date(g.from + "T00:00:00").getTime() / 1000);
          if (g.to) g.to = Math.floor(new Date(g.to + "T23:59:59").getTime() / 1000);
          out = await api("adv/" + ep + "?" + new URLSearchParams({ limit: 5000, ...g }));
        }
        last = repRows(out);
        $("#rOut").innerHTML = repRender(out, last);
      } catch (err) { $("#rOut").innerHTML = `<div class="empty">${esc(err.message)}</div>`; }
    };
    $("#rCsv").onclick = () => {
      if (!last.length) return;
      const ks = Object.keys(last[0]);
      downloadCSV(title, ks.map(k => LABELS[k] || k), last.map(r => ks.map(k => r[k])));
    };
    $("#rPrint").onclick = () => window.print();
    if (!params.some(p => p === "manager" || p === "user" || p === "nas")) $("#rf").requestSubmit();
  });
};
function repVal(k, v) {
  if (/date|time|_on$/.test(k) && /^\d{10}$/.test(String(v))) return advDate(new Date(+v * 1000 - new Date().getTimezoneOffset() * 60000).toISOString().replace("T", " ").slice(0, 19));
  if (k === "paid_unpaid") return +v ? "مدفوع" : "غير مدفوع";
  return v ?? "";
}
function repRows(out) {
  if (Array.isArray(out)) return out;
  for (const k of ["data", "rows", "items", "result", "logs", "sessions", "invoices"]) if (Array.isArray(out && out[k])) return out[k];
  return [];
}
function repRender(out, rows) {
  if (rows.length) {
    const ks = Object.keys(rows[0]).filter(k => typeof rows[0][k] !== "object");
    return `<div class="muted" style="margin:6px 0">عدد السجلات: ${rows.length}</div><div class="tablewrap"><table class="adv-table"><thead><tr>${ks.map(k => `<th>${esc(LABELS[k] || k)}</th>`).join("")}</tr></thead>
      <tbody>${rows.slice(0, 5000).map(r => `<tr>${ks.map(k => `<td>${esc(repVal(k, r[k]))}</td>`).join("")}</tr>`).join("")}</tbody></table></div>`;
  }
  const flat = {}; const walk = (o, p) => Object.entries(o || {}).forEach(([k, v]) => v && typeof v === "object" && !Array.isArray(v) ? walk(v, p + k + " / ") : (k !== "error" && (flat[p + k] = Array.isArray(v) ? v.length + " عنصر" : v)));
  walk(out, "");
  return Object.keys(flat).length ? `<div class="kv">${Object.entries(flat).map(([k, v]) => `<div>${esc(LABELS[k] || k)}</div><div>${esc(v)}</div>`).join("")}</div>` : '<div class="empty">لا توجد بيانات</div>';
}
