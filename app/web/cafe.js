// "كروت الكافيهات": card series per café (like ADV's "مشتركين الكارت") on our RADIUS — create, list the cards,
// print with a card template, online cards, check one card, sales/profit.
"use strict";
const CAFE_TABS = [["series", "bi-collection", "الحزم"], ["online", "bi-broadcast", "الكروت المتصلة"], ["check", "bi-search", "فحص كارت"], ["report", "bi-cash-coin", "المبيعات والأرباح"]];
const cardStatus = s => ({ new: '<span class="dz-pill p-sky">متاح</span>', active: '<span class="dz-pill p-green">شغال</span>', expired: '<span class="dz-pill p-rose">انتهى وقته</span>',
  quota: '<span class="dz-pill p-amber">خلصت الكوتة</span>', disabled: '<span class="dz-pill p-slate">موقوف</span>' })[s] || esc(s);
const unitTxt = u => ({ 1: "يوم", 2: "ساعة", 3: "دقيقة" })[u] || "ساعة";
const modeTxt = m => ({ first_login: "من أول اتصال", creation: "من الإنشاء", online: "وقت اتصال فعلي" })[m] || m;

PAGES.cafe = async () => {
  const tab = (location.hash.split("tab=")[1] || "series").split("&")[0];
  const head = `<div class="dz-card dz-head"><div class="dz-title"><span class="dz-bub big"><i class="bi bi-cup-hot-fill"></i></span>
    <div><h2>كروت الكافيهات</h2><small>الكارت بيشتغل في الكافيه بتاعه بس، ووقته وكوتته بيتحسبوا هنا على الراديوس بتاعنا.</small></div></div>
    <div class="set-tabs" style="margin:14px 0 0">${CAFE_TABS.map(([k, ic, t]) => `<a class="${k === tab ? "on" : ""}" href="#cafe?tab=${k}"><i class="bi ${ic}"></i>${t}</a>`).join("")}</div></div>`;
  if (tab === "online") return cafeOnline(head);
  if (tab === "check") return cafeCheck(head);
  if (tab === "report") return cafeReport(head);
  return cafeSeries(head);
};

