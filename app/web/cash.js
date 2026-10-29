// "معاملات الكاش": payments the StreamCash phone app reads (Vodafone Cash SMS, the bank's InstaPay credit SMS, InstaPay
// notifications) — same columns as ADV's page — plus the paired phones, the points meter and the messages not understood.
"use strict";
const CASH_TABS = [["tx", "bi-cash-stack", "المعاملات"], ["phones", "bi-phone", "الموبايلات"], ["unknown", "bi-question-circle", "رسايل مش مفهومة"], ["adv", "bi-arrow-left-right", "مقارنة ADV"]];
const cashStatus = s => ({ new: '<span class="dz-pill p-sky">جديدة</span>', used: '<span class="dz-pill p-green">مستخدم</span>',
  dup: '<span class="dz-pill p-amber">مكررة</span>', void: '<span class="dz-pill p-slate">ملغية</span>' })[s] || esc(s);
// the payment kind on every row (the المصدر column is off-screen on a phone)
const cashKind = src => src === "vfcash" ? '<span class="dz-pill p-rose">VF</span>'
  : src === "instapay" || src === "ipnotif" ? '<span class="dz-pill p-violet">إنستاباي</span>' : '<span class="dz-pill p-slate">يدوي</span>';
const cashDT = v => v ? esc(String(v).replace("T", " ").slice(0, 19)) : "";

PAGES.cash = async () => {
  const tab = (location.hash.split("tab=")[1] || "tx").split("&")[0];
  const head = `<div class="dz-card dz-head"><div class="dz-title"><span class="dz-bub big"><i class="bi bi-cash-coin"></i></span>
    <div><h2>معاملات الكاش</h2><small>أبلكيشن StreamCash على موبايلك بيقرا رسايل فودافون كاش والبنك وإشعارات إنستاباي ويبعتها هنا. كل تحويل ليه رقم معاملة بيتستخدم مرة واحدة.</small></div></div>
    <div class="set-tabs" style="margin:14px 0 0">${CASH_TABS.map(([k, ic, t]) => `<a class="${k === tab ? "on" : ""}" href="#cash?tab=${k}"><i class="bi ${ic}"></i>${t}</a>`).join("")}</div></div>`;
  if (tab === "phones") return cashPhones(head);
  if (tab === "unknown") return cashUnknown(head);
  if (tab === "adv") return cashAdv(head);
  return cashList(head);
};

