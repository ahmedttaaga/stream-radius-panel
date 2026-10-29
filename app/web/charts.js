// "الرسوم البيانية" (top-N bar charts like ADV's amCharts pages) and "تقرير الأرباح والخسائر".
"use strict";
function barChart(rows, key, val, fmtv = v => fmt(v)) {
  const max = Math.max(1, ...rows.map(r => +r[val] || 0));
  return `<div class="bars">${rows.map(r => `<div class="bar-row"><span class="bar-l" title="${esc(r[key])}">${esc(r[key])}</span><div class="bar-t"><div class="bar" style="width:${(+r[val] || 0) * 100 / max}%"></div></div><span class="bar-v">${fmtv(+r[val] || 0)}</span></div>`).join("") || '<div class="empty">لا توجد بيانات</div>'}</div>`;
}
const CHART_SETS = {
  subscribers: ["المشتركين", [["أكثر المشتركين باندويث", "bw"], ["أكثر المشتركين استخداما للباندويث", "bw"], ["أكثر المشتركين تفعيلا", "act"], ["أكثر المشتركين استخداما للكروت", "none"],
    ["أكثر المشتركين استخداما للرصيد", "money"], ["أكثر المشتركين استخداما للكروت (القيمة)", "none"], ["أكثر المشتركين إتصالا", "sess"], ["أكثر المشتركين إتصالا (الوقت)", "time"], ["أكثر المشتركين استخداما للرصيد التجريبي", "none"]]],
  managers: ["المدراء", [["أكثر المدراء تفعيلا", "mact"], ["أكثر المدراء مبيعات", "msales"], ["أكثر المدراء رصيداً", "mbal"]]],
  profiles: ["العروض", [["أكثر العروض استخداماً", "pusers"], ["أكثر العروض مبيعات", "psales"]]],
  nas: ["NAS", [["المتصلين على كل NAS", "nas"]]],
};
PAGES.charts = async () => {
  const set = new URLSearchParams(location.hash.split("?")[1] || "").get("set") || "subscribers";
  const [title, charts] = CHART_SETS[set] || CHART_SETS.subscribers;
  const today = new Date().toISOString().slice(0, 10), monthAgo = new Date(Date.now() - 30 * 864e5).toISOString().slice(0, 10);
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-bar-chart"></i> ${title}</span></div><div class="adv-body">
    <div class="adv-row"><div class="adv-f"><label>تاريخ البدء</label><input type="date" id="cFrom" value="${monthAgo}"></div><div class="adv-f"><label>تاريخ الانتهاء</label><input type="date" id="cTo" value="${today}"></div>
      <div class="adv-f"><label>المدير</label><select id="cMgr">${options(MANAGERS, 0, "username", ME.admin.username + " ( Administrator )")}</select></div>
      <div class="adv-f"><label>اعلى</label><select id="cLim">${[1, 2, 3, 5, 10, 15, 20, 25, 30].map(n => `<option ${n === 10 ? "selected" : ""}>${n}</option>`).join("")}</select></div></div>
    <div class="tabs" style="margin-bottom:12px">${Object.entries(CHART_SETS).map(([k, [t]]) => `<button class="${k === set ? "on" : ""}" data-set="${k}">${t}</button>`).join("")}</div>
    <div id="cOut"></div></div></div>`);
  document.querySelectorAll("[data-set]").forEach(b => b.onclick = () => { location.hash = "charts?set=" + b.dataset.set; });
  const load = async () => {
    const lim = +$("#cLim").value, from = $("#cFrom").value, to = $("#cTo").value;
    const [online, invs, subs, mgrs, profs] = await Promise.all([api("online"), api(`invoices?from=${from}&to=${to}`), api("subscribers?size=10000&status=online"), api("managers"), api("profiles/full")]);
    const topBy = (arr, key, val) => { const m = {}; arr.forEach(x => { const k = x[key] || "—"; m[k] = (m[k] || 0) + (+x[val] || 0); }); return Object.entries(m).map(([k, v]) => ({ k, v })).sort((a, b) => b.v - a.v).slice(0, lim); };
    // each chart: [rows {k, v}, value format]
    const data = {
      bw: () => [(online.adv || []).map(o => ({ k: o.username, v: (+o.bytes_in || 0) + (+o.bytes_out || 0) })).sort((a, b) => b.v - a.v).slice(0, lim), v => bts(v)],
      act: () => [topBy(invs.filter(i => /renew|activate/.test(i.operation)).map(i => ({ u: i.username, n: 1 })), "u", "n")],
      money: () => [topBy(invs.filter(i => +i.paid).map(i => ({ u: i.username, n: i.amount - i.discount })), "u", "n")],
      sess: () => [(online.adv || []).map(o => ({ k: o.username, v: 1 })).slice(0, lim)],
      time: () => [(online.adv || []).map(o => ({ k: o.username, v: +o.session_sec || 0 })).sort((a, b) => b.v - a.v).slice(0, lim), v => dur(v)],
      mact: () => [topBy(invs.map(i => ({ m: i.manager || i.created_by, n: 1 })), "m", "n")],
      msales: () => [topBy(invs.filter(i => +i.paid).map(i => ({ m: i.manager || i.created_by, n: i.amount - i.discount })), "m", "n")],
      mbal: () => [mgrs.map(m => ({ k: m.username, v: +m.balance || 0 })).sort((a, b) => b.v - a.v).slice(0, lim)],
      pusers: () => [profs.map(p => ({ k: p.name, v: p.users })).sort((a, b) => b.v - a.v).slice(0, lim)],
      psales: () => [topBy(invs.filter(i => +i.paid).map(i => ({ p: i.profile, n: i.amount - i.discount })), "p", "n")],
      nas: () => [topBy((online.adv || []).map(o => ({ n: o.nas_name || o.nas, c: 1 })), "n", "c")],
      none: () => [null],
    };
    const built = charts.map(([t, k]) => [t, ...data[k]()]);
    $("#cOut").innerHTML = built.map(([t, rows, f], i) => `<div class="chart-box"><div class="chart-h"><b>${t}</b>${rows ? `<button class="btn-plain sm" data-exp="${i}">تصدير</button>` : ""}</div>${rows ? barChart(rows, "k", "v", f) : '<div class="empty">هيتعمل لما الكروت تتنقل</div>'}</div>`).join("");
    document.querySelectorAll("[data-exp]").forEach(b => b.onclick = () => { const [t, rows] = built[+b.dataset.exp]; downloadCSV(t, ["الاسم", "القيمة"], rows.map(r => [r.k, r.v])); });
  };
  ["cFrom", "cTo", "cMgr", "cLim"].forEach(i => $("#" + i).onchange = load);
  load();
};

PAGES.profitloss = async () => {
  const d0 = new Date(), y = d0.getFullYear(), m = d0.getMonth();
  const iso = d => d.toISOString().slice(0, 10);
  const PERIODS = { month: ["هذا الشهر", iso(new Date(y, m, 1)), iso(d0)], last: ["الشهر الماضي", iso(new Date(y, m - 1, 1)), iso(new Date(y, m, 0))], quarter: ["ربع السنة", iso(new Date(y, m - 2, 1)), iso(d0)],
    year: [String(y), `${y}-01-01`, iso(d0)], today: ["اليوم", iso(d0), iso(d0)], week: ["الأسبوع", iso(new Date(d0 - 6 * 864e5)), iso(d0)] };
  let cur = "month";
  const box = (cls, v, l) => `<div class="small-box ${cls}"><div class="inner"><h3>${v}</h3><p>${l}</p></div></div>`;
  const render = async () => {
    const [t, from, to] = PERIODS[cur];
    let paidSum = 0, unpaidSum = 0, paidN = 0, unpaidN = 0, disc = 0, daily = {};
    if (ME.mode === "shadow" && ME.admin.role === "admin") {
      const f = Math.floor(new Date(from + "T00:00:00") / 1000), tt = Math.floor(new Date(to + "T23:59:59") / 1000);
      try { const r = await api(`adv/report_invoices?from=${f}&to=${tt}&limit=20000`); paidSum = +r.sum_paid || 0; unpaidSum = +r.sum_unpaid || 0;
        (r.data || []).forEach(i => { const p = +i.paid_unpaid; p ? paidN++ : unpaidN++; const day = new Date(+i.date_operation * 1000).toISOString().slice(0, 10); if (p) daily[day] = (daily[day] || 0) + (+i.amount || +i.new_price || 0); }); } catch (e) { toast(e.message, true); }
    } else {
      const inv = await api(`invoices?from=${from}&to=${to}`);
      inv.forEach(i => { const v = i.amount - i.discount; disc += +i.discount; if (+i.paid) { paidSum += v; paidN++; daily[i.created.slice(0, 10)] = (daily[i.created.slice(0, 10)] || 0) + v; } else { unpaidSum += v; unpaidN++; } });
    }
    const subsAll = (await api("dashboard")).subscribers, net = paidSum, margin = paidSum ? 100 : 0;
    const days = Object.entries(daily).sort(), maxD = Math.max(1, ...days.map(x => x[1]));
    $("#plOut").innerHTML = `<div class="row4">${box("bg-green", fmt(paidSum) + " EGP", "إيرادات")}${box("bg-red", "0.00 EGP", "التكاليف")}${box("bg-blue-steel", fmt(net) + " EGP", "صافي الربح")}${box("bg-g_money", margin + "%", "هامش الربح")}</div>
      <div class="row3">${box("bg-yellow", fmt(disc) + " EGP", "الخصومات")}${box("bg-green", paidN, "الفواتير المدفوعة")}${box("bg-yellow-active", fmt(unpaidSum) + " EGP", "الفواتير غير المدفوعة")}</div>
      <div class="pl-grid"><div class="adv-card"><div class="adv-head"><span>معلومات الموزع</span></div><div class="adv-body"><div class="kv"><div>الموزع</div><div>${esc(ME.admin.full_name || ME.admin.username)}</div><div>فترة</div><div>${t}</div><div>من</div><div>${from}</div><div>إلى</div><div>${to}</div></div></div></div>
        <div class="adv-card"><div class="adv-head"><span>المشتركون</span></div><div class="adv-body"><div class="kv"><div>إجمالي المشتركين</div><div>${fmt(subsAll)}</div><div>متوسط الربح لكل مشترك</div><div>${subsAll ? (paidSum / subsAll).toFixed(2) : 0} EGP</div></div></div></div>
        <div class="adv-card"><div class="adv-head"><span>ملخص الفواتير</span></div><div class="adv-body"><div class="kv"><div>فاتورة مدفوعة</div><div>${paidN}</div><div>فاتورة غير مدفوعة</div><div>${unpaidN} — ${fmt(unpaidSum)} EGP</div></div></div></div></div>
      <div class="adv-card"><div class="adv-head"><span>الإيرادات اليومية</span></div><div class="adv-body"><div class="daily">${days.map(([d, v]) => `<div class="dcol" title="${d}: ${fmt(v)}"><div class="dbar" style="height:${v * 100 / maxD}%"></div><span>${d.slice(5)}</span></div>`).join("") || '<div class="empty">لا توجد بيانات</div>'}</div></div></div>`;
  };
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-line-chart"></i> تقرير الأرباح والخسائر</span></div><div class="adv-body">
    <h3 class="form-section">الفترة الزمنية</h3><div class="adv-actions" style="margin-bottom:14px">${Object.entries(PERIODS).map(([k, [t]]) => `<button class="${k === cur ? "btn-lblue" : "btn-plain"}" data-p="${k}">${t}</button>`).join("")}<button class="btn-plain" onclick="print()">طباعة التقرير</button></div>
    <div id="plOut"><div class="empty"><i class="fa fa-spinner fa-spin"></i></div></div></div></div>`);
  document.querySelectorAll("[data-p]").forEach(b => b.onclick = () => { cur = b.dataset.p; document.querySelectorAll("[data-p]").forEach(x => x.className = x.dataset.p === cur ? "btn-lblue" : "btn-plain"); render(); });
  render();
};