async function cafeSeries(head) {
  const [d, profs, mgrs] = await Promise.all([api("cafe"), api("profiles"), api("managers").catch(() => [])]);
  C(`<div class="dz">${head}
    <div class="dz-card"><div class="dz-card-h"><span><i class="bi bi-collection"></i> الحزم (${d.series.length})</span>
      <button class="green" id="cfNew" ${d.cafes.length ? "" : "disabled"}><i class="bi bi-plus-lg"></i> إنشاء حزمة</button></div>
      ${d.cafes.length ? "" : '<div class="hint-box">ضيف الكافيه الأول من صفحة <a href="#vpn">VPN الكافيهات</a>.</div>'}
      ${d.series.length ? `<div class="tablewrap"><table><tr><th>#</th><th>الحزمة</th><th>الكافيه</th><th>الباقة</th><th>الكمية</th><th>متاح</th><th>شغال</th><th>انتهى</th><th>متصل</th>
        <th>الوقت</th><th>الكوتة</th><th>السعر / التكلفة</th><th>الأجهزة</th><th>انشئ</th><th></th></tr>
        ${d.series.map(s => `<tr class="${+s.enabled ? "" : "muted"}"><td>${s.id}</td><td><b>${esc(s.name)}</b>${s.notes ? `<br><small class="muted">${esc(s.notes)}</small>` : ""}</td>
          <td>${esc(s.cafe || "—")}</td><td>${esc(s.profile || "")}</td><td>${fmt(s.qty)}</td><td>${fmt(s.available)}</td><td>${fmt(s.active)}</td><td>${fmt(s.ended)}</td><td>${fmt(s.online)}</td>
          <td>${+s.time_val ? `${s.time_val} ${unitTxt(+s.time_unit)}<br><small class="muted">${modeTxt(s.time_mode)}</small>` : "مفتوح"}</td>
          <td>${+s.quota_mb ? fmt(+s.quota_mb >= 1024 ? (s.quota_mb / 1024).toFixed(1) + " GB" : s.quota_mb + " MB") : "غير محدود"}</td>
          <td>${fmt(s.price)} / ${fmt(s.cost)}</td><td>${+s.mac_max ? s.mac_max + (+s.mac_max === 1 ? " جهاز" : " أجهزة") : "أي جهاز"}</td><td><small>${esc(String(s.created).slice(0, 16))}</small></td>
          <td class="vpn-act"><button class="b-light" data-cards="${s.id}"><i class="bi bi-list-ul"></i> الكروت</button><button class="b-light" data-print="${s.id}"><i class="bi bi-printer"></i> طباعة</button>
            <button class="b-light" data-csv="${s.id}" data-name="${esc(s.name)}">CSV</button><button class="b-light" data-wa="${s.id}" title="ملخص لصاحب الكافيه"><i class="bi bi-whatsapp"></i></button>
            <button class="b-light" data-tog="${s.id}">${+s.enabled ? "إيقاف" : "تشغيل"}</button><button class="btn-red" data-del="${s.id}">حذف غير المستخدم</button></td></tr>`).join("")}</table></div>`
        : '<div class="empty">لسه مفيش حزم.</div>'}
      <div id="cfCards"></div></div></div>`);
  const post = async (body, msg) => { const r = await act(() => api("cafe", { method: "POST", body }), msg); return r; };
  $("#cfNew").onclick = () => cafeCreateModal(d.cafes, profs, mgrs);
  document.querySelectorAll("[data-tog]").forEach(b => b.onclick = async () => { await post({ Action: "toggle_series", ID: +b.dataset.tog }, "تم"); PAGES.cafe(); });
  document.querySelectorAll("[data-del]").forEach(b => b.onclick = async () => { if (!confirm("حذف كل الكروت اللي لسه ما اتستخدمتش في الحزمة دي؟")) return; const r = await post({ Action: "delete_unused", ID: +b.dataset.del }); toast(`اتحذف ${r.deleted} كارت`); PAGES.cafe(); });
  document.querySelectorAll("[data-wa]").forEach(b => b.onclick = () => post({ Action: "whatsapp", ID: +b.dataset.wa }, "الملخص اتحط في طابور الواتساب"));
  document.querySelectorAll("[data-cards]").forEach(b => b.onclick = async () => {
    const r = await api("cafe?series=" + b.dataset.cards);
    $("#cfCards").innerHTML = `<h4 style="margin:16px 0 8px">كروت الحزمة ${b.dataset.cards}</h4><div class="tablewrap">${table(r.cards, [["username", "الكارت", v => `<span dir="ltr">${esc(v)}</span>`],
      ["password", "الباسورد", v => `<span dir="ltr">${esc(v || "—")}</span>`], ["status", "الحالة", v => cardStatus(v)], ["first_login", "أول اتصال"], ["expires", "ينتهي"],
      ["used_mb", "استهلك (MB)"], ["used_min", "دقائق"], ["mac", "MAC", v => `<span dir="ltr">${esc(v)}</span>`], ["online", "متصل", v => +v ? '<span class="dz-pill p-green">متصل</span>' : ""]])}</div>`;
    $("#cfCards").scrollIntoView({ behavior: "smooth" });
  });
  document.querySelectorAll("[data-csv]").forEach(b => b.onclick = async () => {
    const r = await api("cafe?series=" + b.dataset.csv);
    downloadCSV("cafe-" + b.dataset.name, ["الكارت", "الباسورد", "الحالة", "أول اتصال", "ينتهي", "MAC"], r.cards.map(c => [c.username, c.password, c.status, c.first_login || "", c.expires || "", c.mac]));
  });
  document.querySelectorAll("[data-print]").forEach(b => b.onclick = async () => {
    const s = d.series.find(x => +x.id === +b.dataset.print);
    const sets = Object.fromEntries((await api("settings")).settings.map(x => [x.k, x.v]));
    let tpls = []; try { tpls = JSON.parse(sets.card_templates || "[]"); } catch (_) {}
    if (!tpls.length) { toast("اعمل قالب الأول من «تصميم الكروت والطباعة»", true); location.hash = "cardtpl"; return; }
    const t = tpls.length === 1 ? tpls[0] : tpls[Math.max(0, +prompt("رقم القالب:\n" + tpls.map((x, i) => `${i + 1}- ${x.name}`).join("\n"), "1") - 1)] || tpls[0];
    const r = await api("cafe?series=" + s.id);
    const cards = r.cards.filter(c => c.status === "new");
    if (!cards.length) return toast("مفيش كروت متاحة للطباعة", true);
    const per = t.cols * t.rows, pages = [];
    for (let i = 0; i < cards.length; i += per) pages.push(cards.slice(i, i + per));
    const w = window.open("", "_blank");
    w.document.write(`<!doctype html><html lang="ar" dir="rtl"><head><meta charset="utf-8"><title>${esc(s.name)}</title><style>${CT_PRINT_CSS}.sheet{grid-template-columns:repeat(${t.cols},${t.w}mm);gap:${t.gap}mm}</style></head><body>
      ${pages.map(p => `<div class="sheet">${p.map(c => cardHTML(t, { code: c.password && c.password !== c.username ? `${c.username} / ${c.password}` : c.username, price: fmt(s.price), profile: s.profile || "", series: s.name, company: sets.company || "" }, t.qr.on ? qrFor(c.username) : "")).join("")}</div>`).join("")}
      <script>setTimeout(()=>print(),400)<\/script></body></html>`);
    w.document.close();
  });
}

