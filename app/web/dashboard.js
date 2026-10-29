// "لوحة التحكم" — modern layout (idea from Haitham's system: header with quick actions, 4 KPI cards, traffic chart,
// server resources), with REAL numbers only. ADV's invoice panels stay underneath.
"use strict";
const GB = 1073741824;
function gbText(b) { b = +b || 0; return b >= 1099511627776 ? (b / 1099511627776).toFixed(2) + " TB" : (b / GB).toFixed(1) + " GB"; }

// small SVG area chart: two series (download / upload) per day, no library
function usageChart(days) {
  if (!days || !days.length) return '<div class="empty">لسه مفيش بيانات استهلاك</div>';
  const W = 1400, H = 230, P = 44, n = days.length; // wide and short: stays a strip on big screens
  const max = Math.max(1, ...days.map(d => Math.max(+d.down || 0, +d.up || 0)));
  const TB = max >= 2 * 1099511627776, unit = TB ? 1099511627776 : GB;
  const x = i => P + (n === 1 ? (W - 2 * P) / 2 : i * (W - 2 * P) / (n - 1));
  const y = v => H - P - (v / max) * (H - 2 * P);
  const line = k => days.map((d, i) => `${i ? "L" : "M"}${x(i).toFixed(1)},${y(+d[k] || 0).toFixed(1)}`).join(" ");
  const area = k => `${line(k)} L${x(n - 1).toFixed(1)},${H - P} L${x(0).toFixed(1)},${H - P} Z`;
  const grid = [0, .25, .5, .75, 1].map(f => `<line x1="${P}" x2="${W - P}" y1="${y(max * f)}" y2="${y(max * f)}" class="dz-grid"/>
    <text x="${P - 4}" y="${y(max * f) + 4}" class="dz-tick" text-anchor="end">${(max * f / unit).toFixed(TB ? 1 : 0)}</text>`).join("");
  const labels = days.map((d, i) => i % Math.ceil(n / 7) === 0 || i === n - 1 ? `<text x="${x(i)}" y="${H - 10}" class="dz-tick" text-anchor="middle">${d.day.slice(5)}</text>` : "").join("");
  const dots = days.map((d, i) => `<circle cx="${x(i)}" cy="${y(+d.down || 0)}" r="9" class="dz-hit"><title>${d.day}\nتحميل ${gbText(d.down)}\nرفع ${gbText(d.up)}</title></circle>`).join("");
  return `<svg viewBox="0 0 ${W} ${H}" class="dz-chart" role="img" aria-label="استهلاك آخر ${n} يوم">
    <defs><linearGradient id="gDown" x1="0" x2="0" y1="0" y2="1"><stop offset="0" stop-color="var(--dz-primary)" stop-opacity=".28"/><stop offset="1" stop-color="var(--dz-primary)" stop-opacity="0"/></linearGradient>
    <linearGradient id="gUp" x1="0" x2="0" y1="0" y2="1"><stop offset="0" stop-color="var(--dz-success)" stop-opacity=".22"/><stop offset="1" stop-color="var(--dz-success)" stop-opacity="0"/></linearGradient></defs>
    ${grid}<path d="${area("down")}" fill="url(#gDown)"/><path d="${area("up")}" fill="url(#gUp)"/>
    <path d="${line("down")}" class="dz-line" stroke="var(--dz-primary)"/><path d="${line("up")}" class="dz-line" stroke="var(--dz-success)"/>${labels}${dots}</svg>`;
}