async function cashList(head) {
  const p = new URLSearchParams(location.hash.split("?")[1] || "");
  const today = new Date().toISOString().slice(0, 10), monthAgo = new Date(Date.now() - 30 * 864e5).toISOString().slice(0, 10);
  const from = p.get("from") || monthAgo, to = p.get("to") || today, q = p.get("q") || "";
  const [d, dev] = await Promise.all([api(`cash?from=${from}&to=${to}&q=${encodeURIComponent(q)}`), api("cash?devices=1")]);
  const rows = d.rows || [], live = rows.filter(r => r.status === "new" || r.status === "used");
  const sum = a => a.reduce((s, r) => s + +r.amount, 0);
  const fresh = rows.filter(r => r.status === "new"), todays = live.filter(r => String(r.tx_time || r.received).slice(0, 10) === today);
  const idle = (dev.devices || []).filter(x => +x.enabled && +x.paired && (x.idle_min === null || +x.idle_min > 30));
  C(`<div class="dz">${head}
    ${idle.length ? `<div class="hint-box" style="border-color:#f43f5e"><i class="bi bi-exclamation-triangle-fill"></i> الموبايل ${idle.map(x => `<b>${esc(x.name)}</b>`).join("، ")} ما كلّمش السيرفر من ${idle.map(x => x.idle_min === null ? "ساعة ما اتربط" : fmt(x.idle_min) + " دقيقة").join("، ")} — افتح الأبلكيشن عليه.</div>` : ""}
    ${+d.unknown ? `<div class="hint-box"><i class="bi bi-question-circle"></i> فيه ${fmt(d.unknown)} رسالة من الأرقام المتابعة ما اتفهمتش — <a href="#cash?tab=unknown">شوفها</a>.</div>` : ""}
    <div class="dz-kpis">
      <div class="dz-kpi k-green"><div class="dz-kpi-h"><span>النهارده</span><span class="dz-ico"><i class="bi bi-calendar-check"></i></span></div><div class="dz-num">${fmt(sum(todays))}</div><small>${fmt(todays.length)} تحويل</small></div>
      <div class="dz-kpi k-sky"><div class="dz-kpi-h"><span>لسه ما اتستخدمتش</span><span class="dz-ico"><i class="bi bi-hourglass-split"></i></span></div><div class="dz-num">${fmt(sum(fresh))}</div><small>${fmt(fresh.length)} تحويل</small></div>
      <div class="dz-kpi k-violet"><div class="dz-kpi-h"><span>في الفترة</span><span class="dz-ico"><i class="bi bi-cash-stack"></i></span></div><div class="dz-num">${fmt(sum(live))}</div><small>${fmt(live.length)} تحويل</small></div>
    </div>
    <div class="dz-card"><div class="dz-card-h"><span><i class="bi bi-cash-stack"></i> معاملات الكاش (تم إيجاد ${fmt(rows.length)} نتائج)</span>
      <span><button class="b-light" id="cxCsv">CSV</button> <button class="green" id="cxAdd"><i class="bi bi-plus-lg"></i> معاملة يدوي</button></span></div>
      <form id="cxF" class="g12"><div class="fg s3"><label>تاريخ البدء</label><input type="date" name="from" value="${from}"></div><div class="fg s3"><label>تاريخ الانتهاء</label><input type="date" name="to" value="${to}"></div>
        <div class="fg s4"><label>بحث (رقم معاملة، محفظة، اسم، مبلغ، مشترك)</label><input name="q" value="${esc(q)}"></div><div class="fg s2"><label>&nbsp;</label><button class="green">بحث</button></div></form>
      ${rows.length ? `<div class="tablewrap"><table><tr><th>#</th><th>رقم المعاملة</th><th>رقم المحفظة / إنستاباي</th><th>الاسم</th><th>المبلغ</th><th>وقت التحويل</th><th>المصدر</th><th>الحالة</th><th>بواسطة</th><th>تاريخ الدفع</th><th>المدير</th><th></th></tr>
        ${rows.map((r, i) => `<tr class="${r.status === "void" || r.status === "dup" ? "muted" : ""}"><td>${i + 1}</td><td dir="ltr"><b>${esc(r.code)}</b> ${cashKind(r.source)}</td><td dir="ltr">${esc(r.wallet)}</td><td>${esc(r.sender_name || "")}</td>
          <td><b>${Number(r.amount).toFixed(2)}</b></td><td dir="ltr">${cashDT(r.tx_time)}</td><td><small>${esc(r.source_text || r.source)}</small>${r.ext_ref ? `<br><small class="muted" dir="ltr">${esc(r.ext_ref)}</small>` : ""}</td>
          <td>${cashStatus(r.status)}${r.status === "dup" ? `<br><small class="muted">نسخة من رقم #${esc(r.dup_of)}</small>` : ""}${r.notes ? `<br><small class="muted">${esc(r.notes)}</small>` : ""}</td>
          <td dir="ltr">${esc(r.used_for || "")}${r.suggest ? `<small class="muted">غالبًا: ${esc(r.suggest)}</small>` : ""}</td><td dir="ltr">${cashDT(r.used_at)}</td><td>${esc(r.used_by || "")}</td>
          <td>${r.merged_id && r.status !== "used" ? `<button class="b-light" data-rest="${r.merged_id}" title="لو دول تحويلين مختلفين بنفس المبلغ">فك الدمج</button> ` : ""}${r.status === "new" ? `<button class="green" data-use="${r.code}" data-sug="${esc((r.suggest || "").split("، ")[0])}">أضف لمشترك</button> <button class="b-light" data-void="${r.id}">إلغاء</button>` : ""}${r.status === "void" || r.status === "dup" ? `<button class="b-light" data-rest="${r.id}" title="${r.status === "dup" ? "لو اتدمجت غلط مع تحويل تاني بنفس المبلغ" : ""}">${r.status === "dup" ? "فك الدمج" : "رجّعها"}</button>` : ""}</td></tr>`).join("")}</table></div>`
        : '<div class="empty">مفيش معاملات في الفترة دي.</div>'}</div></div>`);
  $("#cxF").onsubmit = e => { e.preventDefault(); const f = formData(e.target); location.hash = `cash?tab=tx&from=${f.from}&to=${f.to}&q=${encodeURIComponent(f.q)}`; };
  const post = async (body, msg) => { await act(() => api("cash", { method: "POST", body }), msg); cashList(head); };
  document.querySelectorAll("[data-void]").forEach(b => b.onclick = () => confirm("إلغاء المعاملة دي؟ (مش هتتستخدم لأي مشترك)") && post({ Action: "void", ID: +b.dataset.void }, "اتلغت"));
  document.querySelectorAll("[data-rest]").forEach(b => b.onclick = () => confirm("ترجع المعاملة دي كمعاملة مستقلة تتستخدم؟") && post({ Action: "restore", ID: +b.dataset.rest }, "رجعت"));
  document.querySelectorAll("[data-use]").forEach(b => b.onclick = async () => {
    const u = (prompt("اسم المستخدم (اليوزر) اللي هتتضاف له المعاملة " + b.dataset.use, b.dataset.sug || "") || "").trim();
    if (!u) return;
    const f = await api("subscribers?q=" + encodeURIComponent(u)).catch(() => null);
    const s = f && (f.rows || []).find(x => x.username === u);
    if (!s) return toast("المشترك «" + u + "» مش موجود", true);
    const renew = confirm("المبلغ هيتضاف لرصيد " + u + ".\nلو الرصيد يكفي تجديد الباقة، أجدد كمان؟ (OK = جدد، Cancel = رصيد بس)");
    const r = await act(() => api(`subscribers/${s.id}/op`, { method: "POST", body: { Action: "trx", Target: String(b.dataset.use), Count: renew ? 1 : 0 } }));
    if (r) { toast(`اتضاف ${Number(r.amount).toFixed(2)} لـ ${u} — رصيده دلوقتي ${Number(r.money).toFixed(2)}` + (r.expiration ? " — واتجدد لحد " + r.expiration : r.note ? " — " + r.note : "")); cashList(head); }
  });
  $("#cxCsv").onclick = () => downloadCSV("cash.csv", ["رقم المعاملة", "رقم المحفظة", "الاسم", "المبلغ", "وقت التحويل", "المصدر", "مرجع", "الحالة", "بواسطة", "تاريخ الدفع", "المدير"],
    rows.map(r => [r.code, r.wallet, r.sender_name, r.amount, cashDT(r.tx_time), r.source_text, r.ext_ref || "", r.status, r.used_for || "", cashDT(r.used_at), r.used_by || ""]));
  $("#cxAdd").onclick = () => {
    const m = modal(`<h3>معاملة يدوي</h3><p class="muted">لتحويل وصلك فعلًا (شفته في البنك أو إنستاباي) ومفيش رسالة جت بيه. رقم العملية بتاع البنك/المحفظة مطلوب عشان ما يتكررش.</p>
      <form id="mf" class="g12"><div class="fg s6"><label>المبلغ</label><input name="Amount" type="number" step="0.01" min="0" required></div><div class="fg s6"><label>رقم العملية (من البنك / المحفظة)</label><input name="Ref" dir="ltr" required></div>
      <div class="fg s6"><label>رقم المحفظة / عنوان إنستاباي</label><input name="Wallet" dir="ltr"></div><div class="fg s6"><label>وقت التحويل</label><input name="Time" type="datetime-local"></div>
      <div class="fg s12"><label>ملاحظات</label><input name="Notes"></div><div class="fg s12"><button class="green">إضافة</button></div></form>`);
    $("#mf", m).onsubmit = async e => { e.preventDefault(); const f = formData(e.target);
      const r = await act(() => api("cash", { method: "POST", body: { Action: "add_manual", Amount: +f.Amount, Ref: f.Ref, Wallet: f.Wallet, Time: f.Time, Notes: f.Notes } }));
      if (r) { closeModal(); toast("رقم المعاملة: " + r.code); cashList(head); } };
  };
}

async function cashPhones(head) {
  const d = await api("cash?devices=1"), pt = d.points || {};
  C(`<div class="dz">${head}
    <div class="dz-card"><div class="dz-card-h"><span><i class="bi bi-phone"></i> الموبايلات المربوطة</span><button class="green" id="cpAdd"><i class="bi bi-plus-lg"></i> ربط موبايل</button></div>
      ${(d.devices || []).length ? `<div class="tablewrap"><table><tr><th>الموبايل</th><th>الحالة</th><th>آخر اتصال</th><th>حالة الأبلكيشن</th><th></th></tr>
        ${d.devices.map(x => { let st = {}; try { st = JSON.parse(x.info || "{}"); } catch (e) { st = { name: x.info }; }
          const ok = k => st[k] === undefined ? "" : st[k] ? `<span class="dz-pill p-green">${k}</span>` : `<span class="dz-pill p-rose">${k} ✗</span>`;
          return `<tr class="${+x.enabled ? "" : "muted"}"><td><b>${esc(x.name)}</b>${st.model ? `<br><small class="muted">${esc(st.model)} · v${esc(st.version || "")}${d.app && st.version && st.version !== d.app ? ` <span class="dz-pill p-amber">فيه تحديث ${esc(d.app)} — افتح الأبلكيشن ودوس «تحديث»</span>` : ""}</small>` : ""}</td>
          <td>${!+x.enabled ? '<span class="dz-pill p-slate">موقوف</span>' : x.pair_code ? `<span class="dz-pill p-amber">مستني الربط</span><br>الكود <b dir="ltr">${esc(x.pair_code)}</b> — الرقم السري <b dir="ltr">${esc(x.pair_pin)}</b>`
            : +x.paired ? (x.idle_min !== null && +x.idle_min <= 30 ? '<span class="dz-pill p-green"><span class="dz-dot"></span> شغال</span>' : '<span class="dz-pill p-rose">ساكت</span>') : '<span class="dz-pill p-slate">مش مربوط</span>'}</td>
          <td dir="ltr">${cashDT(x.last_seen)}${x.idle_min !== null ? `<br><small class="muted">من ${fmt(x.idle_min)} دقيقة</small>` : ""}</td>
          <td>${ok("sms")} ${ok("notif")} ${ok("battery")} ${st.notif && st.listening === false ? '<span class="dz-pill p-rose">الإشعارات مش واصلة — افتح الأبلكيشن</span>' : ""} ${st.queue ? `<span class="dz-pill p-amber">مستني يتبعت: ${esc(st.queue)}</span>` : ""}</td>
          <td class="vpn-act"><button class="b-light" data-rep="${x.id}">كود ربط جديد</button><button class="b-light" data-tog="${x.id}">${+x.enabled ? "إيقاف" : "تشغيل"}</button></td></tr>`; }).join("")}</table></div>`
        : '<div class="empty">لسه مفيش موبايل مربوط — دوس «ربط موبايل».</div>'}</div>
    <div class="dz-card"><div class="dz-card-h"><span><i class="bi bi-speedometer"></i> عداد النقاط (للمقارنة بـ ADV Cash بس — مفيش فلوس)</span></div>
      <p>نقطة لكل رسالة فودافون كاش، ونقطتين لكل تحويل إنستاباي.${pt.from ? ` من <b dir="ltr">${esc(pt.from)}</b>:` : ":"}
        فودافون كاش <b>${fmt(pt.vf || 0)}</b>، إنستاباي <b>${fmt(pt.ip || 0)}</b> = <b>${fmt(pt.used || 0)}</b> نقطة. الباقي من ${fmt(pt.start || 0)}: <b>${fmt(pt.left || 0)}</b></p>
      <form id="cpPts" class="g12"><div class="fg s4"><label>ابدأ العداد من جديد بـ</label><input name="n" type="number" value="5000"></div><div class="fg s2"><label>&nbsp;</label><button class="b-light">ابدأ</button></div></form></div>
    <form class="dz-card" id="cpSet"><div class="dz-card-h"><span><i class="bi bi-funnel"></i> الأبلكيشن يقرا من مين</span><button class="green">حفظ</button></div>
      <div class="g12"><div class="fg s6"><label>أسماء مرسلين الـ SMS (مفصولة بفاصلة)</label><input name="Senders" dir="ltr" value="${esc(d.senders || "")}"></div>
        <div class="fg s6"><label>أبلكيشنات الإشعارات (جزء من اسم الباكدج)</label><input name="Apps" dir="ltr" value="${esc(d.apps || "")}"></div></div>
      <label><input type="checkbox" name="Auto" ${d.auto ? "checked" : ""}> إضافة تلقائية للرصيد لو رقم المحفظة / عنوان إنستاباي متسجل على مشترك واحد (أو اتستخدم له قبل كده)</label>
      <div class="fg"><label>أرقام / عناوين ما تتضافش تلقائي أبدًا (السنترالات — مفصولة بفاصلة)</label><input name="NoAuto" dir="ltr" value="${esc(d.noauto || "")}"></div>
      <p class="muted">التلقائي بيشتغل بس لو الرقم متسجل على مشترك واحد ومحدش اتدفعله منه غيره. الرقم اللي اتستخدم لأكتر من مشترك بيبقى «اقتراح» بس.</p>
      <p class="muted">أي رسالة من المرسلين دول بتتبعت للسيستم. اللي يتفهم بيبقى معاملة، واللي ما يتفهمش بيظهر في «رسايل مش مفهومة» عشان نضيف شكله.</p></form></div>`);
  const post = async (body, msg) => { const r = await act(() => api("cash", { method: "POST", body }), msg); return r; };
  const showCode = r => r && modal(`<h3>ربط الموبايل</h3><p>افتح أبلكيشن <b>StreamCash</b> على الموبايل واكتب:</p>
    <p style="font-size:22px">الكود: <b dir="ltr">${esc(r.code)}</b><br>الرقم السري: <b dir="ltr">${esc(r.pin)}</b></p><p class="muted">الكود شغال 30 دقيقة ومرة واحدة.</p>
    <button class="green" onclick="closeModal();PAGES.cash()">تمام</button>`);
  $("#cpAdd").onclick = async () => { const n = prompt("اسم الموبايل (مثلا: موبايل أحمد)"); if (n) showCode(await post({ Action: "add_device", Name: n })); };
  document.querySelectorAll("[data-rep]").forEach(b => b.onclick = async () => confirm("كود ربط جديد؟ الربط القديم للموبايل ده هيبطل لحد ما تكتب الكود الجديد.") && showCode(await post({ Action: "repair_device", ID: +b.dataset.rep })));
  document.querySelectorAll("[data-tog]").forEach(b => b.onclick = async () => { await post({ Action: "toggle_device", ID: +b.dataset.tog }, "تم"); cashPhones(head); });
  $("#cpPts").onsubmit = async e => { e.preventDefault(); await post({ Action: "points", Amount: +formData(e.target).n }, "العداد بدأ"); cashPhones(head); };
  $("#cpSet").onsubmit = async e => { e.preventDefault(); const f = formData(e.target); await post({ Action: "settings", Senders: f.Senders, Apps: f.Apps, Auto: !!f.Auto, NoAuto: f.NoAuto }, "اتحفظ"); };
}

async function cashUnknown(head) {
  const rows = await api("cash?raw=unknown");
  C(`<div class="dz">${head}<div class="dz-card"><div class="dz-card-h"><span><i class="bi bi-question-circle"></i> رسايل من الأرقام المتابعة ما اتفهمتش (${rows.length})</span>
    <button class="b-light" id="cuRe"><i class="bi bi-arrow-repeat"></i> اقراهم تاني</button></div>
    <p class="muted">لو فيهم تحويل حقيقي: ابعت صورة الرسالة، أضيف شكلها، وبعدين «اقراهم تاني» بيطلّع منهم المعاملات. ولحد كده تقدر تضيفه كـ «معاملة يدوي».</p>
    ${rows.length ? `<div class="tablewrap"><table><tr><th>الوقت</th><th>الموبايل</th><th>من</th><th>الرسالة</th></tr>${rows.map(r => `<tr><td dir="ltr">${cashDT(r.at || r.received)}</td><td>${esc(r.device || "")}</td>
      <td dir="ltr">${esc(r.sender)}</td><td style="white-space:pre-wrap;max-width:520px">${esc(r.body)}</td></tr>`).join("")}</table></div>` : '<div class="empty">كله اتفهم 👍</div>'}</div></div>`);
  $("#cuRe").onclick = async () => { const r = await act(() => api("cash", { method: "POST", body: { Action: "reparse" } })); if (r) { toast(`اتطلع ${r.added} معاملة`); cashUnknown(head); } };
}

// «مقارنة ADV» (InstaPay only): ADV's cash page (exported there, pasted/uploaded here) next to ours — what ADV has that we
// don't (ناقص عندنا) and what we have that ADV doesn't (زيادة عندنا)
async function cashAdv(head) {
  const p = new URLSearchParams(location.hash.split("?")[1] || "");
  const day = n => new Date(Date.now() - n * 864e5 + 2 * 36e5).toISOString().slice(0, 10);
  const from = p.get("from") || day(1), to = p.get("to") || day(0);
  const d = await api(`cash/adv?from=${from}&to=${to}`);
  const m = d.matched || [], miss = d.missing || [], ext = d.extra || [];
  const sum = a => a.reduce((s, r) => s + +r.amount, 0);
  const how = { address: '<span class="dz-pill p-green">بالعنوان</span>', name: '<span class="dz-pill p-sky">بالاسم</span>', guess: '<span class="dz-pill p-amber">بالمبلغ والوقت</span>' };
  const advUsed = r => r.adv_used ? `<span class="dz-pill p-green">مستخدم</span> <small dir="ltr">${esc(r.adv_by || "")}</small>` : '<span class="dz-pill p-slate">غير مستخدم</span>';
  const tbl = (title, ic, cls, rows, cols, empty) => `<div class="dz-card"><div class="dz-card-h"><span class="${cls}"><i class="bi ${ic}"></i> ${title} (${fmt(rows.length)} — ${fmt(sum(rows))} جنيه)</span></div>
    ${rows.length ? `<div class="tablewrap"><table>${cols}</table></div>` : `<div class="empty">${empty}</div>`}</div>`;
  C(`<div class="dz">${head}
    <div class="dz-card"><div class="dz-card-h"><span><i class="bi bi-upload"></i> بيانات ADV</span></div>
      <p class="muted" style="margin:0 0 8px">في ADV: تقارير ← آخر ← معاملات الكاش ← اختار «500 في الصفحة» ← دوس <b>CSV</b> وارفع الملف هنا. أو حدّد الجدول كله وانسخه والصقه في المربع.
        ${d.imported ? `آخر استيراد: <b dir="ltr">${cashDT(d.imported)}</b>.` : "لسه ما اتستوردش حاجة."}</p>
      <div class="g12"><div class="fg s8"><textarea id="caxT" rows="3" placeholder="الصق جدول ADV هنا"></textarea></div>
        <div class="fg s4"><input type="file" id="caxF" accept=".csv,.txt,text/csv"><button class="green" id="caxGo" style="margin-top:6px"><i class="bi bi-arrow-repeat"></i> استيراد وقارن</button></div></div></div>
    <form id="caxR" class="dz-card g12"><div class="fg s4"><label>من يوم</label><input type="date" name="from" value="${from}"></div><div class="fg s4"><label>لحد يوم</label><input type="date" name="to" value="${to}"></div>
      <div class="fg s4"><label>&nbsp;</label><button class="green">قارن</button></div>
      <div class="fg s12"><small class="muted">إنستاباي بس. وقت ADV متأخر ${fmt(d.offset)} دقيقة عن الحقيقي، والأوقات هنا متصلّحة. ${d.cover ? `بيانات ADV المستوردة بتغطي <b dir="ltr">${esc(d.cover)}</b>` : ""}${+d.outside ? ` — و${fmt(d.outside)} معاملة عندنا برا الوقت ده (لسه ما اتستوردتش من ADV) مش محسوبة زيادة.` : ""}</small></div></form>
    <div class="dz-kpis">
      <div class="dz-kpi k-green"><div class="dz-kpi-h"><span>متطابق</span><span class="dz-ico"><i class="bi bi-check2-all"></i></span></div><div class="dz-num">${fmt(m.length)}</div><small>${fmt(sum(m))} جنيه</small></div>
      <div class="dz-kpi k-rose"><div class="dz-kpi-h"><span>ناقص عندنا</span><span class="dz-ico"><i class="bi bi-dash-circle"></i></span></div><div class="dz-num">${fmt(miss.length)}</div><small>${fmt(sum(miss))} جنيه — في ADV ومش عندنا</small></div>
      <div class="dz-kpi k-amber"><div class="dz-kpi-h"><span>زيادة عندنا</span><span class="dz-ico"><i class="bi bi-plus-circle"></i></span></div><div class="dz-num">${fmt(ext.length)}</div><small>${fmt(sum(ext))} جنيه — عندنا ومش في ADV</small></div>
    </div>
    ${tbl("ناقص عندنا", "bi-dash-circle", "", miss, `<tr><th>رقم ADV</th><th>عنوان إنستاباي</th><th>المبلغ</th><th>الوقت</th><th>في ADV</th><th></th></tr>
      ${miss.map(r => `<tr><td dir="ltr">${esc(r.adv_code)}</td><td dir="ltr">${esc(r.wallet)}</td><td><b>${fmt(r.amount)}</b></td><td dir="ltr">${esc(r.at)}</td><td>${advUsed(r)}</td>
        <td>${r.before_app ? '<small class="muted">قبل تشغيل الأبلكيشن</small>' : ""}</td></tr>`).join("")}`, "مفيش — كل تحويل في ADV موجود عندنا 👍")}
    ${tbl("زيادة عندنا", "bi-plus-circle", "", ext, `<tr><th>رقمنا</th><th>الاسم / العنوان</th><th>المبلغ</th><th>الوقت</th><th>الحالة</th></tr>
      ${ext.map(r => `<tr><td dir="ltr"><b>${esc(r.code)}</b></td><td>${esc(r.name || "")} <small dir="ltr" class="muted">${esc(r.wallet || "")}</small></td><td><b>${fmt(r.amount)}</b></td><td dir="ltr">${esc(r.at)}</td><td>${cashStatus(r.status)}</td></tr>`).join("")}`, "مفيش — مفيش حاجة عندنا مش في ADV 👍")}
    ${tbl("متطابق", "bi-check2-all", "", m, `<tr><th>رقم ADV</th><th>رقمنا</th><th>عنوان إنستاباي</th><th>الاسم عندنا</th><th>المبلغ</th><th>الوقت</th><th>اتطابق</th><th>في ADV</th><th>عندنا</th></tr>
      ${m.map(r => `<tr><td dir="ltr">${esc(r.adv_code)}</td><td dir="ltr"><b>${esc(r.code)}</b></td><td dir="ltr">${esc(r.wallet)}</td><td>${esc(r.name || "")}</td><td><b>${fmt(r.amount)}</b></td>
        <td dir="ltr">${esc(r.our_at)}</td><td>${how[r.how] || ""}</td><td>${advUsed(r)}</td><td>${cashStatus(r.status)}</td></tr>`).join("")}`, "لسه مفيش")}
  </div>`);
  $("#caxR").onsubmit = e => { e.preventDefault(); const f = formData(e.target); location.hash = `cash?tab=adv&from=${f.from}&to=${f.to}`; };
  $("#caxGo").onclick = async () => {
    const file = $("#caxF").files[0];
    const text = file ? await file.text() : $("#caxT").value;
    if (!text.trim()) return toast("ارفع ملف CSV من ADV أو الصق الجدول", true);
    const r = await act(() => api("cash/adv", { method: "POST", body: { Text: text } }));
    if (r) { toast(`اتقرا ${fmt(r.read)} معاملة من ADV (${fmt(r.added)} جديدة)`); cashAdv(head); }
  };
}