function cafeCreateModal(cafes, profs, mgrs) {
  modal(`<h3><i class="bi bi-plus-circle"></i> إنشاء حزمة كروت كافيه</h3><form id="cfForm" class="g12">
    <div class="fg s6"><label>اسم الحزمة</label><input name="Name" required></div>
    <div class="fg s6"><label>الكافيه</label><select name="CafeID">${cafes.map(c => `<option value="${c.id}">${esc(c.name)} (${esc(c.ip)})</option>`).join("")}</select></div>
    <div class="fg s6"><label>الباقة (السرعة)</label><select name="ProfileID">${profs.map(p => `<option value="${p.id}">${esc(p.name)}</option>`).join("")}</select></div>
    <div class="fg s3"><label>العدد (لحد 1000)</label><input name="Qty" type="number" min="1" max="1000" value="100"></div>
    <div class="fg s3"><label>عدد الأجهزة</label><input name="Simul" type="number" min="1" max="10" value="1"></div>
    <div class="fg s3"><label>الوقت</label><input name="TimeVal" type="number" min="0" value="2"></div>
    <div class="fg s3"><label>الوحدة</label><select name="TimeUnit"><option value="3">دقيقة</option><option value="2" selected>ساعة</option><option value="1">يوم</option></select></div>
    <div class="fg s6"><label>طريقة الاحتساب</label><select name="TimeMode"><option value="first_login">من أول اتصال</option><option value="creation">من الإنشاء</option><option value="online">وقت اتصال فعلي بس (بالثانية)</option></select></div>
    <div class="fg s4"><label>الكوتة (ميجا، 0 = مفتوح)</label><input name="QuotaMB" type="number" min="0" value="0"></div>
    <div class="fg s4"><label>عند انتهاء الكوتة</label><select name="QuotaStop"><option value="1">إيقاف الكارت</option><option value="0">ولا حاجة</option></select></div>
    <div class="fg s4"><label>ينتهي في (اختياري)</label><input name="EndDate" type="date"></div>
    <div class="fg s3"><label>سعر البيع للكارت</label><input name="Price" type="number" min="0" step="0.5" value="0"></div>
    <div class="fg s3"><label>التكلفة على الكافيه</label><input name="Cost" type="number" min="0" step="0.5" value="0"></div>
    <div class="fg s6"><label>المدير (يتخصم منه التكلفة في Live)</label><select name="ManagerID"><option value="0">— بدون —</option>${mgrs.map(m => `<option value="${m.id}">${esc(m.username)}</option>`).join("")}</select></div>
    <div class="fg s3"><label>البادئة</label><input name="Prefix" dir="ltr" maxlength="6"></div>
    <div class="fg s3"><label>طول الكارت</label><input name="CodeLen" type="number" min="6" max="16" value="8"></div>
    <div class="fg s3"><label>الحروف</label><select name="Charset"><option value="num">أرقام بس</option><option value="mixed">أرقام وحروف</option></select></div>
    <div class="fg s3"><label>الباسورد</label><select name="PassMode"><option value="1">نفس الكارت</option><option value="0">من غير</option><option value="2">ثابت</option><option value="4">4 أرقام عشوائية</option><option value="6">6 أرقام عشوائية</option></select></div>
    <div class="fg s4 hidden" id="cfFixed"><label>الباسورد الثابت</label><input name="FixedPass" dir="ltr"></div>
    <div class="fg s4"><label>اسم سيرفر الهوتسبوت (اختياري)</label><input name="ServiceName" dir="ltr" placeholder="all"></div>
    <div class="fg s4"><label>عدد الأجهزة المسموح بيها (MAC)</label><select name="MacMax"><option value="0">أي جهاز</option><option value="1">جهاز واحد</option><option value="2" selected>جهازين</option><option value="3">3 أجهزة</option></select></div>
    <div class="fg s12"><label>ملاحظات</label><input name="Notes"></div>
    <div class="fg s12"><button class="green"><i class="bi bi-check2"></i> إنشاء</button></div></form>`);
  $("#cfForm").PassMode.onchange = e => $("#cfFixed").classList.toggle("hidden", e.target.value !== "2");
  $("#cfForm").onsubmit = async e => {
    e.preventDefault();
    const f = formData(e.target), num = k => +f[k] || 0;
    const r = await act(() => api("cafe", { method: "POST", body: { Action: "create", Name: f.Name, CafeID: num("CafeID"), ProfileID: num("ProfileID"), Qty: num("Qty"), Simul: num("Simul"),
      TimeVal: num("TimeVal"), TimeUnit: num("TimeUnit"), TimeMode: f.TimeMode, QuotaMB: num("QuotaMB"), QuotaStop: f.QuotaStop === "1", EndDate: f.EndDate || "",
      Price: num("Price"), Cost: num("Cost"), ManagerID: num("ManagerID"), Prefix: f.Prefix || "", CodeLen: num("CodeLen"), Charset: f.Charset, PassMode: num("PassMode"),
      FixedPass: f.FixedPass || "", ServiceName: f.ServiceName === "all" ? "" : (f.ServiceName || ""), MacMax: num("MacMax"), Notes: f.Notes || "" } }));
    if (r) { toast(`اتعمل ${r.made} كارت`); closeModal(); PAGES.cafe(); }
  };
}