PAGES.dashboard = async () => {
  const d = await api("dashboard");
  let m = {};
  const isAdmin = ME.admin.role === "admin", money = d.inv_subscribers !== undefined; // a distributor without "إظهار الحسابات بالرئيسية" gets no money fields
  if (d.mode === "shadow" && isAdmin) { try { m = (await api("sync")).adv_metrics || {}; } catch (_) {} }
  const qs = m.quick_stats || {}, cs = m.get_cards_sales_stats || {};
  const online = d.mode === "shadow" && isAdmin ? +d.adv_online || +qs.online_users || 0 : d.online;
  const today = d.mode === "shadow" && isAdmin && !(d.inv_subscribers || {}).today ? +qs.today_sales_total || 0 : +((d.inv_subscribers || {}).today || 0);
  const cards = (cs.year || {}).count || d.cards;
  const days = d.usage_days || [], last = days[days.length - 1] || {};
  const act = (href, ic, t, cls) => `<a class="dz-act ${cls || ""}" href="#${href}"><span class="dz-bub"><i class="bi ${ic}"></i></span>${t}</a>`;
  const kpi = (cls, ic, title, val, foot, link) => `<a class="dz-kpi ${cls}" href="#${link}"><div class="dz-kpi-h"><span>${title}</span><span class="dz-ico"><i class="bi ${ic}"></i></span></div>
    <div class="dz-num">${val}</div><div class="dz-foot">${foot}</div></a>`;
  const pill = (cls, t) => `<span class="dz-pill ${cls}">${t}</span>`;
  const inv = key => `<div class="small-box bg-grey-gallery inv-panel" data-inv="${key}"><div class="inv-reload"><i class="fa fa-refresh"></i> Reload</div><div class="inv-rows">
    ${[["paid", `إجمالي الفواتير المدفوعة (${({ managers: "المدراء", subscribers: "المشتركين", cards: "الكروت" })[key]})`], ["unpaid", "إجمالي الفواتير الغير مدفوعة"], ["count", "محصلة الفواتير"],
      ["today", "محصلة الفواتير اليومية"], ["month", "محصلة الفواتير الشهرية"]].map(([k, t]) => `<div class="inv-v" data-k="${k}">---</div><div class="inv-v">---</div><div class="inv-l">${t}</div>`).join("")}
    </div><div class="icon"><i class="fa fa-money"></i></div></div>`;
  C(`<div class="dz">
    <div class="dz-card dz-head">
      <div class="dz-title"><span class="dz-bub big"><i class="bi bi-grid-1x2-fill"></i></span><div><h2>لوحة التحكم</h2>
        <small>${d.mode === "shadow" ? "وضع المراقبة — النظام بيقرا من ADV بس. آخر مزامنة: " + esc(d.last_sync || "لسه") : "وضع التشغيل — الراديوس الجديد هو الشغال"}</small></div>
        <span class="dz-status ${d.radius_ok && d.db_ok ? "ok" : "bad"}"><span class="dz-dot"></span>${d.radius_ok && d.db_ok ? "الراديوس وقاعدة البيانات شغالين" : "فيه مشكلة: " + (!d.radius_ok ? "الراديوس واقف " : "") + (!d.db_ok ? "قاعدة البيانات" : "")}</span></div>
      <div class="dz-actions">${act("user?new=1", "bi-person-plus-fill", "إضافة مشترك", "primary")}${act("subscribers?status=expiring", "bi-arrow-repeat", "تجديد المنتهين قريب", "c-green")}
        ${isAdmin ? act("cards", "bi-credit-card-2-front-fill", "كروت الشحن", "c-amber") + act("speed", "bi-speedometer2", "التحكم في السرعات", "c-indigo") + act("nas", "bi-router-fill", "الراوترات", "c-sky") : ""}
        ${act("reports", "bi-file-earmark-bar-graph-fill", "التقارير", "c-slate")}${isAdmin ? act("sync", "bi-arrow-left-right", "المزامنة مع ADV", "c-rose") : ""}</div>
    </div>
    <div class="dz-kpis">
      ${kpi("k-indigo", "bi-broadcast", "المتصلين دلوقتي", fmt(online), pill("p-green", d.mode === "shadow" ? "<span class='dz-dot'></span> من ADV — كل 20 ثانية" : "<span class='dz-dot'></span> على الراديوس بتاعنا") + (isAdmin && d.cards_online ? pill("p-slate", fmt(d.cards_online) + " كارت") : ""), "online")}
      ${kpi("k-sky", "bi-people-fill", "إجمالي المشتركين", fmt(d.subscribers), pill("p-sky", fmt(d.active) + " نشط") + pill("p-rose", fmt(d.expired) + " منتهي") + (d.disabled ? pill("p-slate", fmt(d.disabled) + " موقوف") : ""), "subscribers")}
      ${kpi("k-amber", "bi-hourglass-split", `بينتهوا خلال ${d.expiring_days || 3} أيام`, fmt(d.expiring), pill("p-amber", "محتاجين تجديد"), "subscribers?status=expiring")}
      ${money ? kpi("k-green", "bi-wallet2", "تحصيل النهارده", fmt(today) + " <small>EGP</small>", pill("p-green", "<i class='bi bi-check-circle-fill'></i> الشهر: " + fmt((d.inv_subscribers || {}).month || 0)), "invoices")
        : isAdmin ? kpi("k-green", "bi-hdd-network-fill", "راوترات شغالة", fmt(d.nas_active), pill("p-green", "عليها مشتركين متصلين"), "nas") : ""}
    </div>
    <div class="dz-kpis small">
      ${isAdmin ? kpi("k-green", "bi-hdd-network-fill", "راوترات عليها متصلين", fmt(d.nas_active), pill("p-slate", fmt(d.nas_total) + " متسجلة عندنا"), "nas") : ""}
      ${money ? kpi("k-violet", "bi-cash-coin", typeof d.balance === "number" ? "رصيدك" : "الرصيد", typeof d.balance === "number" ? fmt(d.balance) : esc(d.balance), pill("p-violet", "مستلف: " + fmt(d.borrowed_total || m.borrowed_total || 0)), "managers") : ""}
      ${isAdmin ? kpi("k-amber", "bi-credit-card-2-front-fill", "كروت", fmt(cards), pill("p-amber", fmt(d.cards_used || 0) + " مستخدم"), "cardusers") : ""}
      ${kpi("k-indigo", "bi-cloud-arrow-down-fill", "استهلاك النهارده", gbText((+last.down || 0) + (+last.up || 0)), pill("p-indigo", "تحميل " + gbText(last.down)) + pill("p-green", "رفع " + gbText(last.up)), "reports")}
    </div>
    <div class="dz-card"><div class="dz-card-h"><span><i class="bi bi-graph-up"></i> استهلاك الشبكة — آخر ${days.length || 14} يوم (${(Math.max(0, ...days.map(x => Math.max(+x.down || 0, +x.up || 0))) >= 2 * 1099511627776) ? "تيرا" : "جيجا"})</span>
      <span class="dz-legend"><i style="background:var(--dz-primary)"></i>تحميل <i style="background:var(--dz-success)"></i>رفع</span></div>${usageChart(days)}</div>
    <div class="dz-card"><div class="dz-card-h"><span><i class="bi bi-cpu"></i> موارد السيرفر</span><span class="dz-pill p-slate">${esc(d.version || "")}</span></div>
      <div class="dz-res">${[["cpu", "bi-cpu", "المعالج"], ["ram", "bi-memory", "الرام"], ["disk", "bi-device-hdd", "الهارد"], ["uptime", "bi-clock-history", "مدة التشغيل"],
        ["ping_radius", "bi-activity", d.mode === "shadow" ? "Ping على ADV Radius" : "Ping على الراديوس"], ["ping_google", "bi-globe2", "Ping على Google"], ["now", "bi-calendar-event", "وقت السيرفر"]]
        .map(([k, ic, t]) => `<div class="dz-tile"><div class="dz-tile-h"><i class="bi ${ic}"></i>${t}</div><div class="dz-tile-v" data-sys="${k}">...</div><div class="dz-bar"><i data-bar="${k}"></i></div></div>`).join("")}</div></div>
    ${money ? `<div class="dz-card"><div class="dz-card-h"><span><i class="bi bi-receipt"></i> الفواتير (زي ADV — دوس Reload)</span></div><div class="row3">${inv("managers")}${inv("subscribers")}${inv("cards")}</div></div>` : ""}
  </div>`);
  // invoice panels: values only on Reload, like ADV
  const invData = { managers: d.inv_managers, subscribers: d.inv_subscribers, cards: d.inv_cards };
  if (d.mode === "shadow") {
    if (!(d.inv_subscribers || {}).count) invData.subscribers = { paid: qs.today_sales_total || 0, unpaid: qs.unpaid_total || 0, count: qs.today_sales_count || 0, today: qs.today_sales_total || 0, month: "---" };
    invData.cards = { paid: (cs.year || {}).total || 0, unpaid: 0, count: (cs.year || {}).count || 0, today: (cs.day || {}).total || 0, month: (cs.month || {}).total || 0 };
  }
  document.querySelectorAll(".inv-panel").forEach(p => p.querySelector(".inv-reload").onclick = () => {
    const v = invData[p.dataset.inv] || {};
    p.querySelectorAll(".inv-v[data-k]").forEach(e => { const x = v[e.dataset.k]; e.textContent = x === "---" || x === undefined ? "---" : fmt(x); });
  });
  api("sysinfo").then(s => document.querySelectorAll("[data-sys]").forEach(e => {
    const k = e.dataset.sys, v = s[k];
    e.innerHTML = v && typeof v === "object" ? `${esc(v.ms)} <small>فقد ${esc(v.loss)}</small>` : esc(v ?? "--");
    const bar = document.querySelector(`[data-bar="${k}"]`), pct = parseFloat(v);
    if (bar) { if (/%$/.test(String(v)) && !isNaN(pct)) { bar.style.width = Math.min(100, pct) + "%"; bar.className = pct > 85 ? "hot" : pct > 65 ? "warm" : ""; } else bar.parentElement.style.visibility = "hidden"; }
  })).catch(() => {});
};