async function cafeOnline(head) {
  const rows = await api("cafe?online=1");
  C(`<div class="dz">${head}<div class="dz-card"><div class="dz-card-h"><span><i class="bi bi-broadcast"></i> الكروت المتصلة (${rows.length})</span><button class="b-light" onclick="PAGES.cafe()"><i class="bi bi-arrow-clockwise"></i></button></div>
    <div class="tablewrap">${table(rows, [["username", "الكارت", v => `<span dir="ltr">${esc(v)}</span>`], ["cafe", "الكافيه"], ["series", "الحزمة"], ["profile", "الباقة"],
      ["framedipaddress", "IP", v => `<span dir="ltr">${esc(v)}</span>`], ["callingstationid", "MAC", v => `<span dir="ltr">${esc(v)}</span>`], ["uptime", "مدة الجلسة"],
      ["down_mb", "تحميل (MB)"], ["up_mb", "رفع (MB)"], ["expires", "ينتهي"], ["username", "", v => `<button class="btn-red" data-dc="${esc(v)}">فصل</button>`]])}</div></div></div>`);
  document.querySelectorAll("[data-dc]").forEach(b => b.onclick = async () => { await act(() => api("cafe", { method: "POST", body: { Action: "disconnect", Card: b.dataset.dc } }), "اتفصل"); setTimeout(() => PAGES.cafe(), 1500); });
}

async function cafeCheck(head) {
  C(`<div class="dz">${head}<form class="dz-card" id="ckF"><div class="dz-card-h"><span><i class="bi bi-search"></i> فحص كارت</span></div>
    <div class="g12"><div class="fg s9"><input name="q" dir="ltr" placeholder="رقم الكارت أو الـ MAC" required></div><div class="fg s3"><button class="green">فحص</button></div></div></form><div id="ckOut"></div></div>`);
  $("#ckF").onsubmit = async e => {
    e.preventDefault();
    const q = formData(e.target).q.trim();
    const r = await act(() => api("cafe?card=" + encodeURIComponent(q)));
    if (!r) return;
    const c = r.card, quota = +c.quota_mb, usedMB = +c.used_bytes / 1048576;
    $("#ckOut").innerHTML = `<div class="dz-card"><div class="dz-card-h"><span dir="ltr">${esc(c.username)}</span>${cardStatus(c.status)}</div>
      <div class="kv"><div>الكافيه</div><div>${esc(c.cafe || "")}</div><div>الحزمة</div><div>${esc(c.series)}</div><div>الباقة</div><div>${esc(c.profile || "")}</div>
        <div>أول اتصال</div><div>${esc(c.first_login || "لسه")}</div><div>ينتهي</div><div>${esc(c.expires || "—")}</div>
        <div>الاستهلاك</div><div>${usedMB.toFixed(1)} MB${quota ? ` من ${fmt(quota)} MB` : ""}</div><div>وقت الاتصال</div><div>${Math.round(+c.used_secs / 60)} دقيقة</div>
        <div>الأجهزة</div><div dir="ltr">${esc((c.macs || c.mac || "—").split(",").join(" ، "))}${+c.mac_max ? ` <small>(مسموح ${c.mac_max})</small>` : ""}</div></div>
      <div class="adv-actions" style="margin-top:12px"><button class="b-light" data-a="disconnect">فصل</button><button class="b-light" data-a="unlock_mac">مسح الأجهزة</button>
        <button class="btn-orange" data-a="reset_card">إعادة تفعيل (يرجع جديد)</button><button class="btn-red" data-a="disable_card">إيقاف الكارت</button></div>
      <h4>آخر الجلسات</h4><div class="tablewrap">${table(r.sessions, [["acctstarttime", "البداية"], ["acctstoptime", "النهاية"], ["duration", "المدة"], ["down_mb", "تحميل MB"], ["up_mb", "رفع MB"],
        ["callingstationid", "MAC"], ["login", "دخل بـ", v => v === c.username ? "الكود" : "الماك"], ["acctterminatecause", "سبب الفصل"]])}</div>
      <h4>الفصل وأسبابه</h4>${(r.events || []).length ? `<div class="tablewrap">${table(r.events, [["ts", "الوقت"], ["reason_text", "السبب"], ["detail", "تفاصيل", v => `<span dir="ltr">${esc(v)}</span>`]])}</div>` : '<div class="empty">مفيش فصل اتسجل.</div>'}</div>`;
    document.querySelectorAll("#ckOut [data-a]").forEach(b => b.onclick = async () => {
      if (b.dataset.a === "reset_card" && !confirm("الكارت هيرجع جديد: وقته وكوتته والماك يبدأوا من الأول. تكمل؟")) return;
      await act(() => api("cafe", { method: "POST", body: { Action: b.dataset.a, Card: c.username } }), "تم"); $("#ckF").requestSubmit();
    });
  };
}

async function cafeReport(head) {
  const rows = await api("cafe?report=1");
  const tot = k => rows.reduce((n, r) => n + (+r[k] || 0), 0);
  C(`<div class="dz">${head}<div class="dz-kpis">
      <div class="dz-kpi k-green"><div class="dz-kpi-h"><span>المبيعات (الكروت المستخدمة)</span><span class="dz-ico"><i class="bi bi-cash-stack"></i></span></div><div class="dz-num">${fmt(tot("sales"))}</div></div>
      <div class="dz-kpi k-indigo"><div class="dz-kpi-h"><span>الربح</span><span class="dz-ico"><i class="bi bi-graph-up-arrow"></i></span></div><div class="dz-num">${fmt(tot("profit"))}</div></div>
      <div class="dz-kpi k-sky"><div class="dz-kpi-h"><span>كروت مستخدمة / متاحة</span><span class="dz-ico"><i class="bi bi-credit-card-2-front"></i></span></div><div class="dz-num">${fmt(tot("used"))} / ${fmt(tot("available"))}</div></div></div>
    <div class="dz-card"><div class="tablewrap">${table(rows, [["name", "الحزمة"], ["cafe", "الكافيه"], ["profile", "الباقة"], ["qty", "الكمية"], ["used", "مستخدم"], ["available", "متاح"],
      ["active", "شغال دلوقتي"], ["price", "سعر البيع"], ["cost", "التكلفة"], ["sales", "المبيعات"], ["profit", "الربح"], ["created", "انشئ"]])}</div></div></div>`);
}
